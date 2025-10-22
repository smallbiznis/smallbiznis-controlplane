package orchestrator

import (
	"context"
	"smallbiznis-controlplane/pkg/config"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	orchestratorv1 "github.com/smallbiznis/go-genproto/smallbiznis/orchestrator/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var Module = fx.Module("orchestrator.service",
	fx.Provide(NewService),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("orchestrator.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	orchestratorv1.RegisterOrchestratorServiceServer(server, service)
	grpc_health_v1.RegisterHealthServer(server, service)
}

type registerServiceHandlerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Mux       *runtime.ServeMux
	Config    *config.Config
	Service   *Service
}

func registerServiceHandlerServer(p registerServiceHandlerParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := orchestratorv1.RegisterOrchestratorServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register tenant http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
