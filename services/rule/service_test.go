package rule

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	rulev1 "github.com/smallbiznis/smallbiznisapis/smallbiznis/rule/v1"
	"smallbiznis-controlplane/services/testutil"
)

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db := testutil.NewTestDB(t, &Rule{})
	repo := NewRepository(db)
	evaluator := NewEvaluator()
	logger := zap.NewNop()
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	svc := NewService(ServiceParams{
		Repository: repo,
		Evaluator:  evaluator,
		Logger:     logger,
		Node:       node,
	})

	return svc, db
}

func TestService_EvaluateRule(t *testing.T) {
	svc, db := newTestService(t)

	rule := Rule{
		RuleID:      "rule-1",
		TenantID:    "tenant-1",
		Name:        "Gold Spender",
		Description: "Reward high value gold users",
		Expression:  "total_spent > 100000 && user_tier == 'gold'",
		ActionType:  rulev1.RuleActionType_RULE_ACTION_TYPE_REWARD_POINT,
		ActionValue: datatypes.JSONMap{"reward_points": 100.0},
		Status:      rulev1.RuleStatus_RULE_STATUS_ENABLED,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&rule).Error)

	ctxStruct, err := structpb.NewStruct(map[string]any{
		"total_spent": 120000.0,
		"user_tier":   "gold",
	})
	require.NoError(t, err)

	resp, err := svc.EvaluateRule(context.Background(), &rulev1.EvaluateRuleRequest{
		TenantId: "tenant-1",
		RuleId:   "rule-1",
		Context:  ctxStruct,
	})
	require.NoError(t, err)
	require.True(t, resp.GetMatched())
	require.NotNil(t, resp.GetActionValue())
	require.EqualValues(t, 100.0, resp.GetActionValue().AsMap()["reward_points"])
}

func TestService_BatchEvaluate(t *testing.T) {
	svc, db := newTestService(t)

	rules := []Rule{
		{
			RuleID:      "rule-a",
			TenantID:    "tenant-1",
			Expression:  "total_spent >= 100000",
			ActionType:  rulev1.RuleActionType_RULE_ACTION_TYPE_REWARD_POINT,
			ActionValue: datatypes.JSONMap{"reward_points": 50.0},
			Status:      rulev1.RuleStatus_RULE_STATUS_ENABLED,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			RuleID:      "rule-b",
			TenantID:    "tenant-1",
			Expression:  "user_tier == 'platinum'",
			ActionType:  rulev1.RuleActionType_RULE_ACTION_TYPE_NOTIFICATION,
			ActionValue: datatypes.JSONMap{"message": "Upgrade"},
			Status:      rulev1.RuleStatus_RULE_STATUS_ENABLED,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			RuleID:     "rule-disabled",
			TenantID:   "tenant-1",
			Expression: "true",
			Status:     rulev1.RuleStatus_RULE_STATUS_DISABLED,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}
	require.NoError(t, db.Create(&rules).Error)

	ctxStruct, err := structpb.NewStruct(map[string]any{
		"total_spent": 150000.0,
		"user_tier":   "gold",
	})
	require.NoError(t, err)

	resp, err := svc.BatchEvaluate(context.Background(), &rulev1.BatchEvaluateRequest{
		TenantId: "tenant-1",
		Context:  ctxStruct,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, resp.GetTotalRules())
	require.EqualValues(t, 1, resp.GetTotalMatched())
	require.Equal(t, "tenant-1", resp.GetTenantId())
	require.NotEmpty(t, resp.GetExecutionId())

	matched := false
	for _, result := range resp.GetResults() {
		switch result.GetRuleId() {
		case "rule-a":
			require.Equal(t, rulev1.EvaluationStatus_EVALUATION_STATUS_SUCCESS, result.GetStatus())
			require.True(t, result.GetMatched())
			require.EqualValues(t, 50.0, result.GetActionValue().AsMap()["reward_points"])
			matched = true
		case "rule-b":
			require.Equal(t, rulev1.EvaluationStatus_EVALUATION_STATUS_SUCCESS, result.GetStatus())
			require.False(t, result.GetMatched())
			require.Nil(t, result.GetActionValue())
		default:
			t.Fatalf("unexpected rule id %s", result.GetRuleId())
		}
	}
	require.True(t, matched, "expected rule-a to match")
}
