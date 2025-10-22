package loyalty

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/hibiken/asynq"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

var TaskModule = fx.Module("task.loyalty",
	fx.Provide(NewTask),
)

type Task struct {
	db    *gorm.DB
	node  *snowflake.Node
	asynq *asynq.Client

	rule    rulev1.RuleServiceClient
	ledger  ledgerv1.LedgerServiceClient
	voucher voucherv1.VoucherServiceClient
}

type TaskParams struct {
	fx.In

	DB    *gorm.DB
	Node  *snowflake.Node
	Asynq *asynq.Client

	Rule    rulev1.RuleServiceClient       `optional:"true"`
	Ledger  ledgerv1.LedgerServiceClient   `optional:"true"`
	Voucher voucherv1.VoucherServiceClient `optional:"true"`
}

func NewTask(p TaskParams) *Task {
	return &Task{
		db:      p.DB,
		node:    p.Node,
		asynq:   p.Asynq,
		rule:    p.Rule,
		ledger:  p.Ledger,
		voucher: p.Voucher,
	}
}

func (s *Task) HandleProcessEarningTask(ctx context.Context, t *asynq.Task) error {
	var payload ProcessEarningPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	zapLog := zap.L().With(
		zap.String("task_type", t.Type()),
		zap.String("tenant_id", payload.TenantID),
		zap.String("earning_id", payload.EarningID),
		zap.String("trace_id", payload.TraceID),
	)
	zapLog.Info("▶️ start process earning task")

	// 1️⃣ Ambil earning dari database
	var earning PointTransaction
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", payload.EarningID, payload.TenantID).
		First(&earning).Error; err != nil {
		zapLog.Error("failed to find earning record", zap.Error(err))
		return err
	}

	// 2️⃣ Parse metadata ke EarningRequest
	var req loyaltyv1.EarningRequest
	if err := protojson.Unmarshal(earning.Metadata, &req); err != nil {
		zapLog.Error("failed to parse earning metadata", zap.Error(err))
		return err
	}

	// 3️⃣ Panggil rule service (contoh pseudo)
	if err := s.evaluateRules(ctx, earning, &req); err != nil {
		zap.L().Error("failed evaluated", zap.Error(err))
		return fmt.Errorf("failed to evaluate rules: %w", err)
	}

	zapLog.Info("🎉 earning processed successfully")
	return nil
}

// evaluateRules contoh dummy (nanti diimplementasi pakai gRPC ke rule service)
func (s *Task) evaluateRules(ctx context.Context, earning PointTransaction, req *loyaltyv1.EarningRequest) error {
	zapLog := zap.L().With(
		zap.String("tenant_id", req.GetTenantId()),
		zap.String("earning_id", earning.ID),
		zap.String("user_id", req.GetUserId()),
	)

	// 1️⃣ Konversi attributes dari EarningRequest ke protobuf Struct
	attrMap := map[string]interface{}{
		"event_type": req.GetEventType(),
		"user_id":    req.GetUserId(),
	}

	switch v := req.Attributes.(type) {
	// use the generated oneof wrapper type for EarningRequest attributes
	case *loyaltyv1.EarningRequest_Transaction:

		if ta := req.GetTransaction(); ta != nil {
			attrMap["total_spent"] = ta.GetAmount().GetAmount()
			attrMap["currency_code"] = ta.GetAmount().GetCurrencyCode()
			attrMap["order_id"] = ta.GetOrderId()
			attrMap["channel"] = ta.GetChannel().String()
			attrMap["payment_method"] = ta.GetPaymentMethod().String()
			attrMap["brand_id"] = ta.GetBrandId()
			attrMap["merchant_id"] = ta.GetMerchantId()
			attrMap["outlet_id"] = ta.GetOutletId()
			attrMap["purchase_count"] = ta.GetOutletId()
		}

		if v.Transaction.Metadata != nil {
			for k, v := range v.Transaction.Metadata.AsMap() {
				attrMap[k] = v
			}
		}
	default:
		attrMap["event_type"] = "unknown"
	}

	contextStruct, err := structpb.NewStruct(attrMap)
	if err != nil {
		zapLog.Info("failed to build structpb", zap.Error(err))
		return err
	}

	// pass the built contextStruct to the rule service; log any error but continue (dummy)
	summary, err := s.rule.BatchEvaluate(ctx, &rulev1.BatchEvaluateRequest{
		TenantId: req.GetTenantId(),
		Context:  contextStruct,
	})
	if err != nil {
		zapLog.Error("batch evaluate failed", zap.Error(err))
		return err
	}

	// var totalPoints int64
	for _, res := range summary.Results {
		if !res.Matched {
			continue
		}

		val := res.ActionValue.AsMap()
		arr, ok := val["actions"].([]any)
		if !ok {
			continue
		}

		for _, raw := range arr {
			act, ok := raw.(map[string]any)
			if !ok {
				continue
			}

			actionType := act["type"].(string)

			switch actionType {
			case rulev1.RuleActionType_RULE_ACTION_TYPE_REWARD_POINT.String():

				var pts int64
				switch v := act["points"].(type) {
				case float64:
					pts = int64(v)
				case int:
					pts = int64(v)
				case string:
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						pts = n
					}
				}

				ref := fmt.Sprint(act["reference"])
				if ref == "" {
					ref = "unknown"
				}

				zapLog.Info("🪙 awarding points",
					zap.Int64("points", pts),
					zap.String("reference", ref),
				)

				// Record ke Ledger Service
				if _, err := s.recordLedger(ctx, &ledgerv1.AddEntryRequest{
					TenantId:    req.GetTenantId(),
					MemberId:    req.GetUserId(),
					Type:        ledgerv1.EntryType_CREDIT,
					Amount:      pts,
					ReferenceId: req.GetReferenceId(),
					Metadata:    map[string]string{"rule_ref": ref},
				}); err != nil {
					zapLog.Error("failed to record ledger", zap.String("reference_id", req.GetReferenceId()), zap.Error(err))
					return err
				}

				// Update point_transaction
				if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					// Ledger sudah dipanggil di luar
					return tx.Model(&PointTransaction{}).
						Where("id = ? AND tenant_id = ?", earning.ID, earning.TenantID).
						Updates(map[string]any{
							"point_delta": pts,
							"status":      loyaltyv1.Status_SUCCESS,
							"expire_date": DefaultEndOfYearDate(),
							"updated_at":  time.Now(),
						}).Error
				}); err != nil {
					return fmt.Errorf("failed to update earning: %w", err)
				}

			case "RULE_ACTION_TYPE_VOUCHER":
				campaignID := fmt.Sprint(act["campaign_id"])
				count := int32(1)

				if c, ok := act["count"].(float64); ok {
					count = int32(c)
				}

				zapLog.Info("🎟 issuing voucher",
					zap.String("campaign_id", campaignID),
					zap.Int32("count", count),
				)

				reqIssue := &voucherv1.IssueVoucherRequest{
					TenantId:   req.GetTenantId(),
					UserId:     req.GetUserId(),
					CampaignId: campaignID,
				}

				resp, err := s.voucher.IssueVoucher(ctx, reqIssue)
				if err != nil {
					zapLog.Error("failed to issue voucher", zap.Error(err))
					return err
				}

				zapLog.Info("✅ voucher issued",
					zap.String("campaign_id", campaignID),
					zap.Int("issued_count", len(resp.Vouchers)),
				)

				// case "RULE_ACTION_TYPE_NOTIFY":
				//     trigger notification...
			}
		}
	}

	return nil
}

// recordLedger contoh dummy (nanti pakai gRPC ke ledger service)
func (s *Task) recordLedger(ctx context.Context, req *ledgerv1.AddEntryRequest) (*ledgerv1.LedgerEntry, error) {
	return s.ledger.AddEntry(ctx, req)
}
