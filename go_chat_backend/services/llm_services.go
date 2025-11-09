package services

import (
	"context"
	"fmt"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/repository"
	"go_chat_backend/utils"
	"strings"
)

type LLMService struct {
	chunkRepository repository.ChunkRepository
	GRPCService     *GRPCService
}

func NewLLMService(chunkRepository repository.ChunkRepository, grpcService *GRPCService) *LLMService {
	return &LLMService{
		chunkRepository: chunkRepository,
		GRPCService:     grpcService,
	}
}
func (s *LLMService) BuildPrompt(history []*models.ChatNode, question, section, fileID, provider, apikey string, ragMode bool) string {
	// assume
	var builder strings.Builder
	builder.WriteString("You are an AI assistant helping the user understand a technical document.\n\n")

	if section != "" {
		ctx := context.Background()
		chunkContext, err := s.chunkRepository.GetNodeBySection(ctx, section, fileID)
		if err != nil {
			logging.Logger.Error("fail GetNodeBySection", err)
		}
		builder.WriteString(fmt.Sprintf("The following questions are about Section %s:\n%s\n\n", section, chunkContext.ChunkText))
	}
	if ragMode {
		embedding, err := s.GRPCService.GetEmbedding(question)
		if err != nil {
			logging.Logger.Error("fail GetEmbedding", err)
		}
		similar, err := s.chunkRepository.SearchSimilar(context.Background(), embedding, 1)
		if err != nil {
			logging.Logger.Error("fail SearchSimilar", err)
		}
		if len(similar) == 0 {
			logging.Logger.Error("fail SearchSimilar", "similar", similar)
		}
		builder.WriteString(fmt.Sprintf("The following context are similar to the question:\n%s\n\n", similar[0].ChunkText))
	}

	if len(history) == 0 {
		return builder.String()
	}

	builder.WriteString("Previous conversation:\n")
	for i, node := range history {
		builder.WriteString(fmt.Sprintf("Q%d: %s\n", i+1, node.Question))
		builder.WriteString(fmt.Sprintf("A%d: %s\n", i+1, node.Answer))
	}

	builder.WriteString("\nNow answer the following question in context of the above:\n")
	builder.WriteString("Q: " + question + "\n")

	return builder.String()
}

func (s *LLMService) CallLLM(prompt, provider, modelName, APIKey string) (string, error) {
	switch provider {
	case "OpenAI":
		return utils.CallOpenAI(prompt, modelName, APIKey)
	case "Gemini":
		return utils.CallGemini(prompt, modelName, APIKey)
	default:
		logging.Logger.Error("invalid provider", "provider", provider)
		return "", fmt.Errorf("invalid provider")
	}
}
