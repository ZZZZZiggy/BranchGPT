package bootstrap

import "go_chat_backend/services"

type Services struct {
	DocService       *services.DocumentService
	ChunkService     *services.ChunkService
	GrpcServices     *services.GRPCService
	ChatsService     *services.ChatService
	LLMConfigService *services.LLMConfigService
}

func NewServices(repos *Repositories, infra *Infrastructure) *Services {
	res := &Services{}

	// LLM 配置服务
	llmConfigService := services.NewLLMConfigService(infra.Cache)
	res.LLMConfigService = llmConfigService

	// 文档服务（注入 LLM 配置服务）
	docService := services.NewDocumentService(repos.DocumentRepository, repos.ChatRepository, infra.Queue, infra.Storage, infra.Cache, llmConfigService)
	res.DocService = docService

	chunkService := services.NewChunkService(infra.DB)
	res.ChunkService = chunkService
	grpcServices := services.NewGRPCService(infra.GrpcClients)
	res.GrpcServices = grpcServices

	// LLM 服务（注入 GRPCService）
	llmServices := services.NewLLMService(repos.ChunkRepository, grpcServices)
	chatServices := services.NewChatService(infra.DB, infra.Cache, llmServices, llmConfigService)
	res.ChatsService = chatServices
	return res
}
