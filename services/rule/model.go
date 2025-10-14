package rule

import (
	"encoding/json"
	"time"

	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

// Rule represents a persisted rule definition.
type Rule struct {
	RuleID        string                 `gorm:"column:rule_id;primaryKey"`
	TenantID      string                 `gorm:"column:tenant_id;index"`
	Name          string                 `gorm:"column:name"`
	Description   string                 `gorm:"column:description"`
	IsActive      bool                   `gorm:"column:is_active"`
	Priority      int32                  `gorm:"column:priority"`
	Trigger       rulev1.RuleTriggerType `gorm:"column:trigger"`
	DSLExpression string                 `gorm:"column:dsl_expression"`
	Actions       datatypes.JSON         `gorm:"column:actions"`
	CreatedAt     time.Time              `gorm:"column:created_at"`
	UpdatedAt     time.Time              `gorm:"column:updated_at"`
}

// TableName defines the storage table name for Rule records.
func (Rule) TableName() string { return "rules" }

// RuleAction describes a rule action payload stored as JSON.
type RuleAction struct {
	Type           rulev1.RuleActionType `json:"type"`
	PointAction    *PointAction          `json:"point_action,omitempty"`
	VoucherAction  *VoucherAction        `json:"voucher_action,omitempty"`
	CashbackAction *CashbackAction       `json:"cashback_action,omitempty"`
	NotifyAction   *NotifyAction         `json:"notify_action,omitempty"`
	TagAction      *TagAction            `json:"tag_action,omitempty"`
}

// PointAction mirrors rulev1.PointAction for JSON storage.
type PointAction struct {
	Points    int32             `json:"points"`
	Reference string            `json:"reference,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// VoucherAction mirrors rulev1.VoucherAction for JSON storage.
type VoucherAction struct {
	VoucherCode   string            `json:"voucher_code"`
	DiscountValue float64           `json:"discount_value"`
	DiscountType  string            `json:"discount_type"`
	ExpiryDate    *time.Time        `json:"expiry_date,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// CashbackAction mirrors rulev1.CashbackAction for JSON storage.
type CashbackAction struct {
	Amount         float64           `json:"amount"`
	Currency       string            `json:"currency,omitempty"`
	TargetWalletID string            `json:"target_wallet_id,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// NotifyAction mirrors rulev1.NotifyAction for JSON storage.
type NotifyAction struct {
	Channel    string            `json:"channel,omitempty"`
	TemplateID string            `json:"template_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TagAction mirrors rulev1.TagAction for JSON storage.
type TagAction struct {
	TagKey   string `json:"tag_key,omitempty"`
	TagValue string `json:"tag_value,omitempty"`
}

// SetActions serialises the provided actions list into the JSON column.
func (r *Rule) SetActions(actions []RuleAction) error {
	if len(actions) == 0 {
		r.Actions = datatypes.JSON([]byte("[]"))
		return nil
	}

	raw, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	r.Actions = datatypes.JSON(raw)
	return nil
}

// ActionsList deserialises the stored JSON into a slice of RuleAction.
func (r *Rule) ActionsList() ([]RuleAction, error) {
	if len(r.Actions) == 0 {
		return nil, nil
	}
	var actions []RuleAction
	if err := json.Unmarshal(r.Actions, &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

// ToProto converts the persisted model into its protobuf representation.
func (r *Rule) ToProto() (*rulev1.Rule, error) {
	actions, err := r.ActionsList()
	if err != nil {
		return nil, err
	}

	protoActions, err := ProtoActionsFromModel(actions)
	if err != nil {
		return nil, err
	}

	out := &rulev1.Rule{
		RuleId:        r.RuleID,
		OrgId:         r.TenantID,
		Name:          r.Name,
		Description:   r.Description,
		IsActive:      r.IsActive,
		Priority:      r.Priority,
		Trigger:       r.Trigger,
		DslExpression: r.DSLExpression,
		Actions:       protoActions,
	}

	if !r.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(r.CreatedAt)
	}
	if !r.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(r.UpdatedAt)
	}

	return out, nil
}

// ModelActionsFromProto converts protobuf actions into JSON friendly structures.
func ModelActionsFromProto(src []*rulev1.RuleAction) ([]RuleAction, error) {
	if len(src) == 0 {
		return nil, nil
	}

	out := make([]RuleAction, 0, len(src))
	for _, action := range src {
		if action == nil {
			continue
		}
		model := RuleAction{Type: action.GetType()}
		if point := action.GetPointAction(); point != nil {
			model.PointAction = &PointAction{
				Points:    point.GetPoints(),
				Reference: point.GetReference(),
				Metadata:  cloneMap(point.GetMetadata()),
			}
		}
		if voucher := action.GetVoucherAction(); voucher != nil {
			var expiry *time.Time
			if ts := voucher.GetExpiryDate(); ts != nil {
				t := ts.AsTime()
				expiry = &t
			}
			model.VoucherAction = &VoucherAction{
				VoucherCode:   voucher.GetVoucherCode(),
				DiscountValue: voucher.GetDiscountValue(),
				DiscountType:  voucher.GetDiscountType(),
				ExpiryDate:    expiry,
				Metadata:      cloneMap(voucher.GetMetadata()),
			}
		}
		if cashback := action.GetCashbackAction(); cashback != nil {
			model.CashbackAction = &CashbackAction{
				Amount:         cashback.GetAmount(),
				Currency:       cashback.GetCurrency(),
				TargetWalletID: cashback.GetTargetWalletId(),
				Metadata:       cloneMap(cashback.GetMetadata()),
			}
		}
		if notify := action.GetNotifyAction(); notify != nil {
			model.NotifyAction = &NotifyAction{
				Channel:    notify.GetChannel(),
				TemplateID: notify.GetTemplateId(),
				Metadata:   cloneMap(notify.GetMetadata()),
			}
		}
		if tag := action.GetTagAction(); tag != nil {
			model.TagAction = &TagAction{
				TagKey:   tag.GetTagKey(),
				TagValue: tag.GetTagValue(),
			}
		}
		out = append(out, model)
	}
	return out, nil
}

// ProtoActionsFromModel converts stored JSON actions into protobuf representations.
func ProtoActionsFromModel(actions []RuleAction) ([]*rulev1.RuleAction, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	out := make([]*rulev1.RuleAction, 0, len(actions))
	for _, action := range actions {
		proto := &rulev1.RuleAction{Type: action.Type}
		switch action.Type {
		case rulev1.RuleActionType_RULE_ACTION_TYPE_EARN_POINT,
			rulev1.RuleActionType_RULE_ACTION_TYPE_REDEEM_POINT:
			if action.PointAction != nil {
				proto.Payload = &rulev1.RuleAction_PointAction{PointAction: &rulev1.PointAction{
					Points:    action.PointAction.Points,
					Reference: action.PointAction.Reference,
					Metadata:  cloneMap(action.PointAction.Metadata),
				}}
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_ISSUE_VOUCHER:
			if action.VoucherAction != nil {
				voucher := &rulev1.VoucherAction{
					VoucherCode:   action.VoucherAction.VoucherCode,
					DiscountValue: action.VoucherAction.DiscountValue,
					DiscountType:  action.VoucherAction.DiscountType,
					Metadata:      cloneMap(action.VoucherAction.Metadata),
				}
				if action.VoucherAction.ExpiryDate != nil {
					voucher.ExpiryDate = timestamppb.New(*action.VoucherAction.ExpiryDate)
				}
				proto.Payload = &rulev1.RuleAction_VoucherAction{VoucherAction: voucher}
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_CASHBACK:
			if action.CashbackAction != nil {
				proto.Payload = &rulev1.RuleAction_CashbackAction{CashbackAction: &rulev1.CashbackAction{
					Amount:         action.CashbackAction.Amount,
					Currency:       action.CashbackAction.Currency,
					TargetWalletId: action.CashbackAction.TargetWalletID,
					Metadata:       cloneMap(action.CashbackAction.Metadata),
				}}
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_NOTIFY:
			if action.NotifyAction != nil {
				proto.Payload = &rulev1.RuleAction_NotifyAction{NotifyAction: &rulev1.NotifyAction{
					Channel:    action.NotifyAction.Channel,
					TemplateId: action.NotifyAction.TemplateID,
					Metadata:   cloneMap(action.NotifyAction.Metadata),
				}}
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_TAG_CUSTOMER:
			if action.TagAction != nil {
				proto.Payload = &rulev1.RuleAction_TagAction{TagAction: &rulev1.TagAction{
					TagKey:   action.TagAction.TagKey,
					TagValue: action.TagAction.TagValue,
				}}
			}
		default:
			// leave payload nil for unsupported types to preserve round trip
			if action.PointAction != nil {
				proto.Payload = &rulev1.RuleAction_PointAction{PointAction: &rulev1.PointAction{
					Points:    action.PointAction.Points,
					Reference: action.PointAction.Reference,
					Metadata:  cloneMap(action.PointAction.Metadata),
				}}
			}
		}
		out = append(out, proto)
	}
	return out, nil
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
