package rule

import (
	"context"

	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"gorm.io/gorm"
)

// ListParams describes filters applied when listing rules from the repository.
type ListParams struct {
	AfterPriority   *int32
	AfterRuleID     string
	Limit           int
	IncludeInactive bool
	Triggers        []rulev1.RuleTriggerType
}

// Repository describes database operations available for rules.
type Repository interface {
	Create(ctx context.Context, rule *Rule) error
	GetByID(ctx context.Context, tenantID, ruleID string) (*Rule, error)
	List(ctx context.Context, tenantID string, params ListParams) ([]Rule, error)
	Update(ctx context.Context, rule *Rule) error
	Delete(ctx context.Context, tenantID, ruleID string) error
	ListActiveByTrigger(ctx context.Context, tenantID string, trigger rulev1.RuleTriggerType) ([]Rule, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a gorm backed Repository implementation.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, rule *Rule) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).Create(rule).Error
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

func (r *gormRepository) List(ctx context.Context, tenantID string, params ListParams) ([]Rule, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	query := r.db.WithContext(ctx).Model(&Rule{}).
		Where("tenant_id = ?", tenantID)

	if len(params.Triggers) > 0 {
		query = query.Where("trigger IN ?", params.Triggers)
	}
	if !params.IncludeInactive {
		query = query.Where("is_active = ?", true)
	}
	if params.AfterPriority != nil && params.AfterRuleID != "" {
		query = query.Where("(priority < ?) OR (priority = ? AND rule_id > ?)", *params.AfterPriority, *params.AfterPriority, params.AfterRuleID)
	}

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	query = query.Order("priority DESC").Order("rule_id ASC")

	var rules []Rule
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *gormRepository) Update(ctx context.Context, rule *Rule) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}

	res := r.db.WithContext(ctx).
		Model(&Rule{}).
		Where("tenant_id = ? AND rule_id = ?", rule.TenantID, rule.RuleID).
		Updates(map[string]any{
			"name":           rule.Name,
			"description":    rule.Description,
			"is_active":      rule.IsActive,
			"priority":       rule.Priority,
			"trigger":        rule.Trigger,
			"dsl_expression": rule.DSLExpression,
			"actions":        rule.Actions,
			"updated_at":     rule.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormRepository) Delete(ctx context.Context, tenantID, ruleID string) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}

	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND rule_id = ?", tenantID, ruleID).
		Delete(&Rule{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormRepository) ListActiveByTrigger(ctx context.Context, tenantID string, trigger rulev1.RuleTriggerType) ([]Rule, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}

	query := r.db.WithContext(ctx).Model(&Rule{}).
		Where("tenant_id = ? AND trigger = ? AND is_active = ?", tenantID, trigger, true).
		Order("priority DESC").Order("rule_id ASC")

	var rules []Rule
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}
