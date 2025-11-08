package services

import (
	"context"
	"fmt"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/cache"
	"go_chat_backend/platform/database"
	"go_chat_backend/repository"
	"time"

	"github.com/google/uuid"
)

type ChatService struct {
	chatRepo         repository.ChatRepository
	docRepo          repository.DocumentRepository
	cacheService     cache.CacheService
	llmService       *LLMService
	llmConfigService *LLMConfigService
}

func NewChatService(db *database.DB, cacheService cache.CacheService, llmService *LLMService, llmConfigService *LLMConfigService) *ChatService {
	return &ChatService{
		chatRepo:         repository.NewChatRepository(db.GetDatabase()),
		docRepo:          repository.NewDocumentRepository(db.GetDatabase()),
		cacheService:     cacheService,
		llmService:       llmService,
		llmConfigService: llmConfigService,
	}
}

func (s *ChatService) GetChatTree(ctx context.Context, fileID string) (*models.ChatTreeNode, error) {
	docMeta, err := s.docRepo.GetByID(ctx, fileID)
	if err != nil {
		logging.Logger.Error("fail GetChatTree", err)
		return nil, err
	}
	rootID := docMeta.Root
	rootNode, err := s.chatRepo.GetNodeByID(ctx, rootID, fileID)
	if err != nil {
		logging.Logger.Error("fail GetChatTree", err)
		return nil, err
	} else if rootNode == nil {
		logging.Logger.Error("fail GetChatTree", "chatNode is nil")
		return nil, fmt.Errorf("chatNode is nil")
	}
	root := &models.ChatTreeNode{
		ID:       rootNode.ID,
		Question: rootNode.Question,
		Answer:   rootNode.Answer,
	}
	queue := []struct {
		Node   *models.ChatTreeNode
		NodeID string
	}{{root, rootID}}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		children, _ := s.chatRepo.GetChatChildren(ctx, fileID, curr.NodeID)
		for _, child := range children {
			childTree := &models.ChatTreeNode{
				ID:       child.ID,
				Question: child.Question,
				Answer:   child.Answer,
			}
			curr.Node.Children = append(curr.Node.Children, childTree)
			queue = append(queue, struct {
				Node   *models.ChatTreeNode
				NodeID string
			}{childTree, child.ID})
		}
	}
	return root, nil
}

func (s *ChatService) AskQuestion(ctx context.Context, fileID string, req models.ChatReq) (*models.ChatRes, error) {
	var ChatHistory []*models.ChatNode
	var err error

	ChatHistory, err = s.GetHistoryByID(ctx, req.ParentID, fileID)
	if err != nil {
		logging.Logger.Error("fail AskQuestion", "error", err)
		return nil, err
	}

	// 获取或使用 LLM 配置（优先使用请求中的配置）
	llmConfig, err := s.llmConfigService.GetOrUseDefault(ctx, req.UserID, req.APIKey, req.Model, req.Provider)
	if err != nil {
		logging.Logger.Error("fail to get LLM config", "error", err, "userID", req.UserID)
		return nil, fmt.Errorf("LLM configuration required: %w", err)
	}

	// 使用脱敏的 API Key 记录日志
	logging.Logger.Info("AskQuestion with LLM config",
		"userID", req.UserID,
		"provider", llmConfig.Provider,
		"model", llmConfig.Model,
		"apiKey", MaskAPIKey(llmConfig.APIKey),
	)

	prompt := s.llmService.BuildPrompt(ChatHistory, req.Question, req.Section, req.FileID, llmConfig.Provider, llmConfig.Model)
	answer, err := s.llmService.CallLLM(prompt, llmConfig.Provider, llmConfig.Model, llmConfig.APIKey)
	if err != nil {
		logging.Logger.Error("fail AskQuestion", "error", err)
		return nil, err
	}
	ID := uuid.New().String()
	newNode := &models.ChatNode{
		ID:        ID,
		FileID:    fileID,
		ParentID:  req.ParentID,
		Answer:    answer,
		CreatedAt: time.Now(),
		Question:  req.Question,
	}
	err = s.chatRepo.Create(ctx, newNode)
	if err != nil {
		return nil, err
	}
	go func() {
		err = s.cacheService.SetCache(req.ParentID, append([]*models.ChatNode{newNode}, ChatHistory...), time.Hour)
		if err != nil {
			logging.Logger.Error("fail to set cache", "error", err)
		}
	}()

	tree, err := s.GetChatTree(ctx, fileID)
	return &models.ChatRes{
		ID:       ID,
		Answer:   answer,
		Question: req.Question,
		Tree:     tree,
	}, err
}

func (s *ChatService) GetHistoryByID(ctx context.Context, ParentID string, fileID string) ([]*models.ChatNode, error) {
	cacheKey := fmt.Sprintf("chat_node:%s:%s", fileID, ParentID)
	var ChatHistory []*models.ChatNode
	var err error
	if ParentID == "" {
		return []*models.ChatNode{}, nil
	}

	// cache
	if cached, ok := s.cacheService.GetCache(cacheKey); ok {
		if ChatHistory, ok = cached.([]*models.ChatNode); ok {
			return ChatHistory, nil
		}
	}

	// db
	ChatHistory, err = s.chatRepo.GetChatHistory(ctx, fileID, ParentID)
	if err != nil {
		logging.Logger.Error("fail AskQuestion", err)
		return nil, err
	}
	// save
	go func() {
		err := s.cacheService.SetCache(cacheKey, ChatHistory, 30*time.Minute)
		if err != nil {
			return
		}
	}()

	return ChatHistory, nil
}
