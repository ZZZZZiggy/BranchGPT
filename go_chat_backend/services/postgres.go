package services

import (
	"fmt"
	"go_chat_backend/models"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitPostgres() error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=prefer TimeZone=UTC",
		os.Getenv("PG_HOST"),
		os.Getenv("PG_USER"),
		os.Getenv("PG_PASSWORD"),
		os.Getenv("PG_DB"),
		os.Getenv("PG_PORT"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get db instance: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	DB = db
	fmt.Println("Connected to Postgres")

	if err := db.AutoMigrate(&models.Document{}); err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}
	if err := db.AutoMigrate(&models.ChatNode{}); err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}
	return nil
}
func SaveToPostgres(ProcessedData map[string]interface{}, docID string, root string) (*models.Document, error) {
	paragraphs, ok := ProcessedData["paragraphs"].([]interface{})
	if !ok || len(paragraphs) == 0 {
		return nil, fmt.Errorf("paragraphs not found or empty")
	}

	firstParagraph, ok := paragraphs[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data structure: first paragraph not found or wrong type")
	}
	title, ok := firstParagraph["chapter"].(string)
	if !ok {
		title = "Untitled Document"
	}

	docMeta := &models.Document{
		DocID:     docID,
		Title:     title,
		Status:    "processed",
		CreatedAt: time.Now(),
		Root:      root,
	}

	if err := DB.Create(&docMeta).Error; err != nil {
		return nil, err
	}
	return docMeta, nil
}

func GetDocument(docID string) (*models.Document, error) {
	var doc models.Document
	err := DB.Where("doc_id = ?", docID).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}
