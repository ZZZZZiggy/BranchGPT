package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"go_chat_backend/config"
	"go_chat_backend/pkg/logging"
	"time"

	"github.com/redis/go-redis/v9"
)

type Service struct {
	Rdb *redis.Client
	Ctx context.Context
}

func InitRedis(cfg *config.Config) (*Service, error) {
	redisUrl := cfg.RedisURL
	if redisUrl == "" {
		return nil, fmt.Errorf("empty redis url")
	}
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse Redis URL: %w", err)
	}
	rdb := redis.NewClient(opt)

	testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rdb.Ping(testCtx).Err(); err != nil {
		return nil, fmt.Errorf("could not connect to Redis: %w", err)
	}
	fmt.Println("Connected to Redis")
	return &Service{
		Rdb: rdb,
		Ctx: context.Background(),
	}, nil
}
func (s *Service) SetCache(key string, value interface{}, expiration time.Duration) error {
	prefixedKey := "cache:" + key

	// 序列化 value 为 JSON
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}

	return s.Rdb.Set(s.Ctx, prefixedKey, jsonData, expiration).Err()
}

func (s *Service) GetCache(key string) (interface{}, bool) {
	prefixedKey := "cache:" + key
	val, err := s.Rdb.Get(s.Ctx, prefixedKey).Result()
	if err != nil {
		return nil, false
	}
	// 返回 JSON 字符串，由上层决定如何反序列化
	return val, true
}
func (s *Service) DelCache(key string) error {
	prefixedKey := "cache:" + key
	return s.Rdb.Del(s.Ctx, prefixedKey).Err()
}
func (s *Service) PushToQueue(queueName string, value interface{}) error {
	prefixedQueueName := "queue:" + queueName
	jsonValue, err := json.Marshal(value)
	if err != nil {
		logging.Logger.Error("fail PushToQueue", err)
		return err
	}
	return s.Rdb.LPush(s.Ctx, prefixedQueueName, string(jsonValue)).Err()
}
func (s *Service) PopFromQueue(queueName string) (interface{}, error) {
	prefixedQueueName := "queue:" + queueName
	return s.Rdb.RPop(s.Ctx, prefixedQueueName).Result()
}
