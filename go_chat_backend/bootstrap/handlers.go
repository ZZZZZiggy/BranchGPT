package bootstrap

import "go_chat_backend/handlers"

type Handlers struct {
	DocHandler  *handlers.DocHandler
	WSHandler   *handlers.WSHandler
	ChatHandler *handlers.ChatHandler
}

func NewHandlers(services *Services, infra *Infrastructure) *Handlers {
	res := &Handlers{}
	d := handlers.NewDocHandler(services.DocService, services.GrpcServices, services.LLMConfigService)
	res.DocHandler = d
	w := handlers.NewWSHandler(infra.EventPublisher)
	res.WSHandler = w
	c := handlers.NewChatHandler(services.ChatsService)
	res.ChatHandler = c
	return res
}
