package main

import (
	"log"

	"github.com/bwmarrin/snowflake"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db"
	"smallbiznis-controlplane/pkg/httpapi"
	"smallbiznis-controlplane/pkg/logger"
	"smallbiznis-controlplane/pkg/redis"
	"smallbiznis-controlplane/pkg/sequence"
	"smallbiznis-controlplane/pkg/server"
	"smallbiznis-controlplane/pkg/task"
	"smallbiznis-controlplane/services/ledger"
)

func main() {
	opts := []fx.Option{
		config.Module,
		logger.Module,
		db.Module,
		redis.Module,
		task.Client,
		sequence.Module,
		fx.Provide(
			server.RegisterServerMux,
			provideTracerProvider,
			provideMeterProvider,
			provideSnowflakeNode,
		),
		httpapi.Module,
		ledger.Module,
		ledger.Gateway,
		server.ProvideGRPCServer,
		server.ProvideHTTPServer,
		fxLogger,
	}

	if err := fx.ValidateApp(opts...); err != nil {
		log.Fatalf("fx validation failed: %v", err)
	}

	app := fx.New(opts...)

	app.Run()
}

var fxLogger = fx.WithLogger(func(cfg *config.Config, logger *zap.Logger) fxevent.Logger {
	return fxevent.NopLogger
})

func provideTracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func provideMeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}

func provideSnowflakeNode() (*snowflake.Node, error) {
	return snowflake.NewNode(1)
}
