package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type URLShortener struct {
	DB *sql.DB
}

type URL struct {
	ID          int
	OriginalURL string
	ShortCode   string
	CreatedAt   time.Time
	Visits      int
}

func main() {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "urlshortener")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping the database: %v", err)
	}

	err = initializeDB(db)
	if err != nil {
		log.Fatalf("Failed to initialize the DB: %v", err)
	}

	shortener := &URLShortener{DB: db}

	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	router.HandleFunc("/", shortener.homeHandler).Methods("GET")
	router.HandleFunc("/shorten", shortener.shortenHandler).Methods("POST")
	router.HandleFunc("/stats", shortener.statsHandler).Methods("GET")
	router.HandleFunc("/{shortCode}", shortener.redirectHandler).Methods("GET")

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on port http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func initializeDB(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS urls(
    id SERIAL PRIMARY KEY,
    original_url TEXT NOT NULL,
    short_code VARCHAR(10) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    visits INTEGER DEFAULT 0
    )
`)
	return err
}

func (s *URLShortener) homeHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./templates/index.html")
}

func (s *URLShortener) shortenHandler(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	originalURL := r.FormValue("url")
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	shortcode := generateShortCode(6)

	_, err = s.DB.Exec(
		"INSERT INTO urls (original_url, short_code) VALUES ($1, $2)",
		originalURL, shortcode,
	)

	if err != nil {
		log.Printf("Error inserting URL: %v", err)
		http.Error(w, "Error creating short url", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		baseURL := fmt.Sprintf("%s://%s", getScheme(r), r.Host)
		shortURL := fmt.Sprintf("%s/%s", baseURL, shortcode)

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`
		<div id="result" class="result-container">
		    <p>Your shortened URL:</p>
			<a href="%s" target="_blank">%s</a>
			<button class="copy-btn" onclick="copyToClipBoard('%s')">Copy</button>
		</div>
		`, shortURL, shortURL, shortURL)))
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *URLShortener) redirectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortCode := vars["shortCode"]

	var url URL
	err := s.DB.QueryRow(
		"SELECT id, original_url FROM urls WHERE short_code =  $1",
		shortCode,
	).Scan(&url.ID, &url.OriginalURL)

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("Database error: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	_, err = s.DB.Exec("UPDATE urls SET visits = visits +1 WHERE id = $1", url.ID)
	if err != nil {
		log.Printf("Error updating visit count: %v", err)
	}

	http.Redirect(w, r, url.OriginalURL, http.StatusMovedPermanently)
}

func (s *URLShortener) statsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`
	SELECT original_url, short_code, created_at, visits 
	FROM urls
	ORDER BY visits DESC
	LIMIT 10
	`)

	if err != nil {
		log.Printf("Error querying stats: %v", err)
		http.Error(w, "Error fetching stats", http.StatusInternalServerError)
	}

	defer rows.Close()

	var urls []URL
	baseURL := fmt.Sprintf("%s://%s", getScheme(r), r.Host)

	for rows.Next() {
		var url URL
		var createdAt time.Time
		err := rows.Scan(&url.OriginalURL, &url.ShortCode, &createdAt, &url.Visits)

		if err != nil {
			log.Printf("Errow scanning row: %v", err)
			continue
		}

		url.CreatedAt = createdAt
		urls = append(urls, url)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		http.Error(w, "Error processing stats", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		html := `<table>
		           <thead>
				       <tr>
					   <th> Original URL </th>
					   <th> Short URL </th>
					   <th> Created </th>
					   <th> Visists </th>
					   </tr>
				   </thead>
		        </tbody>`

		for _, url := range urls {
			shortURL := fmt.Sprintf("%s/%s", baseURL, url.ShortCode)
			html += fmt.Sprintf(`
			<tr>
			    <td><a href="%s" target="_blank">%s</a></td>
				<td><a href="%s" target="_blank">%s</a></td>
				<td>%s</td>
				<td>%d</td>
			</tr>
			`, url.OriginalURL, truncateString(url.OriginalURL, 30), shortURL, shortURL, url.CreatedAt.Format("2006-01-02"), url.Visits)
		}

		html += `</tbody></table>`
		w.Write([]byte(html))
		return
	}

	http.ServeFile(w, r, "./templates/stats.html")
}

func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}

func getScheme(r *http.Request) string {
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	return s[:maxLength-3] + "..."
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
