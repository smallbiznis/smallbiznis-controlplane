package profiling

import (
	"context"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grafana/pyroscope-go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("profiling", fx.Invoke(ProvideProfiling))

func ProvideProfiling(lc fx.Lifecycle, c *config.Config) *pyroscope.Profiler {
	zap.L().Info("starting pyroscope", zap.String("app_name", c.AppName), zap.String("pyroscope_addr", c.Pyroscope.Addr))
	profiler, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: c.AppName,
		ServerAddress:   c.Pyroscope.Addr,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
		},
		Tags: map[string]string{
			"service_name": c.AppName,
			"env":          c.AppEnv,
		},
	})
	if err != nil {
		zap.L().Fatal("failed to start pyroscope", zap.Error(err))
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// Pyroscope-go doesn't expose Stop(), but if future versions support it,
			// handle graceful shutdown here.
			zap.L().Info("Shutting down Pyroscope (noop for now)")
			return profiler.Stop()
		},
	})

	return profiler
}
