package services

import (
	"context"
	"fmt"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/cache"
	"go_chat_backend/platform/storage"
	"go_chat_backend/repository"
	"time"

	"github.com/google/uuid"
)

type DocumentService struct {
	chatRepo            repository.ChatRepository
	docRepo             repository.DocumentRepository
	messageQueueService cache.MessageQueue
	storageService      *storage.Service
	cacheService        cache.CacheService
	llmService          LLMService
	llmConfigService    *LLMConfigService
	ragService          *RagModeService
}

func NewDocumentService(
	docRepo repository.DocumentRepository,
	chatRepo repository.ChatRepository,
	messageQueueService cache.MessageQueue,
	storageService *storage.Service,
	cacheService cache.CacheService,
	llmConfigService *LLMConfigService,
	ragService *RagModeService) *DocumentService {
	return &DocumentService{
		docRepo:             docRepo,
		chatRepo:            chatRepo,
		messageQueueService: messageQueueService,
		storageService:      storageService,
		cacheService:        cacheService,
		llmConfigService:    llmConfigService,
		ragService:          ragService,
	}
}

func (s *DocumentService) RequestUpload(ctx context.Context, req models.UploadReq) (*models.UploadResp, error) {
	docID := uuid.New().String()
	if req.FileSize > 50*1024*1024 {
		logging.Logger.Error("file too large: max 50MB")
		return nil, fmt.Errorf("file too large: max 50MB")
	}
	if req.ContentType != "application/pdf" {
		logging.Logger.Error("unsupported file type: only pdf")
		return nil, fmt.Errorf("unsupported file type: only pdf")
	}
	// ✅ 修复：使用固定的最大值（50MB），而不是实际文件大小
	// 这样可以避免因为 HTTP 头等额外数据导致超过限制
	res, err := s.storageService.GeneratePresignedPostUpload(
		req.FileName, 50*1024*1024, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %v", err)
	}
	presignedUrl, err := s.storageService.GeneratePresignedGetDownload(res.FileKey, res.Expires)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %v", err)
	}
	docBaseInfo := models.DocumentMeta{
		FileID:          docID,
		UserID:          req.UserID,
		URL:             presignedUrl,
		Filename:        req.FileName,
		TotalPages:      0,
		EstimatedChunks: 0,
		FileHash:        "",
		FileSize:        req.FileSize,
		CreatedAt:       time.Now(),
		FileKey:         res.FileKey,
		Root:            "",
		Status:          models.StatusProcessing,
		ChunksReceived:  0,
		ChunksFailed:    0,
		ChunksStored:    0,
		StartedAt:       nil,
		CompletedAt:     nil,
	}

	err = s.docRepo.Create(ctx, &docBaseInfo)
	if err != nil {
		logging.Logger.Error("failed to create document", "error", err, "docID", docID)
		return nil, fmt.Errorf("failed to create document: %v", err)
	}
	return res, err
}

func (s *DocumentService) ConfirmUpload(ctx context.Context, req models.ConfirmUploadReq) (*models.ConfirmUploadResp, error) {
	reqMode := req.RagMode == "true"
	go func() {
		if err := s.ragService.SetRagMode(ctx, req.DocId, reqMode); err != nil {
			logging.Logger.Error("fail to set RAG mode", "error", err, "docID", req.DocId)
		}
	}()
	info, err := s.docRepo.GetByID(ctx, req.DocId)
	if err != nil {
		logging.Logger.Error("fail GetBaseInfo", err)
		return nil, err
	}
	ok, err := s.storageService.FileExists(info.FileKey)
	if err != nil {
		logging.Logger.Error("fail FileExists", "error", err)
		return nil, err
	} else if !ok {
		logging.Logger.Error("file does not exist in storage")
		return nil, fmt.Errorf("file does not exist in storage")
	}
	if info.Status != models.StatusProcessing {
		logging.Logger.Error("fail ConfirmUpload, docID already exists")
		return nil, fmt.Errorf("fail ConfirmUpload, docID already exists")
	}
	etlTask := models.EtlTask{
		DocID:     info.FileID,
		FileName:  info.Filename,
		URL:       info.URL,
		UserID:    info.UserID,
		CreatedAt: time.Now(),
		RagMode:   req.RagMode,
	}
	if err = s.messageQueueService.PushToQueue("upload_tasks", etlTask); err != nil {
		logging.Logger.Error("fail PushToQueue", err)
		return nil, err
	}

	return &models.ConfirmUploadResp{
		Message: "Upload confirmed successfully",
		DocId:   info.FileID,
		Status:  "queued",
	}, nil
}
func (s *DocumentService) GenerateDocumentSummary(docID, userID string, chunkService *ChunkService) (string, error) {
	// 从 ChunkService 的上下文获取 FullText
	chunkService.mu.RLock()
	docCtx, exists := chunkService.docContexts[docID]
	chunkService.mu.RUnlock()

	if !exists {
		logging.Logger.Error("fail GenerateDocumentSummary, document context not found")
		return "", fmt.Errorf("fail GenerateDocumentSummary, document context not found")
	}

	docCtx.mu.Lock()
	fullText := docCtx.FullText.String()
	docCtx.mu.Unlock()

	// 获取用户的 LLM 配置
	llmConfig, err := s.llmConfigService.GetUserLLMConfig(context.Background(), userID)
	if err != nil {
		logging.Logger.Error("fail to get LLM config for summary", "error", err, "userID", userID)
		return "", fmt.Errorf("LLM configuration required for generating summary: %w", err)
	}

	logging.Logger.Info("GenerateDocumentSummary with LLM config",
		"userID", userID,
		"docID", docID,
		"provider", llmConfig.Provider,
		"model", llmConfig.Model,
		"apiKey", MaskAPIKey(llmConfig.APIKey),
	)

	msg := `
	You are an expert researcher.
	Please read the following academic paper carefully and summarize:
	1. The main research topic and its category.
	2. The problem the paper addresses.
	3. The proposed method and its novelty.
	4. The key results and findings.
	5. Limitations or open questions.
	6. The overall significance.

	Paper content:
	`
	prompt := msg + fullText
	summary, err := s.llmService.CallLLM(prompt, llmConfig.Provider, llmConfig.Model, llmConfig.APIKey)
	if err != nil {
		logging.Logger.Error("fail GenerateDocumentSummary", "error", err)
		return "", err
	}
	if len(summary) > 3000 {
		summary = summary[:3000]
	}

	rootID := uuid.New().String()
	node := &models.ChatNode{
		ID:        rootID,
		ParentID:  "",
		FileID:    docID,
		Question:  msg,
		Answer:    summary,
		CreatedAt: time.Now(),
	}
	err = s.chatRepo.Create(context.Background(), node)
	if err != nil {
		logging.Logger.Error("fail GenerateDocumentSummary", err)
		return "", err
	}
	if err = s.docRepo.UpdateRoot(context.Background(), docID, rootID); err != nil {
		logging.Logger.Error("fail GenerateDocumentSummary", err)
		return "", err
	}
	logging.Logger.Info(
		"GenerateDocumentSummary",
		"docID", docID,
		"summary", summary,
		"rootID", rootID,
	)
	return summary, nil
}

func (s *DocumentService) GetSections(ctx context.Context, docID string) ([]string, error) {
	docBaseInfo, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		logging.Logger.Error("fail GetBaseInfo", err)
		return nil, err
	}
	res := docBaseInfo.Sections
	err = s.cacheService.SetCache(docID, res, 24*time.Hour)
	if err != nil {
		logging.Logger.Error("fail to set section cache", err)
		return nil, err
	}
	return res, nil
}

func (s *DocumentService) GetDocumentByID(ctx context.Context, docID string) (*models.DocumentMeta, error) {
	return s.docRepo.GetByID(ctx, docID)
}

func (s *DocumentService) UpdateSections(ctx context.Context, docID string, sections []string) error {
	// 更新数据库
	doc := &models.DocumentMeta{
		Sections: sections,
	}
	if err := s.docRepo.UpdateMetadata(ctx, docID, doc); err != nil {
		logging.Logger.Error("fail UpdateSections", "error", err)
		return err
	}

	// 更新缓存
	if err := s.cacheService.SetCache(docID, sections, 24*time.Hour); err != nil {
		logging.Logger.Error("fail to set section cache", "error", err)
	}

	return nil
}
