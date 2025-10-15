package task

import (
	"context"
	"os"

	"smallbiznis-controlplane/pkg/config"

	"github.com/hibiken/asynq"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Client = fx.Module("asynq:client",
	fx.Provide(registerClient, NewEnqueuer),
)

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

var Server = fx.Module("asynq:server",
	fx.Provide(registerServerMux),
	fx.Invoke(registerAsynqServer),
)

func registerServerMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}

func registerAsynqServer(lc fx.Lifecycle, cfg *config.Config, mux *asynq.ServeMux) {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency:    10,
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
			Queues: map[string]int{
				"critical": 10,
				"default":  5,
				"low":      3,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				zap.L().Error("asynq task permanently failed", zap.String("task_type", task.Type()), zap.Error(err))
			}),
		},
	)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.Start(mux); err != nil {
					zap.L().Error("[Asynq] Failed to start Asynq server", zap.Error(err))
					os.Exit(1)
				}
			}()
			zap.L().Info("[Asynq] Asynq server started", zap.String("addr", cfg.Redis.Addr))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			server.Stop()
			return nil
		},
	})
}
