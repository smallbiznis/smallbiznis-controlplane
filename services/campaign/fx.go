package campaign

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	campaignv1 "github.com/smallbiznis/go-genproto/smallbiznis/campaign/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var Module = fx.Module("campaign.module",
	fx.Provide(NewService),
)

var Gateway = fx.Module("campaign.server",
	fx.Invoke(
		registerServiceServer,
		registerServiceHandlerFromEndpoint,
	),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	campaignv1.RegisterCampaignServiceServer(server, service)
	grpc_health_v1.RegisterHealthServer(server, service)
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

			if err := campaignv1.RegisterCampaignServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				zap.L().Error("failed to register license http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
