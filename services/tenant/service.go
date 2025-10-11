package tenant

import (
	"smallbiznis-controlplane/pkg/repository"

	"github.com/bwmarrin/snowflake"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	node *snowflake.Node
	repo repository.Repository[Tenant]
	tenantv1.UnimplementedTenantServiceServer
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:   p.DB,
		repo: repository.ProvideStore[Tenant](p.DB),
		node: p.Node,
	}
}
