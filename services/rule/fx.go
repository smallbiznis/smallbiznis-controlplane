package rule

import (
	"context"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	rulev1 "github.com/smallbiznis/smallbiznisapis/smallbiznis/rule/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var Module = fx.Module("rule.service",
	fx.Provide(
		NewRepository,
		NewEvaluator,
		NewService,
	),
	fx.Invoke(registerServiceServer),
)

var Gateway = fx.Module("rule.gateway",
	fx.Invoke(registerServiceHandlerServer),
)

func registerServiceServer(server *grpc.Server, service *Service) {
	rulev1.RegisterRuleServiceServer(server, service)
}

type registerServiceHandlerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Mux       *runtime.ServeMux
	Service   *Service
	Logger    *zap.Logger
}

func registerServiceHandlerServer(p registerServiceHandlerParams) {
	if p.Logger == nil {
		p.Logger = zap.NewNop()
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := rulev1.RegisterRuleServiceHandlerServer(ctx, p.Mux, p.Service); err != nil {
				p.Logger.Error("failed to register rule http handler", zap.Error(err))
				return err
			}

			return nil
		},
	})
}
