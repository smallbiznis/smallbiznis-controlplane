package logger

import (
	"smallbiznis-controlplane/pkg/config"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Module = fx.Module("zap",
	fx.Provide(
		New,
	),
)

type ConfigParams struct {
	fx.In
	Cfg *config.Config
}

func New(p ConfigParams) *zap.Logger {

	log := zap.Must(zap.NewDevelopment())
	if p.Cfg.AppEnv == "production" {

		config := zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.StacktraceKey = "stacktrace"
		config.EncoderConfig.LevelKey = "severity"
		config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		config.Encoding = "json"
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}

		var err error
		log, err = config.Build()
		if err != nil {
			panic(err)
		}

		defer log.Sync()
	}

	if p.Cfg != nil {
		log = log.With(
			zap.String("env", p.Cfg.AppEnv),
			zap.String("service_name", p.Cfg.AppName),
		)
	}

	zap.ReplaceGlobals(log)

	return log
}
