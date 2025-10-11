package server

import (
	"context"
	"errors"
	"net"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"smallbiznis-controlplane/internal/config"
)

// ProvideGRPCServer constructs a gRPC server instance.
func ProvideGRPCServer() *grpc.Server {
	return grpc.NewServer()
}

// StartGRPCServer wires the gRPC server lifecycle to the fx application.
func StartGRPCServer(lc fx.Lifecycle, logger *zap.Logger, cfg *config.Config, srv *grpc.Server) {
	var listener net.Listener

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
			listener, err = net.Listen("tcp", cfg.GRPC.Addr)
			if err != nil {
				return err
			}

			go func() {
				logger.Info("Starting gRPC server...", zap.String("addr", cfg.GRPC.Addr))
				if err := srv.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
					logger.Fatal("gRPC server failed", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down gRPC server...", zap.String("addr", cfg.GRPC.Addr))

			done := make(chan struct{})
			go func() {
				srv.GracefulStop()
				close(done)
			}()

			select {
			case <-ctx.Done():
				srv.Stop()
			case <-done:
			}

			if listener != nil {
				_ = listener.Close()
			}

			return nil
		},
	})
}
