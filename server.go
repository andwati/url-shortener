package main

import "github.com/gofiber/fiber/v2"

func main() {
	app := fiber.New()

	app.Get("/", hello)

	err := app.Listen(":3000")
	if err != nil {
		return
	}
}

func hello(c *fiber.Ctx) error {
	err := c.SendString("Hello, World!")
	if err != nil {
		return err
	}
	return nil
}
