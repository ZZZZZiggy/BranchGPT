package routes

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/handlers"
)

func RegisterDocumentRoutes(app *fiber.App, handler *handlers.DocHandler) {
	document := app.Group("api/pdf")
	document.Post("/upload", handler.RequestUpload)
	document.Post("/:doc_id/confirm", handler.ConfirmUpload)
	document.Get("/:doc_id/toc", handler.GetToc)
}
