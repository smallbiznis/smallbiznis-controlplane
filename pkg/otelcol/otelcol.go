package otelcol

import (
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func defaultTraceProviderOption() []trace.TracerProviderOption {
	return []trace.TracerProviderOption{
		trace.WithResource(resource.Default()),
	}
}

func ProvideTrace(exporter trace.SpanExporter, opts ...trace.TracerProviderOption) *trace.TracerProvider {
	if len(opts) == 0 {
		opts = defaultTraceProviderOption()
	}

	opts = append(opts, trace.WithBatcher(exporter))

	return trace.NewTracerProvider(opts...)
}

func defaultMetricProviderOption() []metric.Option {
	return []metric.Option{
		metric.WithResource(resource.Default()),
	}
}

func ProvideMetric(reader metric.Reader, opts ...metric.Option) *metric.MeterProvider {
	if len(opts) == 0 {
		opts = defaultMetricProviderOption()
	}

	opts = append(opts, metric.WithReader(reader))

	return metric.NewMeterProvider(opts...)
}
