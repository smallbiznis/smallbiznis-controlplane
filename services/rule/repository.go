package rule

import (
	"context"

	rulev1 "github.com/smallbiznis/smallbiznisapis/smallbiznis/rule/v1"
	"gorm.io/gorm"
)

// Repository defines data access behavior for rules.
type Repository interface {
	GetByID(ctx context.Context, tenantID, ruleID string) (*Rule, error)
	ListByTenant(ctx context.Context, tenantID string, includeDisabled bool, ruleIDs []string) ([]Rule, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository backed by gorm.DB.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetByID(ctx context.Context, tenantID, ruleID string) (*Rule, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	var rule Rule
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND rule_id = ?", tenantID, ruleID).
		First(&rule).Error
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

func (r *gormRepository) ListByTenant(ctx context.Context, tenantID string, includeDisabled bool, ruleIDs []string) ([]Rule, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	query := r.db.WithContext(ctx).Model(&Rule{}).Where("tenant_id = ?", tenantID)
	if len(ruleIDs) > 0 {
		query = query.Where("rule_id IN ?", ruleIDs)
	}
	if !includeDisabled {
		query = query.Where("status = ?", rulev1.RuleStatus_RULE_STATUS_ENABLED)
	}

	var rules []Rule
	if err := query.Order("rule_id").Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}
