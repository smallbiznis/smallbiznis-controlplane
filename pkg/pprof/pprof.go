package pprof

import (
	"context"
	"net/http"
	_ "net/http/pprof"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("pprof", fx.Invoke(startPprofServer))

func startPprofServer(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				zap.L().Info("Starting pprof server", zap.String("port", "6060"))
				if err := http.ListenAndServe(":6060", nil); err != nil {
					zap.L().Error("Error starting pprof server", zap.Error(err))
				}
			}()
			return nil
		},
	})
}
