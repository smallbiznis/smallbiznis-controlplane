package ledger

import (
	"context"

	"github.com/gogo/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func (s *Service) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// You can optionally check database connectivity here:
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db not ready")
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *Service) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	// Optional: implement streaming health status (rarely used)
	return status.Error(codes.Unimplemented, "Watch method not implemented")
}
