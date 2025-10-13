package tenant

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var Module = fx.Module("tenant.service",
	fx.Provide(NewService),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("tenant.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	tenantv1.RegisterTenantServiceServer(server, service)
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

			if err := tenantv1.RegisterTenantServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register tenant http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
