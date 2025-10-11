package logger

import (
	"os"

	"smallbiznis-controlplane/pkg/config"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("zap",
	fx.Provide(
		New,
	),
)

func defaultOption(cfg *config.Config) []zap.Field {
	return []zap.Field{
		zap.String("environment", cfg.AppEnv),
		zap.String("service_name", cfg.AppName),
		zap.String("service_namespace", cfg.AppNamespace),
	}
}

func New(cfg *config.Config) *zap.Logger {

	log := zap.Must(zap.NewDevelopment())
	if os.Getenv("ENV") == "production" {
		log = zap.Must(zap.NewProduction())
	}

	if cfg != nil {
		log = log.With(
			zap.String("app.env", cfg.AppEnv),
			zap.String("app.name", cfg.AppName),
		)
	}

	zap.ReplaceGlobals(log)

	return log
}
