package services

import "time"

// CacheStore saves L2 cache function
type CacheStore interface {
	GetCache(key string) (interface{}, bool)
	SetCache(key string, value interface{}, expiration time.Duration) error
	DelCache(key string) error
}

type MessageQueue interface {
	PushToQueue(queueName string, value interface{}) error
	PopFromQueue(queueName string) (interface{}, error)
}
