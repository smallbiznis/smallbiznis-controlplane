package domain

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/services/internal/endpoint"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	domainv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/domain/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var Module = fx.Module("domain.module",
	fx.Provide(NewService),
)

var ServerModule = fx.Module("domain.server",
	Module,
	fx.Invoke(
		registerServiceServer,
		registerServiceHandlerFromEndpoint,
	),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	domainv1.RegisterDomainServiceServer(server, service)
}

type registerServiceHandlerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Mux       *runtime.ServeMux
	Config    *config.Config
}

func registerServiceHandlerFromEndpoint(p registerServiceHandlerParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			target := endpoint.Normalize(p.Config.Grpc.Addr)
			opts := []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			}

			if err := domainv1.RegisterDomainServiceHandlerFromEndpoint(ctx, p.Mux, target, opts); err != nil {
				zap.L().Error("failed to register domain http handler", zap.Error(err))
				return err
			}

			zap.L().Info("domain http handler registered", zap.String("endpoint", target))
			return nil
		},
	})
}
