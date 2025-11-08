package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go_chat_backend/platform/cache"
)

// LLMConfig 用户的 LLM 配置
type LLMConfig struct {
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	UserID   string `json:"user_id"`
}

// MarshalBinary 实现 encoding.BinaryMarshaler 接口
func (c *LLMConfig) MarshalBinary() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalBinary 实现 encoding.BinaryUnmarshaler 接口
func (c *LLMConfig) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, c)
}

// LLMConfigService 管理用户的 LLM 配置
type LLMConfigService struct {
	typedCache *cache.TypedCache[LLMConfig]
	cacheTTL   time.Duration // 缓存过期时间，默认 30 分钟
}

func NewLLMConfigService(cacheService cache.CacheService) *LLMConfigService {
	return &LLMConfigService{
		typedCache: cache.NewTypedCache[LLMConfig](cacheService),
		cacheTTL:   30 * time.Minute,
	}
}

// SetUserLLMConfig 设置用户的 LLM 配置（带缓存）
func (s *LLMConfigService) SetUserLLMConfig(ctx context.Context, userID string, config *LLMConfig) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	config.UserID = userID
	cacheKey := s.getCacheKey(userID)

	return s.typedCache.Set(cacheKey, *config, s.cacheTTL)
}

// GetUserLLMConfig 获取用户的 LLM 配置
func (s *LLMConfigService) GetUserLLMConfig(ctx context.Context, userID string) (*LLMConfig, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	cacheKey := s.getCacheKey(userID)

	config, exists, err := s.typedCache.Get(cacheKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM config for user %s: %w", userID, err)
	}
	if !exists {
		return nil, fmt.Errorf("LLM config not found for user %s", userID)
	}

	return &config, nil
}

// GetOrUseDefault 获取用户配置，如果请求提供了配置则优先使用请求的配置并更新缓存
func (s *LLMConfigService) GetOrUseDefault(ctx context.Context, userID, apiKey, model, provider string) (*LLMConfig, error) {
	// 如果请求中提供了完整的 LLM 配置，优先使用并更新缓存
	if apiKey != "" && model != "" && provider != "" {
		config := &LLMConfig{
			APIKey:   apiKey,
			Model:    model,
			Provider: provider,
			UserID:   userID,
		}
		// 异步更新缓存，不阻塞请求
		go func() {
			_ = s.SetUserLLMConfig(context.Background(), userID, config)
		}()
		return config, nil
	}

	// 否则从缓存获取
	cachedConfig, err := s.GetUserLLMConfig(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("no LLM config provided and no cached config found for user %s", userID)
	}

	// 如果请求中提供了部分配置，用请求的覆盖缓存的
	if apiKey != "" {
		cachedConfig.APIKey = apiKey
	}
	if model != "" {
		cachedConfig.Model = model
	}
	if provider != "" {
		cachedConfig.Provider = provider
	}

	return cachedConfig, nil
}

// DeleteUserLLMConfig 删除用户的 LLM 配置（清除缓存）
func (s *LLMConfigService) DeleteUserLLMConfig(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	cacheKey := s.getCacheKey(userID)
	return s.typedCache.Delete(cacheKey)
}

// RefreshTTL 刷新缓存过期时间
func (s *LLMConfigService) RefreshTTL(ctx context.Context, userID string) error {
	config, err := s.GetUserLLMConfig(ctx, userID)
	if err != nil {
		return err
	}

	return s.SetUserLLMConfig(ctx, userID, config)
}

// getCacheKey 生成缓存键
func (s *LLMConfigService) getCacheKey(userID string) string {
	return fmt.Sprintf("llm_config:user:%s", userID)
}

// MaskAPIKey 脱敏 API Key（用于日志）
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
}
