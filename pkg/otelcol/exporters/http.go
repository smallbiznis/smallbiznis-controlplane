package exporters

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

func ProvideHttp(cfg *config.Config) (*otlptrace.Exporter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := otlptracehttp.NewClient(
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		// otlptracehttp.WithEndpoint(cfg.Otel),
	)

	return otlptrace.New(ctx, client)
}
