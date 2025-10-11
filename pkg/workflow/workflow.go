package workflow

import (
	"context"
	"log/slog"
	"os"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ProvideClient = fx.Module("temporal",
	fx.Provide(NewClient),
	fx.Invoke(Close),
)

const (
	WorkflowEarnPoint   = "EarnPoint"
	WorkflowLedgerEntry = "LedgerEntry"
)

type TaskName string

var (
	POINT_TASK_QUEUE TaskName = "POINT_TASK_QUEUE"
)

func (t TaskName) String() string {
	switch t {
	case POINT_TASK_QUEUE:
		return string(t)
	default:
		return ""
	}
}

func NewClient(cfg *config.Config) client.Client {
	var c client.Client
	var err error

	clientOptions := client.Options{
		HostPort:  cfg.Temporal.Addr,
		Namespace: cfg.Temporal.Namespace,
		ConnectionOptions: client.ConnectionOptions{
			KeepAliveTime:    30 * time.Second,
			KeepAliveTimeout: 30 * time.Second,
			DialOptions: []grpc.DialOption{
				grpc.WithTransportCredentials(
					insecure.NewCredentials(),
				),
			},
		},
		Logger: log.With(
			slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				AddSource: true,
				Level:     slog.LevelDebug,
			}))),
	}

	for i := 1; i <= 3; i++ {
		c, err = client.Dial(clientOptions)
		if err == nil {
			break
		}
		zap.L().Warn("retrying Temporal client connection", zap.Int("attempt", i), zap.Error(err))
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		zap.L().Fatal("❌ failed to connect Temporal server after retries", zap.Error(err))
	}

	zap.L().Info("✅ Connected to Temporal server")
	return c
}

func Close(lc fx.Lifecycle, c client.Client) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			c.Close()
			return nil
		},
	})
}
