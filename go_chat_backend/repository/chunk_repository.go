package repository

import (
	"context"
	"go_chat_backend/models"
	"gorm.io/gorm"
)

type chunkRepository struct {
	DB *gorm.DB
}

func NewChunkRepository(db *gorm.DB) ChunkRepository {
	return &chunkRepository{DB: db}
}

func (r *chunkRepository) BatchCreate(ctx context.Context, chunks []*models.Chunk) error {
	return r.DB.WithContext(ctx).Create(chunks).Error
}

func (r *chunkRepository) Create(ctx context.Context, chunk *models.Chunk) error {
	return r.DB.WithContext(ctx).Create(chunk).Error
}

func (r *chunkRepository) GetByFileID(ctx context.Context, fileID string) ([]*models.Chunk, error) {
	return nil, nil
}
func (r *chunkRepository) GetNodeBySection(ctx context.Context, section string, fileID string) (*models.Chunk, error) {
	var chunk models.Chunk
	err := r.DB.WithContext(ctx).Where("chapter = ? AND file_id = ?", section, fileID).First(&chunk).Error
	return &chunk, err
}
func (r *chunkRepository) SearchSimilar(ctx context.Context, embedding []float32, limit int) ([]*models.Chunk, error) {
	return nil, nil
}

func (r *chunkRepository) GetByID(ctx context.Context, chunkID string) (*models.Chunk, error) {
	return nil, nil
}

func (r *chunkRepository) CountByFileID(ctx context.Context, fileID string) (int64, error) {
	return 0, nil
}
