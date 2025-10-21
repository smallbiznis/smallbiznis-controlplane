package main

import (
	"context"
	"log"
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"smallbiznis-controlplane/pkg/client"
	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db"
	"smallbiznis-controlplane/pkg/httpapi"
	"smallbiznis-controlplane/pkg/logger"
	"smallbiznis-controlplane/pkg/sequence"
	"smallbiznis-controlplane/pkg/server"
	"smallbiznis-controlplane/services/loyalty"
)

func main() {
	opts := []fx.Option{
		config.Module,
		logger.Module,
		db.Module,
		// redis.Module,
		sequence.Module,
		fx.Provide(
			server.RegisterServerMux,
			provideTracerProvider,
			provideMeterProvider,
		),
		fx.Provide(
			provideSnowflakeNode,
			registerServerMux,
			registerAsynqServer,
			registerClient,
		),
		fx.Invoke(
			registerHandlers,
			runServerMux,
		),
		fx.Provide(
			client.NewRuleClient,
			client.NewLedgerClient,
		),
		httpapi.Module,
		loyalty.TaskModule,
		loyalty.Module,
		loyalty.Gateway,
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

func registerClient(lc fx.Lifecycle, cfg *config.Config) *asynq.Client {
	client := asynq.NewClient(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
	)

	if err := client.Ping(); err != nil {
		zap.L().Error("[Asynq] Failed to connect to Asynq", zap.Error(err))
		os.Exit(1)
	}

	zap.L().Info("[Asynq] Connected to Asynq")

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return client
}

func registerServerMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}

func registerAsynqServer(cfg *config.Config) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency:    10,
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
			Queues: map[string]int{
				"loyalty": 10,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				zap.L().Error("asynq task permanently failed", zap.String("task_type", task.Type()), zap.Error(err))
			}),
		},
	)
}

func runServerMux(lc fx.Lifecycle, server *asynq.Server, mux *asynq.ServeMux) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.Start(mux); err != nil {
					zap.L().Error("[Asynq] Failed to start Asynq server", zap.Error(err))
					os.Exit(1)
				}
			}()
			zap.L().Info("[Asynq] Asynq server started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			server.Stop()
			return nil
		},
	})
}

func registerHandlers(lc fx.Lifecycle, mux *asynq.ServeMux, svc *loyalty.Task) {
	mux.HandleFunc(loyalty.LoyaltyProcessEarning, svc.HandleProcessEarningTask)
}
