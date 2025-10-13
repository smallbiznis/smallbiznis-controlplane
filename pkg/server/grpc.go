package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"smallbiznis-controlplane/pkg/config"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

var ProvideGRPCServer = fx.Module("grpc.server",
	fx.Provide(
		NewListener,
		WithOption,
		NewGRPCServer,
	),
	fx.Invoke(StartGrpcServer),
)

func NewListener(cfg *config.Config) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf(":%s", cfg.Grpc.Addr))
}

func WithOption(tp trace.TracerProvider, mp metric.MeterProvider, opts ...grpc.ServerOption) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			validator.UnaryServerInterceptor(validator.WithFailFast()),
		),
		grpc.ChainStreamInterceptor(
			validator.StreamServerInterceptor(validator.WithFailFast()),
		),
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(
				otelgrpc.WithTracerProvider(tp),
				otelgrpc.WithMeterProvider(mp),
			),
		),
	}
}

func WithStatsHandler(tp trace.TracerProvider, mp metric.MeterProvider) grpc.ServerOption {
	return grpc.StatsHandler(
		otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(tp),
			otelgrpc.WithMeterProvider(mp),
		),
	)
}

// LoadCertificate
func LoadCertificate(certPath, keyPath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// WithTLS
func WithTLS(tls *tls.Certificate) grpc.ServerOption {
	return grpc.Creds(
		credentials.NewServerTLSFromCert(tls),
	)
}

func NewGRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(opts...)
}

type grpcParams struct {
	fx.In

	Lifecycle      fx.Lifecycle
	Listener       net.Listener
	TraceProvider  trace.TracerProvider
	MetricProvider metric.MeterProvider
}

func StartGrpcServer(lc fx.Lifecycle, srv *grpc.Server, lis net.Listener) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				zap.L().Info("Starting gRPC server", zap.String("addr", lis.Addr().String()))
				reflection.Register(srv)
				if err := srv.Serve(lis); err != nil {
					zap.L().Fatal("gRPC server exited", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			zap.L().Info("Stopping gRPC server")
			srv.GracefulStop()
			return nil
		},
	})
}
