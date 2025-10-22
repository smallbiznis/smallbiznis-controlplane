package main

import (
	"log"
	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db"
	"smallbiznis-controlplane/pkg/logger"

	"go.uber.org/fx"
)

func main() {
	opts := []fx.Option{
		config.Module,
		logger.Module,
		db.Module,
	}

	if err := fx.ValidateApp(opts...); err != nil {
		log.Fatalf("fx validation failed: %v", err)
	}

	app := fx.New(opts...)

	app.Run()
}
