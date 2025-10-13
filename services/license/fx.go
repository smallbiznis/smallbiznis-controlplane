package license

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	licensev1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/license/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var Module = fx.Module("license.module",
	fx.Provide(NewService),
)

var ServerModule = fx.Module("license.server",
	Module,
	fx.Invoke(
		registerServiceServer,
		registerServiceHandlerFromEndpoint,
	),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	licensev1.RegisterLicenseServiceServer(server, service)
}

type registerServiceHandlerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Mux       *runtime.ServeMux
	Config    *config.Config
	Service   *Service
}

func registerServiceHandlerFromEndpoint(p registerServiceHandlerParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := licensev1.RegisterLicenseServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register license http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
