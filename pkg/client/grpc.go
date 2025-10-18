package client

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/config"

	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func newClient(lc fx.Lifecycle, addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		zap.L().Warn("failed to create new client", zap.Error(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			zap.L().Info("warming up grpc client", zap.String("target", addr))

			// 🔥 Force connect in background
			conn.Connect()

			go func() {
				state := conn.GetState()
				for state != connectivity.Ready && state != connectivity.Shutdown {
					if !conn.WaitForStateChange(ctx, state) {
						return
					}
					state = conn.GetState()
					zap.L().Debug("grpc state changed", zap.String("target", addr), zap.String("state", state.String()))
				}

				if state == connectivity.Ready {
					zap.L().Info("grpc connection ready", zap.String("target", addr))
				}
			}()

			go func() {
				healthClient := grpc_health_v1.NewHealthClient(conn)
				resp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
				if err != nil {
					zap.L().Warn("health check failed", zap.String("target", addr), zap.Error(err))
					return
				}
				zap.L().Info("health ok", zap.String("target", addr), zap.String("status", resp.Status.String()))
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return conn, nil
}

func NewTenantClient(lc fx.Lifecycle, cfg *config.Config) (tenantv1.TenantServiceClient, error) {
	conn, err := newClient(lc, "tenant:4317")
	if err != nil {
		zap.L().Error("failed to connected ledger", zap.Error(err), zap.String("url", "tenant:4317"))
		return nil, err
	}
	return tenantv1.NewTenantServiceClient(conn), nil
}

func NewLedgerClient(lc fx.Lifecycle, cfg *config.Config) (ledgerv1.LedgerServiceClient, error) {
	conn, err := newClient(lc, "ledger:4317")
	if err != nil {
		zap.L().Error("failed to connected ledger", zap.Error(err), zap.String("url", "ledger:4317"))
		return nil, err
	}
	return ledgerv1.NewLedgerServiceClient(conn), nil
}

func NewLoyaltyClient(lc fx.Lifecycle, cfg *config.Config) (loyaltyv1.PointServiceClient, error) {
	conn, err := newClient(lc, "loyalty:4317")
	if err != nil {
		zap.L().Error("failed to connected loyalty", zap.Error(err), zap.String("url", "loyalty:4317"))
		return nil, err
	}
	return loyaltyv1.NewPointServiceClient(conn), nil
}
