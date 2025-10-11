package logger

import "go.uber.org/zap"

// Provide returns a production-ready zap logger configured for the application.
func Provide() (*zap.Logger, error) {
	return zap.NewProduction()
}
