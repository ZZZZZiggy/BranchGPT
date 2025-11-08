package models

import (
	"time"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type EtlTask struct {
	DocID     string `gorm:"primaryKey"`
	FileName  string
	UserID    string
	CreatedAt time.Time
	URL       string
}

type DocumentMeta struct {
	// 主键字段
	FileID string `gorm:"column:file_id;type:varchar(255);primaryKey" json:"file_id"`

	// 基本信息字段
	UserID          string    `gorm:"column:user_id;type:varchar(255);not null;index:idx_user_id" json:"user_id"`
	URL             string    `gorm:"column:url;type:text;not null" json:"url"`
	Filename        string    `gorm:"column:filename;type:varchar(512);not null" json:"filename"`
	TotalPages      int32     `gorm:"column:total_pages;type:int" json:"total_pages"`
	EstimatedChunks int32     `gorm:"column:estimated_chunks;type:int" json:"estimated_chunks"`
	FileHash        string    `gorm:"column:file_hash;type:varchar(64);uniqueIndex:idx_file_hash" json:"file_hash"`
	FileSize        int64     `gorm:"column:file_size;type:bigint" json:"file_size"`
	CreatedAt       time.Time      `gorm:"column:created_at;type:timestamp" json:"created_at"`
	FileKey         string         `gorm:"column:file_key;type:varchar(255);not null;index:idx_file_key" json:"file_key"`
	Root            string         `gorm:"column:root;type:varchar(255);index:idx_root" json:"root"`
	Sections        pq.StringArray `gorm:"column:sections;type:text[]" json:"sections"`

	// 状态追踪字段
	Status         string `gorm:"column:status;type:varchar(50);default:'processing';index:idx_status" json:"status"`
	ChunksReceived int32  `gorm:"column:chunks_received;type:int;default:0" json:"chunks_received"`
	ChunksStored   int32  `gorm:"column:chunks_stored;type:int;default:0" json:"chunks_stored"`
	ChunksFailed   int32  `gorm:"column:chunks_failed;type:int;default:0" json:"chunks_failed"`

	// 时间戳字段
	StartedAt   *time.Time `gorm:"column:started_at;type:timestamp;default:now()" json:"started_at"`
	CompletedAt *time.Time `gorm:"column:completed_at;type:timestamp" json:"completed_at,omitempty"`
}

// TableName 指定表名
func (DocumentMeta) TableName() string {
	return "document_meta"
}

// DocumentStatus 文档状态常量
const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// BeforeCreate GORM 钩子：创建前设置默认值
func (d *DocumentMeta) BeforeCreate(tx *gorm.DB) error {
	if d.Status == "" {
		d.Status = StatusProcessing
	}
	if d.StartedAt == nil {
		now := time.Now()
		d.StartedAt = &now
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	return nil
}

// IsCompleted 检查文档是否已完成
func (d *DocumentMeta) IsCompleted() bool {
	return d.Status == StatusCompleted
}

// IsFailed 检查文档是否失败
func (d *DocumentMeta) IsFailed() bool {
	return d.Status == StatusFailed
}

// IsProcessing 检查文档是否正在处理
func (d *DocumentMeta) IsProcessing() bool {
	return d.Status == StatusProcessing
}

// MarkAsCompleted 标记文档为已完成
func (d *DocumentMeta) MarkAsCompleted(db *gorm.DB) error {
	now := time.Now()
	d.Status = StatusCompleted
	d.CompletedAt = &now
	return db.Model(d).Updates(map[string]interface{}{
		"status":       StatusCompleted,
		"completed_at": now,
	}).Error
}

// MarkAsFailed 标记文档为失败
func (d *DocumentMeta) MarkAsFailed(db *gorm.DB) error {
	now := time.Now()
	d.Status = StatusFailed
	d.CompletedAt = &now
	return db.Model(d).Updates(map[string]interface{}{
		"status":       StatusFailed,
		"completed_at": now,
	}).Error
}

type Chunk struct {
	// 主键字段
	ChunkID string `gorm:"column:chunk_id;type:varchar(255);primaryKey" json:"chunk_id"`

	// 外键字段
	FileID string `gorm:"column:file_id;type:varchar(255);not null;index:idx_file_id" json:"file_id"`

	// 基本信息字段
	ChunkIndex int32  `gorm:"column:chunk_index;type:int;not null" json:"chunk_index"`
	Chapter    string `gorm:"column:chapter;type:varchar(512)" json:"chapter"`
	ChunkText  string `gorm:"column:chunk_text;type:text;not null" json:"chunk_text"`

	// 向量字段（pgvector）
	EmbeddingVector pgvector.Vector `gorm:"column:embedding_vector;type:vector(384)" json:"embedding_vector"`

	// 时间戳字段
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:now()" json:"created_at"`
}

// TableName 指定表名
func (Chunk) TableName() string {
	return "chunks"
}

// BeforeCreate GORM 钩子：创建前设置默认值
func (c *Chunk) BeforeCreate(tx *gorm.DB) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	return nil
}
