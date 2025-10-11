package main

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/httpapi"
	"smallbiznis-controlplane/pkg/logger"
	"smallbiznis-controlplane/pkg/server"
	"smallbiznis-controlplane/services/domain"
	"smallbiznis-controlplane/services/license"
	"smallbiznis-controlplane/services/tenant"
)

func main() {
	app := fx.New(
		config.Module,
		logger.Module,
		httpapi.Module,
		fx.Provide(
			provideTracerProvider,
			provideMeterProvider,
		),
		server.ProvideGRPCServer,
		server.ProvideHTTPServer,
		tenant.ServerModule,
		domain.ServerModule,
		license.ServerModule,
	)

	app.Run()
}

func provideTracerProvider() trace.TracerProvider {
	return otel.GetTracerProvider()
}

func provideMeterProvider() metric.MeterProvider {
	return otel.GetMeterProvider()
}
