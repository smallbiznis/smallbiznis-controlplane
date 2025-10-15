package task

import (
	"context"
	"encoding/json"
	"smallbiznis-controlplane/pkg/celengine"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/hibiken/asynq"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"go.uber.org/zap"
)

// BirthdayEventPayload digunakan untuk mendistribusikan voucher otomatis
// saat event "user.birthday" dipicu oleh event bus atau cron.
type BirthdayEventPayload struct {
	TenantID string    `json:"tenant_id"`
	UserID   string    `json:"user_id"`
	User     UserInfo  `json:"user"`
	Today    time.Time `json:"today"`
}

// UserInfo adalah subset dari model user yang relevan untuk evaluasi CEL.
// Kamu bisa isi sesuai dengan struktur user kamu di controlplane.
type UserInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	BirthDate time.Time `json:"birth_date,omitempty"`
	Tier      string    `json:"tier,omitempty"`
	City      string    `json:"city,omitempty"`
	Gender    string    `json:"gender,omitempty"`
}

func HandleBirthdayDistribution(svc voucherv1.VoucherServiceClient, ruleEngine *cel.Env) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload BirthdayEventPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return err
		}

		resp, err := svc.ListCampaigns(ctx, &voucherv1.ListCampaignsRequest{
			TenantId:   payload.TenantID,
			OnlyActive: true,
		})
		if err != nil {
			return err
		}

		attrs := map[string]interface{}{
			"user": payload.User,
			"event": map[string]interface{}{
				"type":  "birthday",
				"today": payload.Today.Format("2006-01-02"),
			},
		}

		env, err := celengine.BuildCelEnvFromAttributes(attrs)
		if err != nil {
			zap.L().Error("failed to build CEL environment", zap.Error(err))
			return err
		}

		for _, c := range resp.Campaigns {
			expr := c.DslExpression
			if expr == "" {
				zap.L().Debug("skip campaign without dsl_expression", zap.String("campaign_id", c.CampaignId))
				continue
			}

			ok, _ := celengine.Evaluate(env, expr, attrs)

			if !ok {
				zap.L().Debug("campaign rule not eligible", zap.String("campaign_id", c.CampaignId))
				continue
			}

			// 6️⃣ Issue voucher ke user
			_, issueErr := svc.IssueVoucher(ctx, &voucherv1.IssueVoucherRequest{
				TenantId:    payload.TenantID,
				UserId:      payload.UserID,
				VoucherCode: c.Code, // atau ambil random dari pool voucher
			})
			if issueErr != nil {
				zap.L().Error("failed to issue voucher", zap.String("campaign_id", c.CampaignId), zap.Error(issueErr))
				continue
			}

			zap.L().Info("issued voucher from campaign",
				zap.String("campaign_id", c.CampaignId),
				zap.String("voucher_code", c.Code),
				zap.String("user_id", payload.UserID),
			)

		}

		return nil
	}
}
