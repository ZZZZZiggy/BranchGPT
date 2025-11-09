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

func (s *GRPCService) GetEmbedding(text string) ([]float32, error) {
	req := &pb.EmbeddingRequest{
		TaskId: "embedding-" + time.Now().Format("20060102150405"),
		Text:   text,
	}
	r, err := s.clients.EmbeddingClient.GetEmbedding(context.Background(), req)
	if err != nil {
		logging.Logger.Error("fail GetEmbedding", err)
	}
	logging.Logger.Info("embedding", "embedding", r)
	return r.Embeddings, err
}
