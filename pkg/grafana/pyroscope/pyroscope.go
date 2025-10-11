package pyroscope

import (
	"smallbiznis-controlplane/pkg/config"

	"github.com/grafana/pyroscope-go"
	"go.uber.org/fx"
)

var ProvidePyroscope = fx.Module("pyroscope",
	fx.Provide(
		NewClient,
		NewConfig,
	),
)

func NewConfig(cfg *config.Config) pyroscope.Config {
	return pyroscope.Config{
		ApplicationName: cfg.AppName,
		ServerAddress:   cfg.Pyroscope.Addr,
		ProfileTypes: []pyroscope.ProfileType{
			// these profile types are enabled by default:
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,

			// these profile types are optional:
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	}
}

func NewClient(cfg pyroscope.Config) (*pyroscope.Profiler, error) {
	return pyroscope.Start(cfg)
}
