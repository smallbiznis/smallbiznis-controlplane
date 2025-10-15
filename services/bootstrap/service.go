package bootstrap

import (
	"context"
	"fmt"
	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/pkg/security"
	"smallbiznis-controlplane/pkg/sequence"
	"smallbiznis-controlplane/services/apikey"
	"smallbiznis-controlplane/services/domain"
	"smallbiznis-controlplane/services/tenant"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/status"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	node   *snowflake.Node
	seq    sequence.Generator
	config *config.Config
	repo   repository.Repository[tenant.Tenant]
}

type ServiceParams struct {
	fx.In
	DB     *gorm.DB
	Node   *snowflake.Node
	Seq    sequence.Generator
	Config *config.Config
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:     p.DB,
		node:   p.Node,
		seq:    p.Seq,
		config: p.Config,
		repo:   repository.ProvideStore[tenant.Tenant](p.DB),
	}
}

func (s *Service) Migrate() error {
	ctx := context.Background()

	platform := s.config.Platform
	if platform.ID == "" || platform.Name == "" || platform.CountryCode == "" || platform.Timezone == "" || platform.Domain == "" {
		zap.L().Error("[bootstrap] Platform configuration is incomplete. Skipping default tenant creation.")
		return status.Errorf(codes.FailedPrecondition, "platform configuration is incomplete")
	}

	exist, err := s.repo.FindOne(ctx, &tenant.Tenant{
		IsDefault: true,
	})
	if err != nil {
		zap.L().Error("[bootstrap] Error checking tenant", zap.Error(err))
		return status.Errorf(codes.Internal, "failed to check existing tenant: %v", err)
	}

	if exist != nil {
		zap.L().Info("[bootstrap] Default tenant already exists", zap.String("tenant_name", platform.Name))
		return status.Errorf(codes.AlreadyExists, "default tenant already exists")
	}

	tenantID := s.node.Generate().String()
	tenantCode, err := s.seq.NextTenantCode(ctx)
	if err != nil {
		return status.Error(codes.Internal, "failed create default tenant")
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {

		tenant := &tenant.Tenant{
			ID:          tenantID,
			Type:        tenant.Platform,
			Name:        platform.Name,
			Code:        tenantCode,
			Slug:        platform.ID,
			CountryCode: platform.CountryCode,
			Timezone:    platform.Timezone,
			IsDefault:   true,
			Status:      tenant.Active,
		}

		if err := tx.Create(tenant).Error; err != nil {
			return fmt.Errorf("failed to create tenant: %w", err)
		}

		domainID := s.node.Generate().String()
		domain := &domain.Domain{
			ID:                 domainID,
			TenantID:           tenantID,
			Type:               domain.System,
			Hostname:           platform.Domain,
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

		return nil
	}); err != nil {
		zap.L().Error("failed to create tenant transaction", zap.Error(err))
		return status.Error(codes.Internal, err.Error())
	}

	zap.L().Info("[bootstrap] Default tenant created", zap.String("tenant_name", platform.Name))

	return nil
}
