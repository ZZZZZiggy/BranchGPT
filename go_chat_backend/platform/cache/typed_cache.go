package cache

import (
	"encoding/json"
	"fmt"
	"time"
)

// TypedCache 提供类型安全的缓存操作
type TypedCache[T any] struct {
	cache CacheService
}

// NewTypedCache 创建类型化的缓存包装器
func NewTypedCache[T any](cache CacheService) *TypedCache[T] {
	return &TypedCache[T]{cache: cache}
}

// Set 设置缓存，自动序列化
func (tc *TypedCache[T]) Set(key string, value T, expiration time.Duration) error {
	return tc.cache.SetCache(key, value, expiration)
}

// Get 获取缓存，自动反序列化
func (tc *TypedCache[T]) Get(key string) (T, bool, error) {
	var zero T

	rawValue, exists := tc.cache.GetCache(key)
	if !exists {
		return zero, false, nil
	}

	// 尝试直接类型断言
	if typedValue, ok := rawValue.(T); ok {
		return typedValue, true, nil
	}

	// 如果是字符串或字节数组，尝试 JSON 反序列化
	var result T
	switch v := rawValue.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return zero, true, fmt.Errorf("failed to unmarshal cache value: %w", err)
		}
		return result, true, nil
	case []byte:
		if err := json.Unmarshal(v, &result); err != nil {
			return zero, true, fmt.Errorf("failed to unmarshal cache value: %w", err)
		}
		return result, true, nil
	default:
		// 尝试通过 JSON 中转
		jsonData, err := json.Marshal(rawValue)
		if err != nil {
			return zero, true, fmt.Errorf("failed to marshal intermediate value: %w", err)
		}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			return zero, true, fmt.Errorf("failed to unmarshal cache value: %w", err)
		}
		return result, true, nil
	}
}

// Delete 删除缓存
func (tc *TypedCache[T]) Delete(key string) error {
	return tc.cache.DelCache(key)
}
