package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"go_chat_backend/handlers"
)

func SetupWebSocketRoutes(app *fiber.App, wsHandler *handlers.WSHandler) {
	ws := app.Group("/ws")

	// WebSocket route
	ws.Use("/document/:doc_id", wsHandler.WebSocketUpgrade)
	ws.Get("/document/:doc_id", websocket.New(wsHandler.HandleDocumentEvents))
}
