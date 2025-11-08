package servers

import (
	"context"
	"fmt"
	"go_chat_backend/config"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/platform/events"
	pb "go_chat_backend/platform/proto/cognicore"
	"go_chat_backend/services"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"
)

type IngestService struct {
	port string
	pb.UnimplementedIngestServiceServer
	chunkService    *services.ChunkService
	documentService *services.DocumentService
	eventPublisher  *events.EventPublisher
	server          *grpc.Server
	listener        net.Listener
}

func NewIngestService(
	cfg *config.Config,
	chunkService *services.ChunkService,
	documentService *services.DocumentService,
	eventPublisher *events.EventPublisher,
) *IngestService {
	return &IngestService{
		port:            cfg.GoGrpcIngestPort,
		chunkService:    chunkService,
		documentService: documentService,
		eventPublisher:  eventPublisher,
	}
}
func (s *IngestService) Start() error {
	lis, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		logging.Logger.Error("fail NewIngestService", "error", err)
		return err
	}
	s.listener = lis
	s.server = grpc.NewServer()
	pb.RegisterIngestServiceServer(s.server, s)
	logging.Logger.Info("start grpc server", "port", s.port)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			logging.Logger.Error("fail grpc server", "error", err)
		}
	}()
	return nil
}

func (s *IngestService) Stop() error {
	if s.server != nil {
		s.server.GracefulStop()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *IngestService) IngestDocument(stream pb.IngestService_IngestDocumentServer) error {
	logging.Logger.Info("start getting stream")

	var fileId string
	var chunksReceived, chunksStored, chunksFailed int32
	var metadata *pb.DocumentMetadata
	timeStart := time.Now()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// processed all chunks
			if chunksFailed == 0 {
				// 从 metadata 获取 userID 并生成摘要
				summary, err := s.documentService.GenerateDocumentSummary(fileId, metadata.UserId, s.chunkService)
				// failed to get summary
				if err != nil {
					logging.Logger.Error("fail GenerateDocumentSummary", "error", err)
					// 即使 summary 失败也继续处理
					err = s.eventPublisher.PublishDocumentEvent(&models.DocumentEvent{
						Type:    models.EventDocumentCompleted,
						DocID:   fileId,
						UserID:  metadata.UserId,
						Status:  "completed_without_summary",
						Message: "Document processed but summary generation failed: " + err.Error(),
					})
					if err != nil {
						return fmt.Errorf("failed to publish event: %w", err)
					}
				}
				// succeed - 从 ChunkService 的上下文获取 sections
				sections := s.chunkService.GetSections(fileId)

				// 更新数据库中的 sections
				if len(sections) > 0 {
					if err := s.documentService.UpdateSections(context.Background(), fileId, sections); err != nil {
						logging.Logger.Error("fail UpdateSections", "error", err)
					}
				}

				err = s.eventPublisher.PublishDocumentEvent(&models.DocumentEvent{
					Type:     models.EventDocumentCompleted,
					DocID:    fileId,
					UserID:   metadata.UserId,
					Status:   "completed",
					Message:  "completed",
					Summary:  summary,
					Sections: sections,
					Progress: &models.ProgressInfo{
						ChunksFailed:   chunksFailed,
						ChunksReceived: chunksReceived,
						ChunksStored:   chunksStored,
						Percentage:     100,
					},
				})
				if err != nil {
					return err
				}

				// 清理文档处理上下文
				s.chunkService.CleanupContext(fileId)
			} else {
				// cannot process all chunks
				err := s.eventPublisher.PublishDocumentEvent(&models.DocumentEvent{
					Type:    models.EventDocumentFailed,
					DocID:   fileId,
					UserID:  metadata.UserId,
					Status:  "failed",
					Message: fmt.Sprintf("Processing failed: %d chunks failed", chunksFailed),
					Progress: &models.ProgressInfo{
						ChunksReceived: chunksReceived,
						ChunksStored:   chunksStored,
						ChunksFailed:   chunksFailed,
					},
				})
				if err != nil {
					return err
				}
			}
			// send response
			response := &pb.IngestResponse{
				Success:          chunksFailed == 0,
				Message:          fmt.Sprintf("finish %d chunks", chunksReceived),
				ChunksReceived:   chunksReceived,
				ChunksStored:     chunksStored,
				ChunksFailed:     chunksFailed,
				ProcessingTimeMs: time.Since(timeStart).Milliseconds(),
				FileId:           fileId,
			}
			return stream.SendAndClose(response)
		}
		// cannot finish processing
		if err != nil {
			logging.Logger.Error("fail IngestDocument", "error", err)
			return err
		}
		switch requestType := req.RequestType.(type) {
		case *pb.IngestRequest_Metadata:
			metadata = requestType.Metadata
			fileId = metadata.FileId
			if err := s.chunkService.ProcessDocumentMetadata(metadata); err != nil {
				return err
			}
			err := s.eventPublisher.PublishDocumentEvent(&models.DocumentEvent{
				Type:    models.EventDocumentProcessing,
				DocID:   fileId,
				UserID:  metadata.UserId,
				Status:  "processing",
				Message: "Document processing started",
				Progress: &models.ProgressInfo{
					TotalChunks: metadata.EstimatedChunks,
					Percentage:  0,
				},
			})
			if err != nil {
				return err
			}
			logging.Logger.Info("receive metadata",
				"fileId", fileId,
				"filename", metadata.Filename,
				"estimated_chunks", metadata.EstimatedChunks,
			)
		case *pb.IngestRequest_Chunk:
			chunk := requestType.Chunk
			chunksReceived++

			logging.Logger.Info("receive chunk",
				"chunk_id", chunk.ChunkId,
				"chunk_index", chunk.ChunkIndex,
				"text_length", len(chunk.ChunkText),
			)
			if err := s.chunkService.ProcessChunk(chunk); err != nil {
				chunksFailed++
				logging.Logger.Error("fail processChunk", "error", err)
			} else {
				chunksStored++
			}
			if chunksReceived%10 == 0 && metadata != nil {
				percentage := 0
				if metadata.EstimatedChunks > 0 {
					percentage = int(float32(chunksStored) / float32(metadata.EstimatedChunks) * 100)
				}
				err := s.eventPublisher.PublishDocumentEvent(&models.DocumentEvent{
					Type:    models.EventDocumentProcessing,
					DocID:   fileId,
					UserID:  metadata.UserId,
					Status:  "processing",
					Message: fmt.Sprintf("Processing: %d/%d chunks", chunksStored, metadata.EstimatedChunks),
					Progress: &models.ProgressInfo{
						ChunksReceived: chunksReceived,
						ChunksStored:   chunksStored,
						ChunksFailed:   chunksFailed,
						TotalChunks:    metadata.EstimatedChunks,
						Percentage:     percentage,
					},
				})
				if err != nil {
					logging.Logger.Error("fail PublishDocumentEvent", "error", err)
					return err
				}
			}
		}
	}
}
