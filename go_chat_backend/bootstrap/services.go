package bootstrap

import "go_chat_backend/services"

type Services struct {
	DocService       *services.DocumentService
	ChunkService     *services.ChunkService
	GrpcServices     *services.GRPCService
	ChatsService     *services.ChatService
	LLMConfigService *services.LLMConfigService
	RagService       *services.RagModeService
}

func NewServices(repos *Repositories, infra *Infrastructure) *Services {
	res := &Services{}

	llmConfigService := services.NewLLMConfigService(infra.Cache)
	res.LLMConfigService = llmConfigService

	ragService := services.NewRagModeService(infra.Cache, repos.DocumentRepository)
	res.RagService = ragService

	docService := services.NewDocumentService(repos.DocumentRepository, repos.ChatRepository, infra.Queue, infra.Storage, infra.Cache, llmConfigService, ragService)
	res.DocService = docService

	chunkService := services.NewChunkService(infra.DB)
	res.ChunkService = chunkService
	grpcServices := services.NewGRPCService(infra.GrpcClients)
	res.GrpcServices = grpcServices

	// LLM 服务（注入 GRPCService）
	llmServices := services.NewLLMService(repos.ChunkRepository, grpcServices)
	chatServices := services.NewChatService(repos.ChatRepository, repos.DocumentRepository, infra.Cache, llmServices, llmConfigService, ragService)
	res.ChatsService = chatServices


	return res
}
