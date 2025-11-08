package cache

import "time"

// CacheService Services saves L2 cache function
type CacheService interface {
	GetCache(key string) (interface{}, bool)
	SetCache(key string, value interface{}, expiration time.Duration) error
	DelCache(key string) error
}

// MessageQueue is for redis message queue
type MessageQueue interface {
	PushToQueue(queueName string, value interface{}) error
	PopFromQueue(queueName string) (interface{}, error)
}
