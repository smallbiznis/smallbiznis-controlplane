package rule

import (
	"time"

	rulev1 "github.com/smallbiznis/smallbiznisapis/smallbiznis/rule/v1"
	"gorm.io/datatypes"
)

// Rule represents a rule definition stored in the database.
type Rule struct {
	RuleID      string                `gorm:"column:rule_id;primaryKey"`
	TenantID    string                `gorm:"column:tenant_id;index"`
	Name        string                `gorm:"column:name"`
	Description string                `gorm:"column:description"`
	Expression  string                `gorm:"column:expression"`
	ActionType  rulev1.RuleActionType `gorm:"column:action_type"`
	ActionValue datatypes.JSONMap     `gorm:"column:action_value"`
	Status      rulev1.RuleStatus     `gorm:"column:status"`
	CreatedAt   time.Time             `gorm:"column:created_at"`
	UpdatedAt   time.Time             `gorm:"column:updated_at"`
}

// TableName sets the table name for the Rule model.
func (Rule) TableName() string { return "rules" }
