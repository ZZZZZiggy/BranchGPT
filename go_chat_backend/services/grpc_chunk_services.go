package services

import (
	"context"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/database"
	"go_chat_backend/platform/proto/cognicore"
	"go_chat_backend/repository"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

type ChunkService struct {
	chunkRepo    repository.ChunkRepository
	metadataRepo repository.DocumentRepository

	// 为每个文档维护独立的处理上下文
	docContexts map[string]*DocumentProcessContext
	mu          sync.RWMutex
}

// DocumentProcessContext 每个文档的处理上下文
type DocumentProcessContext struct {
	FileID   string
	FullText *strings.Builder
	Sections []string
	mu       sync.Mutex
}

func NewChunkService(db *database.DB) *ChunkService {
	return &ChunkService{
		chunkRepo:    repository.NewChunkRepository(db.GetDatabase()),
		metadataRepo: repository.NewDocumentRepository(db.GetDatabase()),
		docContexts:  make(map[string]*DocumentProcessContext),
	}
}

// getOrCreateContext 获取或创建文档处理上下文
func (cs *ChunkService) getOrCreateContext(fileID string) *DocumentProcessContext {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if ctx, exists := cs.docContexts[fileID]; exists {
		return ctx
	}

	ctx := &DocumentProcessContext{
		FileID:   fileID,
		FullText: &strings.Builder{},
		Sections: []string{},
	}
	cs.docContexts[fileID] = ctx
	return ctx
}

// CleanupContext 清理文档处理上下文
func (cs *ChunkService) CleanupContext(fileID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.docContexts, fileID)
}

// GetSections 获取文档的 sections
func (cs *ChunkService) GetSections(fileID string) []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if ctx, exists := cs.docContexts[fileID]; exists {
		ctx.mu.Lock()
		defer ctx.mu.Unlock()
		sections := make([]string, len(ctx.Sections))
		copy(sections, ctx.Sections)
		return sections
	}
	return []string{}
}

func (cs *ChunkService) ProcessDocumentMetadata(metadata *cognicore.DocumentMetadata) error {
	ctx := context.Background()
	now := time.Now()

	// 为这个文档创建处理上下文
	docCtx := cs.getOrCreateContext(metadata.FileId)
	docCtx.mu.Lock()
	docCtx.FullText.WriteString(metadata.Filename)
	docCtx.FullText.WriteString("\n")
	docCtx.mu.Unlock()

	doc := &models.DocumentMeta{
		FileID:          metadata.FileId,
		UserID:          metadata.UserId,
		Filename:        metadata.Filename,
		FileHash:        metadata.FileHash,
		FileSize:        metadata.FileSize,
		TotalPages:      metadata.TotalPages,
		EstimatedChunks: metadata.EstimatedChunks,
		Status:          models.StatusProcessing,
		StartedAt:       &now,
		Sections:        pq.StringArray{}, // 初始化为空数组
	}

	return cs.metadataRepo.UpdateMetadata(ctx, metadata.FileId, doc)
}
func (cs *ChunkService) ProcessChunk(chunk *cognicore.TextChunk) error {
	ctx := context.Background()

	//  DEBUG: Check embedding vector length
	if len(chunk.EmbeddingVector) == 0 {
		logging.Logger.Error("chunk %d: embedding vector is EMPTY (length=0)", chunk.ChunkIndex)
	}

	// Clean text to remove NULL bytes (PostgreSQL doesn't allow \x00 in UTF-8)
	cleanedChapter := strings.ReplaceAll(chunk.Chapter, "\x00", "")
	cleanedText := strings.ReplaceAll(chunk.ChunkText, "\x00", "")

	// 获取文档上下文并更新
	docCtx := cs.getOrCreateContext(chunk.FileId)
	docCtx.mu.Lock()
	docCtx.FullText.WriteString(cleanedChapter)
	docCtx.FullText.WriteString("\n")
	docCtx.FullText.WriteString(cleanedText)
	docCtx.FullText.WriteString("\n")

	// 去重并添加 section
	if cleanedChapter != "" && !contains(docCtx.Sections, cleanedChapter) {
		docCtx.Sections = append(docCtx.Sections, cleanedChapter)
	}
	docCtx.mu.Unlock()

	chunkRes := &models.Chunk{
		ChunkID:         chunk.ChunkId,
		FileID:          chunk.FileId,
		ChunkIndex:      chunk.ChunkIndex,
		Chapter:         cleanedChapter,
		ChunkText:       cleanedText,
		EmbeddingVector: pgvector.NewVector(chunk.EmbeddingVector),
		CreatedAt:       time.Now(),
	}

	return cs.chunkRepo.Create(ctx, chunkRes)
}

// contains 辅助函数：检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
