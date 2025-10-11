package main

import (
	"go.uber.org/fx"

	"smallbiznis-controlplane/internal/config"
	"smallbiznis-controlplane/internal/httpapi"
	"smallbiznis-controlplane/internal/logger"
	"smallbiznis-controlplane/internal/server"
)

func main() {
	app := fx.New(
		fx.Provide(
			config.Provide,
			logger.Provide,
			httpapi.ProvideMux,
			server.ProvideHTTPServer,
			server.ProvideGRPCServer,
		),
		fx.Invoke(
			server.Run,
			server.StartGRPCServer,
		),
	)

	app.Run()
}
