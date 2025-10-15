package voucher

import (
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

type VoucherIssueStatus string

// 'issued', 'redeemed', 'expired', 'canceled'
var (
	Issued   VoucherIssueStatus = "issued"
	Redeemed VoucherIssueStatus = "redeemed"
	Expired  VoucherIssueStatus = "expired"
	Canceled VoucherIssueStatus = "cancelled"
)

func (s VoucherIssueStatus) String() string {
	switch s {
	case Issued, Redeemed, Expired, Canceled:
		return string(s)
	default:
		return ""
	}
}

// VoucherCampaign represents a promotional campaign (top-level container).
type VoucherCampaign struct {
	CampaignID     snowflake.ID           `gorm:"column:campaign_id;primaryKey;autoIncrement"`
	TenantID       snowflake.ID           `gorm:"column:tenant_id;index;not null"`
	Name           string                 `gorm:"column:name;not null"`
	Code           string                 `gorm:"column:code"`
	Description    string                 `gorm:"column:description"`
	Type           voucherv1.CampaignType `gorm:"column:type;not null"`
	TotalStock     int32                  `gorm:"column:total_stock;not null;default:0"`
	RemainingStock int32                  `gorm:"column:remaining_stock;not null;default:0"`
	IsActive       bool                   `gorm:"column:is_active;default:true"`
	StartAt        *time.Time             `gorm:"column:start_at"`
	EndAt          *time.Time             `gorm:"column:end_at"`
	DslExpression  datatypes.JSON         `gorm:"column:dsl_expression;type:text"` // CEL rule for eligibility
	Metadata       datatypes.JSON         `gorm:"column:metadata;type:jsonb"`
	CreatedAt      time.Time              `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time              `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Vouchers []Voucher `gorm:"foreignKey:CampaignID;constraint:OnDelete:CASCADE"`
}

func (m *VoucherCampaign) ToProto() *voucherv1.VoucherCampaign {

	vouchers := make([]*voucherv1.Voucher, 0)
	if len(m.Vouchers) > 0 {
		for _, v := range m.Vouchers {
			vouchers = append(vouchers, v.ToProto())
		}
	}

	var metadataStruct *structpb.Struct
	if m.Metadata != nil {
		metadataStruct, _ = structpb.NewStruct(map[string]interface{}{})
		_ = metadataStruct.UnmarshalJSON(m.Metadata)
	}

	return &voucherv1.VoucherCampaign{
		CampaignId:     m.CampaignID.String(),
		TenantId:       m.TenantID.String(),
		Name:           m.Name,
		Code:           m.Code,
		Description:    m.Description,
		Type:           m.Type,
		TotalStock:     m.TotalStock,
		RemainingStock: m.RemainingStock,
		IsActive:       m.IsActive,
		StartAt:        timestamppb.New(*m.StartAt),
		EndAt:          timestamppb.New(*m.EndAt),
		Metadata:       metadataStruct,
	}
}

type Voucher struct {
	VoucherID      string                 `gorm:"column:voucher_id;primaryKey"` // Snowflake string ID
	TenantID       string                 `gorm:"column:tenant_id;index;not null"`
	CampaignID     string                 `gorm:"column:campaign_id;index"`
	Code           string                 `gorm:"column:code;uniqueIndex;not null"`
	Name           string                 `gorm:"column:name;not null"`
	DiscountType   voucherv1.DiscountType `gorm:"column:discount_type;not null"` // fixed, percent, free_item, etc.
	DiscountValue  float64                `gorm:"column:discount_value;not null;default:0"`
	Currency       string                 `gorm:"column:currency;default:'IDR'"`
	Stock          int32                  `gorm:"column:stock;not null;default:0"`
	RemainingStock int32                  `gorm:"column:remaining_stock;not null;default:0"`
	ExpiryDate     *time.Time             `gorm:"column:expiry_date"`
	IsActive       bool                   `gorm:"column:is_active;default:true"`
	CreatedAt      time.Time              `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time              `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Campaign   *VoucherCampaign  `gorm:"foreignKey:CampaignID;references:CampaignID;constraint:OnDelete:SET NULL"`
	Issuances  []VoucherIssuance `gorm:"foreignKey:VoucherID;constraint:OnDelete:CASCADE"`
	VoucherEvt []VoucherEvent    `gorm:"foreignKey:VoucherID;constraint:OnDelete:CASCADE"`
}

func (v *Voucher) ToProto() *voucherv1.Voucher {
	return &voucherv1.Voucher{
		VoucherId:      v.VoucherID,
		TenantId:       v.TenantID,
		CampaignId:     fmt.Sprint(v.CampaignID),
		Code:           v.Code,
		Name:           v.Name,
		DiscountType:   v.DiscountType,
		DiscountValue:  v.DiscountValue,
		Currency:       v.Currency,
		Stock:          int32(v.Stock),
		RemainingStock: int32(v.RemainingStock),
		ExpiryDate:     timestamppb.New(*v.ExpiryDate),
		IsActive:       v.IsActive,
		CreatedAt:      timestamppb.New(v.CreatedAt),
		UpdatedAt:      timestamppb.New(v.UpdatedAt),
	}
}

// VoucherIssuance represents a voucher instance given to a specific user.
type VoucherIssuance struct {
	IssuanceID string                   `gorm:"column:issuance_id;primaryKey;autoIncrement"`
	TenantID   string                   `gorm:"column:tenant_id;index;not null"`
	VoucherID  string                   `gorm:"column:voucher_id;index;not null"`
	UserID     string                   `gorm:"column:user_id;index;not null"`
	Status     voucherv1.IssuanceStatus `gorm:"column:status;not null;default:'issued'"` // issued, redeemed, expired, canceled
	IssuedAt   time.Time                `gorm:"column:issued_at;autoCreateTime"`
	RedeemedAt *time.Time               `gorm:"column:redeemed_at"`
	OrderID    *string                  `gorm:"column:order_id"`
	Metadata   datatypes.JSON           `gorm:"column:metadata;type:jsonb"`
	CreatedAt  time.Time                `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time                `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Voucher *Voucher `gorm:"foreignKey:VoucherID;references:VoucherID;constraint:OnDelete:CASCADE"`
}

func (v *VoucherIssuance) ToProto() *voucherv1.VoucherIssuance {
	var metadataStruct *structpb.Struct
	if v.Metadata != nil {
		metadataStruct, _ = structpb.NewStruct(map[string]interface{}{})
		_ = metadataStruct.UnmarshalJSON(v.Metadata)
	}

	return &voucherv1.VoucherIssuance{
		IssuanceId: v.IssuanceID,
		TenantId:   v.TenantID,
		VoucherId:  v.VoucherID,
		UserId:     v.UserID,
		Status:     v.Status,
		IssuedAt:   timestamppb.New(v.IssuedAt),
		RedeemedAt: timestamppb.New(*v.RedeemedAt),
		OrderId:    *v.OrderID,
		Metadata:   metadataStruct,
		CreatedAt:  timestamppb.New(v.CreatedAt),
		UpdatedAt:  timestamppb.New(v.UpdatedAt),
	}
}

// VoucherEvent stores audit logs of voucher lifecycle.
type VoucherEvent struct {
	EventID    int64                  `gorm:"column:event_id;primaryKey;autoIncrement"`
	TenantID   string                 `gorm:"column:tenant_id;index;not null"`
	VoucherID  int64                  `gorm:"column:voucher_id;index"`
	IssuanceID *int64                 `gorm:"column:issuance_id"`
	UserID     *string                `gorm:"column:user_id"`
	EventType  string                 `gorm:"column:event_type;not null"` // issued, redeemed, expired, restocked
	Payload    map[string]interface{} `gorm:"column:payload;type:jsonb"`
	CreatedAt  time.Time              `gorm:"column:created_at;autoCreateTime"`
}
