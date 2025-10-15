package tenant

import (
	"context"
	"fmt"
	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/db/pagination"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/pkg/security"
	"smallbiznis-controlplane/pkg/sequence"
	"smallbiznis-controlplane/pkg/task"
	"smallbiznis-controlplane/services/apikey"
	"smallbiznis-controlplane/services/domain"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gosimple/slug"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	asynq  task.Enqueuer
	node   *snowflake.Node
	seq    sequence.Generator
	config *config.Config
	repo   repository.Repository[Tenant]
	tenantv1.UnimplementedTenantServiceServer
}

type ServiceParams struct {
	fx.In
	DB     *gorm.DB
	Asynq  task.Enqueuer
	Node   *snowflake.Node
	Seq    sequence.Generator
	Config *config.Config
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:     p.DB,
		asynq:  p.Asynq,
		node:   p.Node,
		seq:    p.Seq,
		config: p.Config,
		repo:   repository.ProvideStore[Tenant](p.DB),
	}
}

func (s *Service) ListTenants(ctx context.Context, req *tenantv1.ListTenantsRequest) (*tenantv1.ListTenantsResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	opts := []option.QueryOption{
		option.ApplyPagination(pagination.Pagination{
			Limit: int(req.Limit),
		}),
	}

	tenants, err := s.repo.Find(ctx, &Tenant{}, opts...)
	if err != nil {
		zapLog.Error("failed to list tenants", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list tenants")
	}

	out := make([]*tenantv1.Tenant, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, t.ToProto())
	}

	return &tenantv1.ListTenantsResponse{
		Tenants: out,
	}, nil
}

func (s *Service) CreateTenant(ctx context.Context, req *tenantv1.CreateTenantRequest) (*tenantv1.Tenant, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	if s.config.Platform.Domain == "" {
		zapLog.Error("failed to create tenant, platform domain not configured")
		return nil, status.Error(codes.Internal, "failed to create tenant, platform domain not configured")
	}

	slugName := req.GetSlug()
	if slugName == "" {
		slugName = slug.Make(req.GetName())
	}

	exist, err := s.repo.FindOne(ctx, &Tenant{
		Slug: slugName,
	})
	if err != nil {
		zapLog.Error("failed query get tenant by slug", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to check existing tenant")
	}

	if exist != nil {
		zapLog.Warn("tenant already exists", zap.String("slug", slugName))
		return nil, status.Error(codes.AlreadyExists, "tenant already exists")
	}

	tenantID := s.node.Generate().String()
	tenantCode, err := s.seq.NextTenantCode(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed create tenant")
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {

		tenant := &Tenant{
			ID:          tenantID,
			Type:        TenantType(req.GetType()),
			Name:        req.GetName(),
			Slug:        slugName,
			Code:        tenantCode,
			CountryCode: req.GetCountryCode(),
			Timezone:    req.GetTimezone(),
			Status:      Active,
		}

		if err := tx.Create(tenant).Error; err != nil {
			return fmt.Errorf("failed to create tenant: %w", err)
		}

		defaultHostName := fmt.Sprintf("%s.%s", slugName, s.config.RootDomain)
		domainID := s.node.Generate().String()
		domain := &domain.Domain{
			ID:                 domainID,
			TenantID:           tenantID,
			Type:               domain.System,
			Hostname:           defaultHostName,
			VerificationMethod: domain.DNS,
			VerificationCode:   nil,
			CertificateStatus:  domain.Active,
			IsPrimary:          true,
			Verified:           true,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		if err := tx.Create(domain).Error; err != nil {
			return fmt.Errorf("failed to create domain: %w", err)
		}

		secret, err := security.GenerateBase64Secret(32)
		if err != nil {
			return fmt.Errorf("failed to generate api key secret: %w", err)
		}

		hash, err := security.HashArgon2(secret)
		if err != nil {
			return fmt.Errorf("failed to hash api key secret: %w", err)
		}

		apiKeyID := s.node.Generate().String()
		apiKey := &apikey.APIKey{
			ID:         apiKeyID,
			TenantID:   tenantID,
			KeyID:      fmt.Sprintf("sbsk_live_%s", apiKeyID),
			KeyType:    apikey.APIKeyTypeServer,
			SecretHash: hash,
			Scopes:     []string{"*"},
			Status:     string(apikey.APIKeyStatusActive),
			CreatedAt:  time.Now(),
		}

		if err := tx.Create(apiKey).Error; err != nil {
			return fmt.Errorf("failed to create api key: %w", err)
		}

		// payload := map[string]interface{}{
		// 	"tenant_id":   tenantID,
		// 	"tenant_slug": slugName,
		// 	"domain":      defaultHostName,
		// }
		// payloadBytes, _ := json.Marshal(payload)

		// tasks := []*asynq.Task{
		// 	asynq.NewTask("tenant:provisioning:ledger", payloadBytes),
		// 	asynq.NewTask("tenant:provisioning:inventory", payloadBytes),
		// 	asynq.NewTask("tenant:provisioning:rules", payloadBytes),
		// 	asynq.NewTask("tenant:provisioning:loyalty", payloadBytes),
		// 	asynq.NewTask("tenant:provisioning:voucher", payloadBytes),
		// }

		// for _, task := range tasks {
		// 	if _, err := s.asynq.Enqueue(task, asynq.Queue("critical")); err != nil {
		// 		zapLog.Error("failed enqueue provisioning task", zap.String("task_type", task.Type()), zap.Error(err))
		// 		return fmt.Errorf("failed to enqueue provisioning task: %w", err)
		// 	}
		// }

		return nil
	}); err != nil {
		zapLog.Error("failed to create tenant transaction", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.GetTenant(ctx, &tenantv1.GetTenantRequest{
		TenantId: tenantID,
	})
}

func (s *Service) GetTenant(ctx context.Context, req *tenantv1.GetTenantRequest) (*tenantv1.Tenant, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	tenant, err := s.repo.FindOne(ctx, &Tenant{
		ID: req.TenantId,
	})
	if err != nil {
		zapLog.Error("failed query get tenant by id", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get tenant")
	}

	if tenant == nil {
		zapLog.Warn("failed get tenant, tenant not found", zap.String("tenant_id", req.TenantId))
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	return tenant.ToProto(), nil
}
