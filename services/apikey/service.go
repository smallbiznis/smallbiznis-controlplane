package apikey

import (
	"smallbiznis-controlplane/pkg/repository"

	"github.com/bwmarrin/snowflake"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	node *snowflake.Node
	repo repository.Repository[APIKey]
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
		repo: repository.ProvideStore[APIKey](p.DB),
	}
}
