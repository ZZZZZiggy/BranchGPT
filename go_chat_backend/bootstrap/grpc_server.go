package bootstrap

import (
	"fmt"
	"go_chat_backend/config"
	"go_chat_backend/platform/grpc/servers"
)

type GrpcServices struct {
	IngestService *servers.IngestService
}

func NewGrpcServices(cfg *config.Config, services *Services, infra *Infrastructure) (*GrpcServices, error) {
	s := &GrpcServices{
		servers.NewIngestService(cfg, services.ChunkService, services.DocService, infra.EventPublisher),
	}
	if err := s.IngestService.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ingest service: %w", err)
	}
	return s, nil
}
func (s *GrpcServices) Shutdown() error {
	if s.IngestService != nil {
		return s.IngestService.Stop()
	}
	return nil
}
