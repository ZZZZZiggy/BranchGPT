package services

import (
	"context"
	"fmt"
	"go_chat_backend/platform/cache"
	"go_chat_backend/repository"
	"time"
)

const (
	ragModeCachePrefix = "rag_mode:"
	ragModeCacheTTL    = 24 * time.Hour // 缓存 24 小时
)

// RagModeService 管理 RAG 模式的服务
// 使用两层缓存策略：L1 内存缓存 + 数据库持久化
type RagModeService struct {
	cache   cache.CacheService
	docRepo repository.DocumentRepository
	l1Cache *cache.TypedCache[bool] // L1 内存缓存，快速访问
}

func NewRagModeService(
	cacheService cache.CacheService,
	docRepo repository.DocumentRepository,
) *RagModeService {
	return &RagModeService{
		cache:   cacheService,
		docRepo: docRepo,
		l1Cache: cache.NewTypedCache[bool](cacheService),
	}
}

// GetRagMode 获取文档的 RAG 模式
// 优先级：L1 内存缓存 > 数据库
func (s *RagModeService) GetRagMode(ctx context.Context, fileID string) (bool, error) {
	cacheKey := s.getCacheKey(fileID)

	// 1. 尝试从 L1 内存缓存获取（最快）
	if ragMode, found, err := s.l1Cache.Get(cacheKey); err == nil && found {
		return ragMode, nil
	}

	// 2. 从数据库查询
	doc, err := s.docRepo.GetByID(ctx, fileID)
	if err != nil {
		return false, fmt.Errorf("failed to get document: %w", err)
	}

	// 3. 更新 L1 缓存
	_ = s.l1Cache.Set(cacheKey, doc.RagMode, ragModeCacheTTL)

	return doc.RagMode, nil
}

// SetRagMode 设置文档的 RAG 模式
// 同时更新数据库和缓存
func (s *RagModeService) SetRagMode(ctx context.Context, fileID string, ragMode bool) error {
	cacheKey := s.getCacheKey(fileID)

	// 1. 先获取文档
	doc, err := s.docRepo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// 2. 更新 RagMode 字段
	doc.RagMode = ragMode

	// 3. 更新数据库（使用 UpdateMetadata）
	if err := s.docRepo.UpdateMetadata(ctx, fileID, doc); err != nil {
		return fmt.Errorf("failed to update rag mode in database: %w", err)
	}

	// 4. 更新 L1 缓存
	if err := s.l1Cache.Set(cacheKey, ragMode, ragModeCacheTTL); err != nil {
		// 缓存失败不影响主流程，只记录日志
		// 下次查询时会从数据库重新加载
		return nil
	}

	return nil
}

// InvalidateCache 清除指定文档的缓存
// 用于需要强制刷新的场景
func (s *RagModeService) InvalidateCache(fileID string) error {
	cacheKey := s.getCacheKey(fileID)
	return s.l1Cache.Delete(cacheKey)
}

// BatchGetRagMode 批量获取多个文档的 RAG 模式
// 适用于需要同时查询多个文档的场景
func (s *RagModeService) BatchGetRagMode(ctx context.Context, fileIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)

	// 逐个查询（简化版本，后续可以优化为批量查询）
	for _, fileID := range fileIDs {
		ragMode, err := s.GetRagMode(ctx, fileID)
		if err != nil {
			// 跳过错误的文档，继续处理其他文档
			continue
		}
		result[fileID] = ragMode
	}

	return result, nil
}

// getCacheKey 生成缓存 key
func (s *RagModeService) getCacheKey(fileID string) string {
	return ragModeCachePrefix + fileID
}
