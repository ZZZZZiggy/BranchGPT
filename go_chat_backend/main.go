package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go_chat_backend/config"
	"go_chat_backend/handlers"
	"go_chat_backend/middleware"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/routes"
	"go_chat_backend/services"
	"os"
)

func main() {
	logging.Init()
	// env
	_ = godotenv.Load()
	cfg := config.LoadConfig()
	app := fiber.New()
	// middleware
	app.Use(middleware.Logger())
	app.Use(middleware.CORS())

	// initiate bucket
	StorageService, err := services.InitStorageService(cfg)
	if err != nil {
		logging.Logger.Error("fail Initializing Bucket", err)
	}
	// initiate postgres sql db
	if err = services.InitPostgres(); err != nil {
		logging.Logger.Error("fail Initializing Postgres", err)
	}
	// initiate redis
	redisService, err := services.InitRedis()
	if err != nil {
		logging.Logger.Error("fail Initializing Redis", err)
	}
	l1CacheService := services.InitL1Cache()
	CacheService := services.NewCacheService(l1CacheService, redisService)

	chatHandler := handlers.NewChatHandler(CacheService)
	docHandler := handlers.NewDocHandler(StorageService)
	routes.RegisterHealthRoutes(app)
	routes.RegisterDocumentRoutes(app)
	routes.RegisterUserRoutes(app, chatHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	logging.Logger.Info("Server running on http://localhost:" + port)
	logging.Logger.Error("Server terminated", app.Listen(":"+port))
	os.Exit(1)
}
