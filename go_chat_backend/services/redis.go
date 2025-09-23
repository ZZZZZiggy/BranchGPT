package services

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

var Rdb *redis.Client
var Ctx = context.Background()

func InitRedis() error {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	_, err := Rdb.Ping(Ctx).Result()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	fmt.Println("Connected to Redis")
	return nil
}
func SaveSection(docID string, sectionID string, content string) error {
	key := fmt.Sprintf("docID:%s, sectionID:%s", docID, sectionID)
	return Rdb.Set(Ctx, key, content, 24*time.Hour).Err()
}
func GetSection(docID string, sectionID string) (string, error) {
	key := fmt.Sprintf("docID:%s, sectionID:%s", docID, sectionID)
	return Rdb.Get(Ctx, key).Result()
}

// SaveToRedis processedData type: "paragraphs": {"chapter", "content", "page"}
func SaveToRedis(processedData map[string]interface{}, docID string) error {
	paragraphs, ok := processedData["paragraphs"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid data structure: paragraphs not found or wrong type")
	}
	for i, paragraph := range paragraphs {
		paragraph, ok := paragraph.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid data structure: paragraph not found or wrong type")
		}
		chapter, ok := paragraph["chapter"].(string)
		if !ok {
			return fmt.Errorf("invalid data structure: chapter not found or wrong type")
		}
		content, ok := paragraph["content"].(string)
		if !ok {
			return fmt.Errorf("invalid data structure: content not found or wrong type")
		}
		err := SaveSection(docID, chapter, content)
		if err != nil {
			return fmt.Errorf("failed to save paragraph %d: %w", i, err)
		}
	}
	return nil
}
