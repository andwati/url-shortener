CREATE DATABASE urlshortener;

\c urlshortener

CREATE TABLE IF NOT EXISTS urls(
    id SERIAL PRIMARY KEY,
    original_url TEXT NOT NULL,
    short_code VARCHAR(10) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    visits INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_short_code ON urls(short_code)