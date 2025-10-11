package tenant

import (
	"context"
	"fmt"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/server"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RegisterServiceServer(s *grpc.Server, srv *Service) {
	tenantv1.RegisterTenantServiceServer(s, srv)
}

func RegisterServiceHandlerFromEndpoint(lc fx.Lifecycle, mux *runtime.ServeMux, cfg *config.Config) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {

			opts := []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			}

			if err := tenantv1.RegisterTenantServiceHandlerFromEndpoint(ctx, mux, fmt.Sprintf(":%s", cfg.Server.Addr), opts); err != nil {
				zap.L().Error("failed to RegisterServiceHandlerFromEndpoint", zap.Error(err))
			}

			return nil
		},
	})
}

var Module = fx.Module("tenant.module",
	fx.Provide(
		NewService,
	),
)

var Server = fx.Module("tenant.server",
	Module,
	fx.Invoke(
		RegisterServiceServer,
		RegisterServiceHandlerFromEndpoint,
	),
	server.ProvideGRPCServer,
)
