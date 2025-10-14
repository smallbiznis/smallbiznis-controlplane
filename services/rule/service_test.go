package rule

import (
	"context"
	"strconv"
	"testing"

	"github.com/bwmarrin/snowflake"
	common "github.com/smallbiznis/go-genproto/smallbiznis/common"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

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

func withOrgContext(ctx context.Context, orgID string) context.Context {
	md := metadata.New(map[string]string{metadataOrgIDKey: orgID})
	return metadata.NewIncomingContext(ctx, md)
}

func TestService_CreateAndGetRule(t *testing.T) {
	svc, _ := newTestService(t)

	ctx := withOrgContext(context.Background(), "org-1")

	createResp, err := svc.CreateRule(ctx, &rulev1.CreateRuleRequest{
		Name:          "High spender",
		Description:   "reward high spending orders",
		IsActive:      true,
		Priority:      10,
		Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_ORDER_CREATED,
		DslExpression: "total_spent > 100",
		Actions: []*rulev1.RuleAction{
			{
				Type: rulev1.RuleActionType_RULE_ACTION_TYPE_EARN_POINT,
				Payload: &rulev1.RuleAction_PointAction{PointAction: &rulev1.PointAction{
					Points: 50,
				}},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "org-1", createResp.GetOrgId())
	require.Equal(t, int32(10), createResp.GetPriority())
	require.Len(t, createResp.GetActions(), 1)

	getResp, err := svc.GetRule(ctx, &rulev1.GetRuleRequest{RuleId: createResp.GetRuleId()})
	require.NoError(t, err)
	require.Equal(t, createResp.GetRuleId(), getResp.GetRuleId())
	require.True(t, getResp.GetIsActive())
}

func TestService_ListRulesPagination(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := withOrgContext(context.Background(), "org-1")

	for i := 0; i < 3; i++ {
		_, err := svc.CreateRule(ctx, &rulev1.CreateRuleRequest{
			Name:          "rule" + strconv.Itoa(i),
			IsActive:      true,
			Priority:      int32(i),
			Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_ORDER_CREATED,
			DslExpression: "true",
		})
		require.NoError(t, err)
	}

	listResp, err := svc.ListRules(ctx, &rulev1.ListRulesRequest{
		Paging: &common.CursorRequest{Limit: 2},
	})
	require.NoError(t, err)
	require.Len(t, listResp.GetRules(), 2)
	require.True(t, listResp.GetPaging().GetHasMore())
	cursor := listResp.GetPaging().GetNextCursor()
	require.NotEmpty(t, cursor)

	secondResp, err := svc.ListRules(ctx, &rulev1.ListRulesRequest{
		Paging: &common.CursorRequest{Cursor: cursor, Limit: 2},
	})
	require.NoError(t, err)
	require.LessOrEqual(t, len(secondResp.GetRules()), 1)
}

func TestService_EvaluateRules(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := withOrgContext(context.Background(), "org-1")

	_, err := svc.CreateRule(ctx, &rulev1.CreateRuleRequest{
		Name:          "Earn points",
		IsActive:      true,
		Priority:      10,
		Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_ORDER_CREATED,
		DslExpression: "total_spent > 100",
		Actions: []*rulev1.RuleAction{
			{
				Type: rulev1.RuleActionType_RULE_ACTION_TYPE_EARN_POINT,
				Payload: &rulev1.RuleAction_PointAction{PointAction: &rulev1.PointAction{
					Points: 25,
				}},
			},
		},
	})
	require.NoError(t, err)

	_, err = svc.CreateRule(ctx, &rulev1.CreateRuleRequest{
		Name:          "High tier",
		IsActive:      true,
		Priority:      5,
		Trigger:       rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_ORDER_CREATED,
		DslExpression: "total_spent > 1000",
	})
	require.NoError(t, err)

	evalResp, err := svc.EvaluateRules(ctx, &rulev1.EvaluateRuleRequest{
		Trigger: rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_ORDER_CREATED.String(),
		Attributes: map[string]string{
			"total_spent": "150",
		},
	})
	require.NoError(t, err)
	require.Len(t, evalResp.GetMatchedRules(), 1)
	require.Equal(t, int32(25), evalResp.GetTotalPoints())
}
