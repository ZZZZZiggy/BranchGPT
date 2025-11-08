package repository

import (
	"context"
	"go_chat_backend/models"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type documentRepository struct {
	DB *gorm.DB
}

func NewDocumentRepository(db *gorm.DB) DocumentRepository {
	return &documentRepository{DB: db}
}

func (r *documentRepository) Create(ctx context.Context, doc *models.DocumentMeta) error {
	return r.DB.WithContext(ctx).Create(doc).Error
}

func (r *documentRepository) GetByID(ctx context.Context, fileID string) (*models.DocumentMeta, error) {
	var doc models.DocumentMeta
	err := r.DB.WithContext(ctx).Where("file_id = ?", fileID).First(&doc).Error
	return &doc, err
}

func (r *documentRepository) GetByHash(ctx context.Context, fileHash string) (*models.DocumentMeta, error) {
	var doc models.DocumentMeta
	err := r.DB.WithContext(ctx).Where("file_hash = ?", fileHash).First(&doc).Error
	return &doc, err
}

func (r *documentRepository) UpdateStatus(ctx context.Context, fileID string, status string) error {
	return r.DB.WithContext(ctx).Model(&models.DocumentMeta{}).Where("file_id = ?", fileID).Update("status", status).Error
}

func (r *documentRepository) UpdateProcessingStats(ctx context.Context, fileID string, received, stored, failed int32) error {
	return r.DB.WithContext(ctx).Model(&models.DocumentMeta{}).Where("file_id = ?", fileID).Update("received", received).Update("stored", stored).Update("failed", failed).Error
}

func (r *documentRepository) UpsertByHash(ctx context.Context, doc *models.DocumentMeta) error {
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "file_hash"}},
			Where: clause.Where{
				Exprs: []clause.Expression{
					clause.Or(
						clause.Eq{Column: "document_meta.status", Value: models.StatusFailed},
						clause.Eq{Column: "document_meta.status", Value: models.StatusProcessing},
					),
				},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"total_pages",
				"estimated_chunks",
				"started_at",
			}),
		}).
		Create(doc).Error
}

func (r *documentRepository) UpdateRoot(ctx context.Context, fileID string, rootID string) error {
	return r.DB.WithContext(ctx).Model(&models.DocumentMeta{}).Where("file_id = ?", fileID).Update("root", rootID).Error
}
func (r *documentRepository) UpdateMetadata(ctx context.Context, fileID string, doc *models.DocumentMeta) error {
	return r.DB.WithContext(ctx).
		Model(&models.DocumentMeta{}).
		Where("file_id = ?", fileID).
		Updates(map[string]interface{}{
			"file_hash":        doc.FileHash,
			"file_size":        doc.FileSize,
			"total_pages":      doc.TotalPages,
			"estimated_chunks": doc.EstimatedChunks,
			"started_at":       doc.StartedAt,
			"sections":         pq.Array(doc.Sections),  // ← 使用 pq.Array 包装
		}).Error
}
