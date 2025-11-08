package cache

import (
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/redis"
	"time"

	"golang.org/x/sync/singleflight"
)

type Config struct {
	L1MaxSize int
	L2MaxSize int
}

type Service struct {
	l1     *L1CacheService
	l2     *redis.Service
	config *Config
	sf     *singleflight.Group
}

func NewCacheService(l1 *L1CacheService, l2 *redis.Service) CacheService {
	return &Service{l1: l1, l2: l2}
}

func (cs *Service) GetCache(key string) (interface{}, bool) {
	if data, ok := cs.l1.Get(key); ok {
		return data, ok
	}
	if data, ok := cs.l2.GetCache(key); ok {
		return data, ok
	}
	return nil, false
}
func (cs *Service) SetCache(key string, value interface{}, expiration time.Duration) error {

	err := cs.l2.SetCache(key, value, expiration)
	if err != nil {
		logging.Logger.Error("l2 fail SetCacheHistory", err)
		return err
	}
	cs.l1.Set(key, value, time.Duration(float64(expiration)*0.3))
	return nil
}
func (cs *Service) DelCache(key string) error {
	cs.l1.Del(key)
	if err := cs.l2.DelCache(key); err != nil {
		logging.Logger.Error("l2 fail DelCacheHistory", err)
		return err
	}
	return nil
}
