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
	var chunks []*models.Chunk
	err := r.DB.WithContext(ctx).
		Where("file_id = ?", fileID).
		Order("chunk_index ASC").
		Find(&chunks).Error
	if err != nil {
		return nil, err
	}
	return chunks, nil
}

func (r *chunkRepository) GetNodeBySection(ctx context.Context, section string, fileID string) (*models.Chunk, error) {
	var chunk models.Chunk
	err := r.DB.WithContext(ctx).Where("chapter = ? AND file_id = ?", section, fileID).First(&chunk).Error
	return &chunk, err
}

func (r *chunkRepository) SearchSimilar(ctx context.Context, embedding []float32, limit int) ([]*models.Chunk, error) {
	var chunks []*models.Chunk

	// 将 []float32 转换为 pgvector.Vector
	queryVector := make([]float64, len(embedding))
	for i, v := range embedding {
		queryVector[i] = float64(v)
	}

	// 使用余弦相似度进行向量搜索
	// <=> 是 pgvector 的余弦距离操作符（值越小越相似）
	// 也可以使用：
	// <-> L2 距离（欧几里得距离）
	// <#> 负内积（最大内积搜索）
	err := r.DB.WithContext(ctx).
		Select("chunk_id, file_id, chunk_index, chapter, chunk_text, embedding_vector, created_at").
		Order(gorm.Expr("embedding_vector <=> ?", queryVector)).
		Limit(limit).
		Find(&chunks).Error

	if err != nil {
		return nil, err
	}

	return chunks, nil
}

func (r *chunkRepository) GetByID(ctx context.Context, chunkID string) (*models.Chunk, error) {
	var chunk models.Chunk
	err := r.DB.WithContext(ctx).Where("chunk_id = ?", chunkID).First(&chunk).Error
	if err != nil {
		return nil, err
	}
	return &chunk, nil
}

func (r *chunkRepository) CountByFileID(ctx context.Context, fileID string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&models.Chunk{}).
		Where("file_id = ?", fileID).
		Count(&count).Error
	return count, err
}
