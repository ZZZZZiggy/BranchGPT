package handlers

import (
	"context"
	"encoding/json"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/events"
)

type WSHandler struct {
	eventPublisher *events.EventPublisher
}

func NewWSHandler(eventPublisher *events.EventPublisher) *WSHandler {
	return &WSHandler{eventPublisher: eventPublisher}
}

func (h *WSHandler) WebSocketUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return c.Status(400).JSON(fiber.Map{"error": "Not a websocket request"})
}

func (h *WSHandler) HandleDocumentEvents(c *websocket.Conn) {
	docID := c.Params("doc_id")
	userID := c.Query("user_id")

	logging.Logger.Info("WebSocket connected",
		"docID", docID,
		"userID", userID,
	)

	// cancelable contex, cancels when the function ends
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventChan, err := h.eventPublisher.SubscribeDocumentEvents(ctx)
	if err != nil {
		logging.Logger.Error("Failed to subscribe to events", "error", err)
		err := c.WriteMessage(websocket.TextMessage, []byte(`{"error":"Failed to subscribe"}`))
		if err != nil {
			return
		}
		return
	}
	// send back to frontend
	err = c.WriteJSON(fiber.Map{
		"type":    "connected",
		"message": "WebSocket connected successfully",
		"doc_id":  docID,
	})
	if err != nil {
		return
	}

	for {
		select {
		case event := <-eventChan:
			if event == nil {
				return
			}
			if event.DocID != docID {
				continue
			}
			if userID != "" && event.UserID != userID {
				continue
			}
			data, _ := json.Marshal(event)
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				logging.Logger.Error("Failed to send WebSocket message", "error", err)
				return
			}

			logging.Logger.Info("Event sent to client",
				"type", event.Type,
				"docID", event.DocID,
			)

		case <-ctx.Done():
			return
		}
	}
}
