package main

import (
	"go_chat_backend/routes"
	"go_chat_backend/services"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	// 环境变量
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables from container")
	}


	app := fiber.New()

	if err := services.InitRedis(); err != nil {
		log.Fatal(err)
	}
	if err = services.InitPostgres(); err != nil {
		log.Fatal(err)
	}
	routes.RegisterDocumentRoutes(app)
	routes.RegisterUserRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Println("Server running on http://localhost:" + port)
	log.Fatal(app.Listen(":" + port))
}
