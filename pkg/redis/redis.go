package redis

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("redis",
	fx.Provide(New),
)

func New(lc fx.Lifecycle, c *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:        c.Redis.Addr,
		Password:    c.Redis.Password, // no password set
		DB:          c.Redis.DB,       // use default DB
		PoolSize:    c.Redis.PoolSize,
		PoolTimeout: c.Redis.PoolTimeout,
	})

	for i := 0; i < 5; i++ {
		_, err := rdb.Ping(context.Background()).Result()
		if err != nil {
			break
		}

		zap.L().Warn("Redis not ready, retrying in 3 seconds...", zap.Int("retry", i+1), zap.Error(err))
		time.Sleep(3 * time.Second)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return rdb.Close()
		},
	})

	return rdb
}
