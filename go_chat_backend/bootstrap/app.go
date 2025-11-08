package bootstrap

import (
	"go_chat_backend/config"
	"go_chat_backend/pkg/logging"
)

type App struct {
	Cfg            *config.Config
	Infrastructure *Infrastructure
	Repositories   *Repositories
	Services       *Services
	GrpcServices   *GrpcServices
	Handlers       *Handlers
}

func NewApp(cfg *config.Config) (*App, error) {
	app := &App{Cfg: cfg}
	infra, err := NewInfrastructure(cfg)
	if err != nil {
		logging.Logger.Error("fail NewInfrastructure", err)
		return nil, err
	}
	app.Infrastructure = infra

	// repos
	repos := NewRepositories(infra.DB)
	app.Repositories = repos

	// services
	services := NewServices(repos, infra)
	app.Services = services

	handlers := NewHandlers(services, infra)
	app.Handlers = handlers

	// grpc server
	GrpcServices, err := NewGrpcServices(cfg, services, infra)
	if err != nil {
		logging.Logger.Error("fail NewGrpcServices", err)
		return nil, err
	}

	app.GrpcServices = GrpcServices

	return app, nil
}

// Shutdown infra
func (a *App) Shutdown() error {
	if a == nil {
		return nil
	}
	if a.GrpcServices != nil {
		if err := a.GrpcServices.Shutdown(); err != nil {
			return err
		}
	}
	if a.Infrastructure != nil {
		if err := a.Infrastructure.Shutdown(); err != nil {
			return err
		}
	}
	return nil
}
