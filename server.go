package main

import "github.com/gofiber/fiber/v2"


func main(){
	app := fiber.New()

	app.Get("/", hello)

	app.Listen(":3000")
}

func hello(c *fiber.Ctx) error {
	c.SendString("Hello, World!")	
	return nil
}