package orchestrator

import (
	"context"
	"fmt"
	"strconv"

	"smallbiznis-controlplane/pkg/celengine"

	"github.com/bwmarrin/snowflake"
	campaignv1 "github.com/smallbiznis/go-genproto/smallbiznis/campaign/v1"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	orchestratorv1 "github.com/smallbiznis/go-genproto/smallbiznis/orchestrator/v1"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

type Service struct {
	orchestratorv1.UnimplementedOrchestratorServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db   *gorm.DB
	node *snowflake.Node

	rule     rulev1.RuleServiceClient
	loyalty  loyaltyv1.PointServiceClient
	voucher  voucherv1.VoucherServiceClient
	campaign campaignv1.CampaignServiceClient
	ledger   ledgerv1.LedgerServiceClient
}

type Params struct {
	fx.In

	DB   *gorm.DB
	Node *snowflake.Node

	Rule     rulev1.RuleServiceClient         `optional:"true"`
	Loyalty  loyaltyv1.PointServiceClient     `optional:"true"`
	Voucher  voucherv1.VoucherServiceClient   `optional:"true"`
	Campaign campaignv1.CampaignServiceClient `optional:"true"`
	Ledger   ledgerv1.LedgerServiceClient     `optional:"true"`
}

func NewService(p Params) *Service {
	return &Service{
		db:       p.DB,
		node:     p.Node,
		rule:     p.Rule,
		loyalty:  p.Loyalty,
		voucher:  p.Voucher,
		campaign: p.Campaign,
		ledger:   p.Ledger,
	}
}

// ========================================================
// IMPLEMENTASI ORCHESTRATOR SERVICE
// ========================================================

func (s *Service) ProcessEarning(ctx context.Context, req *orchestratorv1.ProcessEarningRequest) (*orchestratorv1.ProcessEarningResponse, error) {
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	zapLog := zap.L().With(
		zap.String("trace_id", traceID),
		zap.String("tenant_id", req.GetTenantId()),
		zap.String("user_id", req.GetUserId()),
		zap.String("event_type", req.GetEventType().String()),
	)

	zapLog.Info("▶️ start ProcessEarning")

	contextStruct, err := s.buildContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("invalid attributes: %w", err)
	}

	// 2️⃣ Evaluasi ke Rule Engine
	ruleResp, err := s.rule.BatchEvaluate(ctx, &rulev1.BatchEvaluateRequest{
		TenantId: req.GetTenantId(),
		Context:  contextStruct,
		Trigger:  rulev1.RuleTriggerType(req.EventType),
	})
	if err != nil {
		zapLog.Error("failed to evaluate rules", zap.Error(err))
		return nil, err
	}

	zapLog.Debug("Result", zap.Any("result", ruleResp))

	results := []*orchestratorv1.EarningResult{}

	// 3️⃣ Loop setiap hasil evaluasi rule
	for _, r := range ruleResp.Results {
		if !r.Matched {
			continue
		}

		actionVal := r.ActionValue.AsMap()
		arr, ok := actionVal["actions"].([]any)
		if !ok {
			continue
		}

		for _, raw := range arr {
			act, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			zap.L().Debug("action", zap.Any("act", act))
			actionType := act["type"].(string)
			switch actionType {

			// 🪙 REWARD POINT
			case rulev1.RuleActionType_REWARD_POINT.String():
				points := toInt64(act["points"])
				ref := fmt.Sprint(act["reference"])
				if ref == "" {
					ref = fmt.Sprintf("rule_%s", r.RuleId)
				}

				// Call Loyalty service
				_, err := s.loyalty.AddPoint(ctx, &loyaltyv1.AddPointsRequest{
					TenantId:    req.GetTenantId(),
					UserId:      req.GetUserId(),
					RuleId:      r.GetRuleId(),
					Points:      points,
					Description: ref,
					ReferenceId: req.GetReferenceId(),
				})
				status := orchestratorv1.EarningStatus_SUCCESS
				msg := fmt.Sprintf("Added %d points", points)
				if err != nil {
					status = orchestratorv1.EarningStatus_FAILED
					msg = err.Error()
					zapLog.Error("failed to add points", zap.Error(err))
				}

				results = append(results, &orchestratorv1.EarningResult{
					CampaignId:      r.RuleId,
					Matched:         true,
					RewardType:      "POINT",
					RewardValue:     toStruct(map[string]any{"points": points}),
					ExecutorService: "loyalty",
					Status:          status,
					Message:         msg,
				})

			// 🎟️ ISSUE VOUCHER
			case rulev1.RuleActionType_VOUCHER.String():
				campaignID := fmt.Sprint(act["campaign_id"])
				count := int32(1)
				if v, ok := act["count"].(float64); ok {
					count = int32(v)
				}

				resp, err := s.voucher.IssueVoucher(ctx, &voucherv1.IssueVoucherRequest{
					TenantId:   req.GetTenantId(),
					UserId:     req.GetUserId(),
					CampaignId: campaignID,
				})
				status := orchestratorv1.EarningStatus_SUCCESS
				msg := fmt.Sprintf("Issued %d vouchers", len(resp.GetVouchers()))
				if err != nil {
					status = orchestratorv1.EarningStatus_FAILED
					msg = err.Error()
					zapLog.Error("failed to issue voucher", zap.Error(err))
				}

				results = append(results, &orchestratorv1.EarningResult{
					CampaignId:      campaignID,
					Matched:         true,
					RewardType:      "VOUCHER",
					RewardValue:     toStruct(map[string]any{"count": count}),
					ExecutorService: "voucher",
					Status:          status,
					Message:         msg,
				})

			// 🔔 NOTIFY USER (future)
			case rulev1.RuleActionType_NOTIFY.String():
				results = append(results, &orchestratorv1.EarningResult{
					CampaignId:      r.RuleId,
					Matched:         true,
					RewardType:      "NOTIFY",
					RewardValue:     toStruct(act),
					ExecutorService: "notification",
					Status:          orchestratorv1.EarningStatus_PENDING,
					Message:         "notification scheduled",
				})
			}
		}
	}

	// 4️⃣ (Optional) Fetch current balance from Ledger
	balanceResp, err := s.ledger.GetBalance(ctx, &ledgerv1.GetBalanceRequest{
		TenantId: req.GetTenantId(),
		MemberId: req.GetUserId(),
	})
	if err != nil {
		zapLog.Warn("unable to fetch balance from ledger", zap.Error(err))
	}

	zapLog.Info("🎉 ProcessEarning completed",
		zap.Int("results", len(results)),
		zap.Int64("current_balance", balanceResp.GetBalance()),
	)

	return &orchestratorv1.ProcessEarningResponse{
		TraceId:       traceID,
		Results:       results,
		OverallStatus: orchestratorv1.EarningStatus_SUCCESS,
	}, nil
}

func (s *Service) buildContext(ctx context.Context, req *orchestratorv1.ProcessEarningRequest) (*structpb.Struct, error) {
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	zapLog := zap.L().With(
		zap.String("trace_id", traceID),
		zap.String("tenant_id", req.GetTenantId()),
		zap.String("user_id", req.GetUserId()),
		zap.String("event_type", req.GetEventType().String()),
	)

	zapLog.Info("▶️ start buildContext")

	attrMap := map[string]interface{}{
		"user_id": req.GetUserId(),
	}

	switch v := req.Attributes.(type) {
	case *orchestratorv1.ProcessEarningRequest_Transaction:

		if ta := req.GetTransaction(); ta != nil {
			zap.L().Debug("Transaction", zap.Any("transaction_attribute", ta))
			for k, v := range celengine.StructToMap(req.GetTransaction()) {
				attrMap[k] = v
			}
		}

		if v.Transaction.Metadata != nil {
			for k, v := range v.Transaction.Metadata.AsMap() {
				attrMap[k] = v
			}
		}
	default:
		attrMap["event_type"] = "unknown"
	}

	zapLog.Debug("attributes", zap.Any("attr", attrMap))

	contextStruct, err := structpb.NewStruct(attrMap)
	if err != nil {
		zapLog.Info("failed to build structpb", zap.Error(err))
		return nil, err
	}

	return contextStruct, nil
}

// ========================================================
// HELPERS
// ========================================================

func toStruct(v map[string]any) *structpb.Struct {
	s, _ := structpb.NewStruct(v)
	return s
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	default:
		return 0
	}
}
