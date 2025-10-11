package server

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"smallbiznis-controlplane/internal/config"
)

// ProvideHTTPServer constructs an *http.Server configured from the application config.
func ProvideHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
}

// Run wires the HTTP server lifecycle to the fx application.
func Run(lc fx.Lifecycle, logger *zap.Logger, cfg *config.Config, srv *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info("Starting HTTP server...", zap.String("addr", cfg.Server.Addr), zap.Bool("tls_enabled", cfg.TLS.Enable))
				var err error
				if cfg.TLS.Enable {
					err = srv.ListenAndServeTLS(cfg.TLS.CertPath, cfg.TLS.KeyPath)
				} else {
					err = srv.ListenAndServe()
				}
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Fatal("HTTP server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down HTTP server...", zap.String("addr", cfg.Server.Addr))
			return srv.Shutdown(ctx)
		},
	})
}
