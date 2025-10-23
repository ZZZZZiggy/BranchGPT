package routes

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/handlers"
)

func RegisterUserRoutes(app *fiber.App, chatHandler *handlers.ChatHandler) {
	chats := app.Group("api/chat")
	chats.Post("/:doc_id/questions", chatHandler.AskQuestions)
	chats.Get("/:doc_id/tree", handlers.GetTree)
}
