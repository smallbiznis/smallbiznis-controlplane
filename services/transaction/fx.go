package transaction

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	transactionv1 "github.com/smallbiznis/go-genproto/smallbiznis/transaction/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var Module = fx.Module("transaction.service",
	fx.Provide(NewService),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("transaction.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	transactionv1.RegisterTransactionServiceServer(server, service)
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

			if err := transactionv1.RegisterTransactionServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register tenant http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
