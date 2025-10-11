package client

import (
	"context"

	"smallbiznis-controlplane/pkg/config"

	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGRPCConn(lc fx.Lifecycle, addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return conn, nil
}

func NewLedgerClient(lc fx.Lifecycle, cfg *config.Config) (ledgerv1.LedgerServiceClient, error) {
	conn, err := NewGRPCConn(lc, cfg.LedgerURL)
	if err != nil {
		zap.L().Error("failed to connected ledger", zap.Error(err), zap.String("url", cfg.LedgerURL))
		return nil, err
	}
	return ledgerv1.NewLedgerServiceClient(conn), nil
}
