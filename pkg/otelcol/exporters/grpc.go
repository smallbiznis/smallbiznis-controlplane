package exporters

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

func ProvideGrpc(cfg *config.Config) (*otlptrace.Exporter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithCompressor("gzip"),
		// otlptracegrpc.WithEndpoint(),
	)

	return otlptrace.New(ctx, client)
}
