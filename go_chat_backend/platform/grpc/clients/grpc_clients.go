package clients

import (
	"context"
	"fmt"
	"go_chat_backend/config"
	"go_chat_backend/pkg/logging"
	pb "go_chat_backend/platform/proto/cognicore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"time"
)

type GrpcClients struct {
	// connections
	embeddingConn *grpc.ClientConn

	// servers
	EmbeddingClient pb.EmbeddingServiceClient
}

func NewGrpcClients(cfg *config.Config) *GrpcClients {
	clients := &GrpcClients{}
	embeddingConn, err := createGrpcConnection(cfg.GrpcEmbeddingAddr)
	if err != nil {
		logging.Logger.Error("fail createGrpcConnection", err)
		return nil
	}
	clients.embeddingConn = embeddingConn
	clients.EmbeddingClient = pb.NewEmbeddingServiceClient(embeddingConn)

	return clients
}

func createGrpcConnection(address string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),

		// Keep-Alive
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),

		// default settings
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024),
			grpc.MaxCallSendMsgSize(100*1024*1024),
		),
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	if ok := conn.WaitForStateChange(ctx, connectivity.Idle); ok {
		err := conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	return conn, nil
}
func (c *GrpcClients) Close() error {
	var errs []error

	if c.embeddingConn != nil {
		if err := c.embeddingConn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Embedding connection: %w", err))
		} else {
			logging.Logger.Info("Embedding connection closed")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}

	return nil
}
