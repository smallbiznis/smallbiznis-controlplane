package main

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db"
	"smallbiznis-controlplane/pkg/httpapi"
	"smallbiznis-controlplane/pkg/logger"
	"smallbiznis-controlplane/pkg/server"
	"smallbiznis-controlplane/services/tenant"
)

func main() {
	app := fx.New(
		config.Module,
		logger.Module,
		db.Module,
		tenant.Module,
		httpapi.Module,
		fx.Provide(
			provideTracerProvider,
			provideMeterProvider,
			provideSnowflakeNode,
		),
		server.ProvideGRPCServer,
		server.ProvideHTTPServer,
		fx.Invoke(
			logHTTPServerStartup,
			registerTenantServiceServer,
			registerTenantGateway,
		),
	)

	app.Run()
}

func provideTracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func provideMeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}

func provideSnowflakeNode() (*snowflake.Node, error) {
	return snowflake.NewNode(1)
}

func logHTTPServerStartup(lc fx.Lifecycle, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			logger.Info("Starting HTTP server...")
			return nil
		},
	})
}

func registerTenantServiceServer(server *grpc.Server, service *tenant.Service) {
	tenantv1.RegisterTenantServiceServer(server, service)
}

type gatewayParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Mux       *runtime.ServeMux
	Config    *config.Config
}

func registerTenantGateway(p gatewayParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			target := normalizeEndpoint(p.Config.Grpc.Addr)
			opts := []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			}

			if err := tenantv1.RegisterTenantServiceHandlerFromEndpoint(ctx, p.Mux, target, opts); err != nil {
				zap.L().Error("failed to register tenant http handler", zap.Error(err))
				return err
			}

			zap.L().Info("tenant http handler registered", zap.String("endpoint", target))
			return nil
		},
	})
}

func normalizeEndpoint(addr string) string {
	if addr == "" {
		return ":0"
	}

	if addr[0] == ':' {
		return addr
	}

	return ":" + addr
}
