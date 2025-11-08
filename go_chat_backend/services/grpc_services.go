package services

import (
	"context"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/grpc/clients"
	pb "go_chat_backend/platform/proto/cognicore"
	"time"
)

type GRPCService struct {
	clients *clients.GrpcClients
}

func NewGRPCService(clients *clients.GrpcClients) *GRPCService {
	return &GRPCService{clients: clients}
}

func (s *GRPCService) SendAPIKey(ctx context.Context, taskID, apiKey, provider string) error {
	req := &pb.APIKeyRequest{
		TaskId:   taskID,
		ApiKey:   apiKey,
		Provider: provider,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	r, err := s.clients.APIKeyClient.ProvideAPIKey(ctx, req)
	if err != nil {
		logging.Logger.Error("fail ProvideAPIKey", err)
		return err
	}
	if r.Success {
		logging.Logger.Info("successfully sent api_key")
	} else {
		logging.Logger.Error("failed on sending api_key")
		return err
	}
	logging.Logger.Info("api_key", "api_key", r)
	return nil
}

func (s *GRPCService) GetEmbedding(text, apiKey, provider string) ([]float32, error) {
	req := &pb.EmbeddingRequest{
		TaskId:   "embedding-" + time.Now().Format("20060102150405"),
		Text:     text,
		ApiKey:   apiKey,
		Provider: provider,
	}
	r, err := s.clients.EmbeddingClient.GetEmbedding(context.Background(), req)
	if err != nil {
		logging.Logger.Error("fail GetEmbedding", err)
	}
	logging.Logger.Info("embedding", "embedding", r)
	return r.Embeddings, err
}
