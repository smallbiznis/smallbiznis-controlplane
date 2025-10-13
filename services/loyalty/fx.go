package loyalty

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

var Module = fx.Module("apikey.service",
	// fx.Provide(NewService),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("apikey.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	// ledgerv1.RegisterLedgerServiceServer(server, service)
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

			// if err := ledgerv1.RegisterLedgerServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
			// 	zap.L().Error("failed to register tenant http handler", zap.Error(err))
			// 	return err
			// }

			return nil
		},
	})
}
