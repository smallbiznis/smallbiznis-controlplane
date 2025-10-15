package testdata

import (
	"encoding/json"
	"fmt"
	"smallbiznis-controlplane/services/rule"
	"time"

	"github.com/bwmarrin/snowflake"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"go.uber.org/fx"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var SeedRule = fx.Module("seed.rule",
	fx.Invoke(GenerateTestRules),
)

// GenerateTestRules inserts sample rules for testing
func GenerateTestRules(db *gorm.DB) error {
	node, err := snowflake.NewNode(1)
	if err != nil {
		return fmt.Errorf("failed to init snowflake node: %w", err)
	}

	now := time.Now()
	tenantID := "1977705426556293120"

	type pointAction struct {
		Points    int64             `json:"points"`
		Reference string            `json:"reference"`
		Metadata  map[string]string `json:"metadata,omitempty"`
	}

	type voucherAction struct {
		VoucherCode   string            `json:"voucherCode"`
		DiscountValue float64           `json:"discountValue"`
		DiscountType  string            `json:"discountType"`
		ExpiryDate    time.Time         `json:"expiryDate"`
		Metadata      map[string]string `json:"metadata,omitempty"`
	}

	type ruleAction struct {
		Type          string         `json:"type"`
		PointAction   *pointAction   `json:"pointAction,omitempty"`
		VoucherAction *voucherAction `json:"voucherAction,omitempty"`
	}

	makeActions := func(actions []ruleAction) datatypes.JSON {
		b, _ := json.Marshal(actions)
		return datatypes.JSON(b)
	}

	rules := []rule.Rule{
		{
			RuleID:        node.Generate().String(),
			TenantID:      tenantID,
			Name:          "Welcome Bonus",
			Description:   "Give 100 points to new users",
			IsActive:      true,
			Priority:      1,
			Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_USER_SIGNUP,
			DSLExpression: `is_new_user == true`,
			Actions: makeActions([]ruleAction{
				{
					Type: "RULE_ACTION_TYPE_REWARD_POINT",
					PointAction: &pointAction{
						Points:    100,
						Reference: "welcome_bonus",
						Metadata:  map[string]string{"channel": "signup"},
					},
				},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			RuleID:        node.Generate().String(),
			TenantID:      tenantID,
			Name:          "Gold Member Reward",
			Description:   "Give 500 points to gold tier users spending > 1,000,000",
			IsActive:      true,
			Priority:      2,
			Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_PURCHASE,
			DSLExpression: `user_tier == "gold" && total_spent > 1000000`,
			Actions: makeActions([]ruleAction{
				{
					Type: "RULE_ACTION_TYPE_REWARD_POINT",
					PointAction: &pointAction{
						Points:    500,
						Reference: "gold_reward",
						Metadata:  map[string]string{"tier": "gold"},
					},
				},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			RuleID:        node.Generate().String(),
			TenantID:      tenantID,
			Name:          "Voucher for Loyal Users",
			Description:   "Give 10% discount voucher for users with 10+ purchases",
			IsActive:      true,
			Priority:      3,
			Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_CAMPAIGN,
			DSLExpression: `purchase_count >= 10`,
			Actions: makeActions([]ruleAction{
				{
					Type: "RULE_ACTION_TYPE_VOUCHER",
					VoucherAction: &voucherAction{
						VoucherCode:   "LOYAL10",
						DiscountValue: 10,
						DiscountType:  "percent",
						ExpiryDate:    now.AddDate(0, 1, 0),
						Metadata:      map[string]string{"campaign": "loyalty_reward"},
					},
				},
			}),
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, rule := range rules {
		if err := db.Create(&rule).Error; err != nil {
			return fmt.Errorf("failed to insert rule %s: %w", rule.Name, err)
		}
		fmt.Printf("✅ Inserted rule: %s (%s)\n", rule.Name, rule.RuleID)
	}

	return nil
}
