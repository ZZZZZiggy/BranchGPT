package routes

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/handlers"
)

func RegisterDocumentRoutes(app *fiber.App) {
	document := app.Group("api/pdf")
	document.Post("/process", handlers.ProcessPDF)
}
