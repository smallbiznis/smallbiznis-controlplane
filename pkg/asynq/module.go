package asynq

import (
	"context"

	"smallbiznis-controlplane/pkg/config"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Client = fx.Module("asynq:client",
	fx.Provide(registerClient),
)

func registerClient(lc fx.Lifecycle, redis *redis.Client) *asynq.Client {
	client := asynq.NewClientFromRedisClient(redis)

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

func registerAsynqServer(lc fx.Lifecycle, cfg *config.Config, client *asynq.Client, mux *asynq.ServeMux) {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: cfg.Redis.Addr,
			DB:   cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency:    10,
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				zap.L().Error("asynq task permanently failed", zap.String("task_type", task.Type()), zap.Error(err))
			}),
		},
	)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go server.Run(mux)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			server.Stop()
			return nil
		},
	})
}
