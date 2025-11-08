package bootstrap

import (
	"go_chat_backend/config"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/cache"
	"go_chat_backend/platform/database"
	"go_chat_backend/platform/events"
	"go_chat_backend/platform/grpc/clients"
	"go_chat_backend/platform/queue"
	"go_chat_backend/platform/redis"
	"go_chat_backend/platform/storage"
)

type Infrastructure struct {
	DB             *database.DB
	Redis          *redis.Service
	Storage        *storage.Service
	Queue          cache.MessageQueue
	Cache          cache.CacheService
	EventPublisher *events.EventPublisher
	GrpcClients    *clients.GrpcClients
}

func NewInfrastructure(cfg *config.Config) (*Infrastructure, error) {
	infra := &Infrastructure{}

	// database
	db, err := database.InitPostgres(cfg)
	if err != nil {
		return nil, err
	}
	infra.DB = db
	if err := infra.DB.AutoMigrate(); err != nil {
		return nil, err
	}
	// redis services
	redisService, err := redis.InitRedis(cfg)
	if err != nil {
		logging.Logger.Error("fail Initializing Redis", err)
		return nil, err
	}
	infra.Redis = redisService

	// storage services
	storageService, err := storage.InitStorageService(cfg)
	if err != nil {
		logging.Logger.Error("fail Initializing Bucket", err)
		return nil, err
	}
	infra.Storage = storageService

	// message queue
	queueService := queue.NewMessageService(redisService)
	infra.Queue = queueService

	// cache
	l1CacheService := cache.InitL1Cache()
	cacheService := cache.NewCacheService(l1CacheService, redisService)
	infra.Cache = cacheService

	// event publisher
	eventPublisher := events.NewEventPublisher(redisService.Rdb)
	infra.EventPublisher = eventPublisher

	grpcClient := clients.NewGrpcClients(cfg)
	infra.GrpcClients = grpcClient

	return infra, nil
}

func (infra *Infrastructure) Shutdown() error {
	if err := infra.DB.Close(); err != nil {
		logging.Logger.Error("fail closing database", err)
		return err
	}
	if err := infra.Redis.Rdb.Close(); err != nil {
		logging.Logger.Error("fail closing redis", err)
		return err
	}
	if err := infra.GrpcClients.Close(); err != nil {
		logging.Logger.Error("fail closing grpc", err)
		return err
	}
	return nil
}
