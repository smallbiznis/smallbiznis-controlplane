package loyalty

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var Module = fx.Module("loyalty.service",
	fx.Provide(NewService),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("loyalty.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	loyaltyv1.RegisterPointServiceServer(server, service)
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

			if err := loyaltyv1.RegisterPointServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register tenant http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
