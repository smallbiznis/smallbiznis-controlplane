package domain

import (
	"context"
	"fmt"
	"smallbiznis-controlplane/pkg/dns"
	"smallbiznis-controlplane/pkg/repository"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/status"
	domainv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/domain/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db   *gorm.DB
	node *snowflake.Node
	repo repository.Repository[Domain]
	domainv1.UnimplementedDomainServiceServer
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:   p.DB,
		node: p.Node,
		repo: repository.ProvideStore[Domain](p.DB),
	}
}

func (s *Service) ListDomains(ctx context.Context, req *domainv1.ListDomainsRequest) (*domainv1.ListDomainsResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	domains, err := s.repo.Find(ctx, &Domain{
		TenantID: req.GetTenantId(),
	})
	if err != nil {
		zapLog.Error("failed to list domains", zap.Error(err), zap.String("tenant_id", req.GetTenantId()))
		return nil, status.Errorf(codes.Internal, "failed to list domains: %v", err)
	}

	out := make([]*domainv1.Domain, 0, len(domains))
	for _, d := range domains {
		out = append(out, d.ToProto())
	}

	return &domainv1.ListDomainsResponse{
		Domains: out,
	}, nil
}

func (s *Service) GetDomain(ctx context.Context, req *domainv1.GetDomainRequest) (*domainv1.Domain, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	if strings.TrimSpace(req.GetDomainId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "domain_id is required")
	}

	domain, err := s.repo.FindOne(ctx, &Domain{
		ID: req.GetDomainId(),
	})
	if err != nil {
		zapLog.Error("failed to get domain", zap.Error(err), zap.String("domain_id", req.GetDomainId()))
		return nil, status.Errorf(codes.Internal, "failed to get domain: %v", err)
	}

	if domain == nil {
		return nil, status.Error(codes.NotFound, "domain not found")
	}

	return domain.ToProto(), nil
}

func (s *Service) VerifyDomain(ctx context.Context, req *domainv1.VerifyDomainRequest) (*domainv1.VerifyDomainResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceFields := []zap.Field{
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	}
	zap.L().With(traceFields...).Info("VerifyDomain", zap.Any("req", req))

	if strings.TrimSpace(req.GetHostname()) == "" {
		return nil, status.Error(codes.InvalidArgument, "hostname is required")
	}

	var domain Domain
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND hostname = ?", req.GetTenantId(), req.GetHostname()).
		First(&domain).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, status.Error(codes.NotFound, "domain not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to query domain: %v", err)
	}

	// Skip if already verified
	if domain.Verified {
		return &domainv1.VerifyDomainResponse{
			Success: true,
			Message: fmt.Sprintf("domain %s already verified", domain.Hostname),
		}, nil
	}

	// --- Verification logic ---
	switch domain.VerificationMethod {
	case DNS:
		if err := dns.VerifyDNSRecord(domain.Hostname, *domain.VerificationCode); err != nil {
			zap.L().Warn("DNS verification failed", zap.String("domain", domain.Hostname), zap.Error(err))
			return &domainv1.VerifyDomainResponse{
				Success: false,
				Message: fmt.Sprintf("dns verification failed: %v", err),
			}, nil
		}
	default:
		return nil, status.Errorf(codes.Unimplemented, "verification method %s not supported", domain.VerificationMethod)
	}

	// Update domain as verified
	now := time.Now()
	if err := s.db.Model(&domain).Updates(map[string]interface{}{
		"verified":           true,
		"verified_at":        now,
		"certificate_status": Active,
		"updated_at":         now,
	}).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update domain: %v", err)
	}

	zap.L().Info("Domain verified successfully",
		zap.String("tenant_id", domain.TenantID),
		zap.String("hostname", domain.Hostname),
	)

	return &domainv1.VerifyDomainResponse{
		Success: true,
		Message: fmt.Sprintf("domain %s verified successfully", domain.Hostname),
	}, nil
}
