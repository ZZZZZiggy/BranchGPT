package main

import (
	"go_chat_backend/bootstrap"
	"go_chat_backend/config"
	"go_chat_backend/middleware"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/routes"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	logging.Init()
	// env
	_ = godotenv.Load()
	cfg := config.LoadConfig()

	// initiate bucket
	app, err := bootstrap.NewApp(cfg)
	if err != nil {
		logging.Logger.Error("fail to create app", "error", err)
		return
	}
	defer func(app *bootstrap.App) {
		err := app.Shutdown()
		if err != nil {
			logging.Logger.Error("fail to shutdown app", "error", err)
		}
	}(app)

	// https server
	httpServer := fiber.New(fiber.Config{
		AppName: "CogniCore API",
	})

	// middleware
	httpServer.Use(middleware.Logger())
	httpServer.Use(middleware.CORS())
	routes.RegisterHealthRoutes(httpServer)
	routes.RegisterDocumentRoutes(httpServer, app.Handlers.DocHandler)
	routes.SetupWebSocketRoutes(httpServer, app.Handlers.WSHandler)
	routes.RegisterChatRoutes(httpServer, app.Handlers.ChatHandler)

	go func() {
		if err := httpServer.Listen(":" + cfg.HttpPort); err != nil {
			logging.Logger.Error("fail to listen", "error", err)
		}
	}()
	logging.Logger.Info("Application started",
		"http_port", cfg.HttpPort,
		"grpc_port", cfg.GoGrpcIngestPort,
	)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Logger.Info("Shutting down...")
	err = httpServer.Shutdown()
	if err != nil {
		logging.Logger.Error("fail to shutdown http server", "error", err)
		return
	}
	err = app.Shutdown()
	if err != nil {
		logging.Logger.Error("fail to shutdown app", "error", err)
		return
	}
	logging.Logger.Info("✓ Shutdown complete")
}
