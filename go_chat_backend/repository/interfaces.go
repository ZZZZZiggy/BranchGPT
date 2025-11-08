package repository

import (
	"context"
	"go_chat_backend/models"
)

type DocumentRepository interface {
	Create(ctx context.Context, doc *models.DocumentMeta) error
	//CreateBaseInfo(ctx context.Context, info *models.DocBaseInfo) error

	GetByID(ctx context.Context, fileID string) (*models.DocumentMeta, error)
	GetByHash(ctx context.Context, fileHash string) (*models.DocumentMeta, error)

	UpdateStatus(ctx context.Context, fileID string, status string) error
	UpdateProcessingStats(ctx context.Context, fileID string, received, stored, failed int32) error
	UpsertByHash(ctx context.Context, doc *models.DocumentMeta) error
	UpdateRoot(ctx context.Context, fileID string, rootID string) error
	UpdateMetadata(ctx context.Context, fileID string, doc *models.DocumentMeta) error // ✅ 新增
	//MarkAsCompleted(ctx context.Context, fileID string) error
	//MarkAsFailed(ctx context.Context, fileID string) error
	//
	//Delete(ctx context.Context, fileID string) error
	//
	//CheckDuplicate(ctx context.Context, fileHash, fileID string) (bool, error)

}

type ChunkRepository interface {
	BatchCreate(ctx context.Context, chunks []*models.Chunk) error

	Create(ctx context.Context, chunk *models.Chunk) error

	GetByFileID(ctx context.Context, fileID string) ([]*models.Chunk, error)
	GetByID(ctx context.Context, chunkID string) (*models.Chunk, error)

	SearchSimilar(ctx context.Context, embedding []float32, limit int) ([]*models.Chunk, error)

	CountByFileID(ctx context.Context, fileID string) (int64, error)
	GetNodeBySection(ctx context.Context, section string, fileID string) (*models.Chunk, error)
}

type ChatRepository interface {
	Create(ctx context.Context, node *models.ChatNode) error
	GetChatHistory(ctx context.Context, fileID string, nodeID string) ([]*models.ChatNode, error)
	GetChatChildren(ctx context.Context, fileID string, nodeID string) ([]*models.ChatNode, error)
	GetNodeByID(ctx context.Context, nodeID string, fileID string) (*models.ChatNode, error)
}
