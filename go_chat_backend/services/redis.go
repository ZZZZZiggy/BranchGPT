package services

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

type RedisService struct {
	Rdb *redis.Client
	Ctx context.Context
}

func InitRedis() (*RedisService, error) {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		return nil, fmt.Errorf("empty redis url")
	}
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse Redis URL: %w", err)
	}
	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("could not connect to Redis: %w", err)
	}
	fmt.Println("Connected to Redis")
	return &RedisService{
		Rdb: rdb,
		Ctx: ctx,
	}, nil
}

func (s *RedisService) SetCache(key string, value interface{}, expiration time.Duration) error {
	prefixedKey := "cache:" + key
	return s.Rdb.Set(s.Ctx, prefixedKey, value, expiration).Err()
}
func (s *RedisService) GetCache(key string) (interface{}, bool) {
	prefixedKey := "cache:" + key
	val, err := s.Rdb.Get(s.Ctx, prefixedKey).Result()
	if err != nil {
		return nil, false
	}
	return val, true
}
func (s *RedisService) DelCache(key string) error {
	prefixedKey := "cache:" + key
	return s.Rdb.Del(s.Ctx, prefixedKey).Err()
}
func (s *RedisService) PushToQueue(queueName string, value interface{}) error {
	prefixedQueueName := "queue:" + queueName
	return s.Rdb.LPush(s.Ctx, prefixedQueueName, value).Err()
}
func (s *RedisService) PopFromQueue(queueName string) (interface{}, error) {
	prefixedQueueName := "queue:" + queueName
	return s.Rdb.RPop(s.Ctx, prefixedQueueName).Result()
}

// SaveSection expired
//func (s *RedisService) SaveSection(docID string, sectionID string, content string) error {
//	key := fmt.Sprintf("docID:%s, sectionID:%s", docID, sectionID)
//	return s.Rdb.Set(s.Ctx, key, content, 24*time.Hour).Err()
//}
//func (s *RedisService) GetSection(docID string, sectionID string) (string, error) {
//	key := fmt.Sprintf("docID:%s, sectionID:%s", docID, sectionID)
//	return s.Rdb.Get(s.Ctx, key).Result()
//}

// SaveToRedis processedData type: "paragraphs": {"chapter", "content", "page"}
//func SaveToRedis(processedData map[string]interface{}, docID string) ([]string, error) {
//	var result []string
//	paragraphs, ok := processedData["paragraphs"].([]interface{})
//	if !ok {
//		return result, fmt.Errorf("invalid data structure: paragraphs not found or wrong type")
//	}
//	for i, paragraph := range paragraphs {
//		paragraph, ok := paragraph.(map[string]interface{})
//		if !ok {
//			return result, fmt.Errorf("invalid data structure: paragraph not found or wrong type")
//		}
//		chapter, ok := paragraph["chapter"].(string)
//		result = append(result, chapter)
//		if !ok {
//			return result, fmt.Errorf("invalid data structure: chapter not found or wrong type")
//		}
//		content, ok := paragraph["content"].(string)
//		if !ok {
//			return result, fmt.Errorf("invalid data structure: content not found or wrong type")
//		}
//		err := SaveSection(docID, chapter, content)
//		if err != nil {
//			return result, fmt.Errorf("failed to save paragraph %d: %w", i, err)
//		}
//	}
//	return result, nil
//}
