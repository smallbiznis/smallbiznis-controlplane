package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// ZapGormLogger implements gorm.io/gorm/logger.Interface
type ZapGormLogger struct {
	Zap           *zap.Logger
	SlowThreshold time.Duration
	LogLevel      logger.LogLevel
	ShowSQL       bool
}

// NewZapGormLogger returns a new ZapGormLogger with sane defaults
func NewZapGormLogger(z *zap.Logger, logLevel logger.LogLevel, showSQL bool) *ZapGormLogger {
	return &ZapGormLogger{
		Zap:           z,
		LogLevel:      logLevel,
		ShowSQL:       showSQL,
		SlowThreshold: 200 * time.Millisecond, // can override
	}
}

func (l *ZapGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *ZapGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.Zap.Info(fmt.Sprintf(msg, data...))
	}
}

func (l *ZapGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.Zap.Warn(fmt.Sprintf(msg, data...))
	}
}

func (l *ZapGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.Zap.Error(fmt.Sprintf(msg, data...))
	}
}

func (l *ZapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && !errors.Is(err, logger.ErrRecordNotFound):
		l.Zap.Error("gorm.query",
			zap.String("file", utils.FileWithLineNum()),
			zap.Error(err),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
		)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0:
		l.Zap.Warn("gorm.slow_query",
			zap.String("file", utils.FileWithLineNum()),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
			zap.Duration("threshold", l.SlowThreshold),
		)
	case l.LogLevel == logger.Info && l.ShowSQL:
		l.Zap.Info("gorm.query",
			zap.String("file", utils.FileWithLineNum()),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
		)
	}
}
