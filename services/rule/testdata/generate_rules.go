package testdata

import (
	"fmt"
	"os"
	"time"

	"github.com/bwmarrin/snowflake"
	"go.uber.org/fx"
)

var SeedRule = fx.Module("seed.rule",
	fx.Invoke(GenerateTestRules),
)

// func GenerateTestRules(db *gorm.DB) error {
// 	node, err := snowflake.NewNode(1)
// 	if err != nil {
// 		return fmt.Errorf("failed to init snowflake node: %w", err)
// 	}

// 	now := time.Now()
// 	tenantID := "1978556034901348352"

// 	type pointAction struct {
// 		Points    int64             `json:"points"`
// 		Reference string            `json:"reference"`
// 		Metadata  map[string]string `json:"metadata,omitempty"`
// 	}

// 	type voucherAction struct {
// 		VoucherCode   string            `json:"voucher_code"`
// 		DiscountValue float64           `json:"discount_value"`
// 		DiscountType  string            `json:"discount_type"`
// 		ExpiryDate    time.Time         `json:"expiry_date"`
// 		Metadata      map[string]string `json:"metadata,omitempty"`
// 	}

// 	type notifyAction struct {
// 		Channel    string            `json:"channel,omitempty"`
// 		TemplateID string            `json:"template_id,omitempty"`
// 		Metadata   map[string]string `json:"metadata,omitempty"`
// 	}

// 	type ruleAction struct {
// 		Type          string         `json:"type"`
// 		PointAction   *pointAction   `json:"point_action,omitempty"`
// 		VoucherAction *voucherAction `json:"voucher_action,omitempty"`
// 		NotifyAction  *notifyAction  `json:"notif_action,omitempty"`
// 	}

// 	makeActions := func(actions []ruleAction) datatypes.JSON {
// 		b, _ := json.Marshal(actions)
// 		return datatypes.JSON(b)
// 	}

// 	rules := []rule.Rule{
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Welcome Bonus",
// 			Description:   "Give 100 points to new users",
// 			IsActive:      true,
// 			Priority:      1,
// 			Trigger:       rulev1.RuleTriggerType_USER_SIGNUP.String(),
// 			DSLExpression: `is_new_user == true`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_REWARD_POINT.String(),
// 					PointAction: &pointAction{
// 						Points:    100,
// 						Reference: "welcome_bonus",
// 						Metadata:  map[string]string{"channel": "signup"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Gold Member Reward",
// 			Description:   "Give 500 points to gold tier users spending > 1,000,000",
// 			IsActive:      true,
// 			Priority:      2,
// 			Trigger:       rulev1.RuleTriggerType_PURCHASE.String(),
// 			DSLExpression: `user_tier == "gold" && sub_total > 1000000`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_REWARD_POINT.String(),
// 					PointAction: &pointAction{
// 						Points:    500,
// 						Reference: "gold_reward",
// 						Metadata:  map[string]string{"tier": "gold"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Voucher for Loyal Users",
// 			Description:   "Give 10% discount voucher for users with 10+ purchases",
// 			IsActive:      true,
// 			Priority:      3,
// 			Trigger:       rulev1.RuleTriggerType_CAMPAIGN.String(),
// 			DSLExpression: `purchase_count >= 10`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_VOUCHER.String(),
// 					VoucherAction: &voucherAction{
// 						VoucherCode:   "LOYAL10",
// 						DiscountValue: 10,
// 						DiscountType:  "percent",
// 						ExpiryDate:    now.AddDate(0, 1, 0),
// 						Metadata:      map[string]string{"campaign": "loyalty_reward"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Small Purchase Bonus",
// 			Description:   "Reward 50 points for purchases between 100K–500K",
// 			IsActive:      true,
// 			Priority:      1,
// 			Trigger:       rulev1.RuleTriggerType_PURCHASE.String(),
// 			DSLExpression: `sub_total >= 100000 && sub_total < 500000`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_REWARD_POINT.String(),
// 					PointAction: &pointAction{
// 						Points:    50,
// 						Reference: "small_purchase_bonus",
// 						Metadata:  map[string]string{"segment": "small"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Medium Purchase Bonus",
// 			Description:   "Reward 150 points for purchases between 500K–1M",
// 			IsActive:      true,
// 			Priority:      2,
// 			Trigger:       rulev1.RuleTriggerType_PURCHASE.String(),
// 			DSLExpression: `sub_total >= 500000 && sub_total < 1000000`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_REWARD_POINT.String(),
// 					PointAction: &pointAction{
// 						Points:    150,
// 						Reference: "medium_purchase_bonus",
// 						Metadata:  map[string]string{"segment": "medium"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 		{
// 			RuleID:        node.Generate().String(),
// 			TenantID:      tenantID,
// 			Name:          "Big Spender Reward",
// 			Description:   "Reward 500 points and send notification for purchases over 1M",
// 			IsActive:      true,
// 			Priority:      3,
// 			Trigger:       rulev1.RuleTriggerType_PURCHASE.String(),
// 			DSLExpression: `sub_total >= 1000000`,
// 			Actions: makeActions([]ruleAction{
// 				{
// 					Type: rulev1.RuleActionType_REWARD_POINT.String(),
// 					PointAction: &pointAction{
// 						Points:    500,
// 						Reference: "big_spender_bonus",
// 						Metadata:  map[string]string{"segment": "big"},
// 					},
// 				},
// 				{
// 					Type: rulev1.RuleActionType_NOTIFY.String(),
// 					NotifyAction: &notifyAction{
// 						Channel:    "email",
// 						TemplateID: "notify-big-spender",
// 						Metadata:   map[string]string{"type": "reward_notification"},
// 					},
// 				},
// 			}),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		},
// 	}

// 	for _, rule := range rules {
// 		if err := db.Create(&rule).Error; err != nil {
// 			return fmt.Errorf("failed to insert rule %s: %w", rule.Name, err)
// 		}
// 		fmt.Printf("✅ Inserted rule: %s (%s)\n", rule.Name, rule.RuleID)
// 	}

// 	return nil
// }

func GenerateTestRules() {
	node, _ := snowflake.NewNode(1)
	now := time.Now().Format("2006-01-02 15:04:05")
	tenantID := "1978556034901348352"

	f, _ := os.Create("seed_rules.sql")
	defer f.Close()

	fmt.Fprintln(f, "-- Massive Seed Rules with text trigger")
	fmt.Fprintln(f, "BEGIN;")

	for i := 1; i <= 1000; i++ {
		ruleID := node.Generate().String()
		name := fmt.Sprintf("Auto Rule #%04d", i)
		expr := fmt.Sprintf(`sub_total >= %d && sub_total < %d`, 10000*i, 10000*(i+1))

		trigger := "PURCHASE"
		action := "REWARD_POINT"

		actionJSON := fmt.Sprintf(`[
			{
				"type": "%s",
				"point_action": {
					"points": %d,
					"reference": "auto_rule_%04d",
					"metadata": {"segment": "auto"}
				}
			}
		]`, action, (i%10+1)*10, i)

		sql := fmt.Sprintf(`
INSERT INTO rules (
	rule_id, tenant_id, name, description, is_active, priority, trigger, dsl_expression, actions, created_at, updated_at
) VALUES (
	'%s', '%s', '%s', 'Auto generated rule #%d',
	true, %d, '%s', '%s', '%s', '%s', '%s'
);`,
			ruleID, tenantID, name, i, i%5+1, trigger, expr, actionJSON, now, now)

		fmt.Fprintln(f, sql)
	}

	fmt.Fprintln(f, "COMMIT;")
	fmt.Println("✅ Generated seed_rules.sql (text trigger, 1000 rows)")
}
