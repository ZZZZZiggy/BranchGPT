package bootstrap

import (
	"go_chat_backend/platform/database"
	"go_chat_backend/repository"
)

type Repositories struct {
	ChunkRepository    repository.ChunkRepository
	DocumentRepository repository.DocumentRepository
	ChatRepository     repository.ChatRepository
}

func NewRepositories(db *database.DB) *Repositories {
	sqlDB := db.GetDatabase()
	return &Repositories{
		ChunkRepository:    repository.NewChunkRepository(sqlDB),
		DocumentRepository: repository.NewDocumentRepository(sqlDB),
		ChatRepository:     repository.NewChatRepository(sqlDB),
	}
}
