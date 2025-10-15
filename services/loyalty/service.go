package loyalty

import (
	"github.com/bwmarrin/snowflake"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Service struct {
	loyaltyv1.UnimplementedPointServiceServer

	db   *gorm.DB
	node *snowflake.Node
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
	}
}
