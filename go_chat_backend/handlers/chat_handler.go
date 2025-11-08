package handlers

import (
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/services"
)

type ChatHandler struct {
	chatService *services.ChatService
}

func NewChatHandler(chatService *services.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) AskQuestions(c *fiber.Ctx) error {
	docID := c.Params("doc_id")
	var req models.ChatReq
	if err := c.BodyParser(&req); err != nil {
		logging.Logger.Error("fail Parsing Requests", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	ctx := c.Context()
	ans, err := h.chatService.AskQuestion(ctx, docID, req)
	if err != nil {
		logging.Logger.Error("fail AskQuestions", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to ask question"})
	}
	return c.JSON(ans)
}
