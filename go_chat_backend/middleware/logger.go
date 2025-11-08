package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"os"
	"time"
)

func Logger() fiber.Handler {
	env := os.Getenv("APP_ENV")
	if env == "prod" {
		return logger.New(logger.Config{
			Format:     `{"time":"${time}","ip":"${ip}","method":"${method}","path":"${path}","status":${status},"latency":"${latency}"}\n`,
			TimeFormat: time.RFC3339,
			TimeZone:   "Local",
			Output:     os.Stdout,
		})
	}
	// dev mode
	return logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "15:04:05",
		TimeZone:   "Local",
		Output:     os.Stdout,
	})
}
