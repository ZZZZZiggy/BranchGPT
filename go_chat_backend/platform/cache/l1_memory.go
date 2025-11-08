package cache

import (
	"github.com/patrickmn/go-cache"
	"time"
)

type L1CacheService struct {
	client *cache.Cache
}

func InitL1Cache() *L1CacheService {
	return &L1CacheService{
		client: cache.New(5*time.Minute, 10*time.Minute),
	}
}

func (s *L1CacheService) Get(key string) (interface{}, bool) {
	return s.client.Get(key)
}

func (s *L1CacheService) Set(key string, value interface{}, expiration time.Duration) {
	s.client.Set(key, value, expiration)
}
func (s *L1CacheService) Del(key string) {
	s.client.Delete(key)
}
