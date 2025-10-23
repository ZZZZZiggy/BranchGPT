package middleware

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"os"
)

func CORS() fiber.Handler {
	allowOrigins := os.Getenv("ALLOWORIGINS")
	fmt.Println("CORS AllowOrigins:", allowOrigins)
	return cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowHeaders: "Origin, Content-Type, Accept",
	})
}
