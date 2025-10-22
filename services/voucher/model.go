package voucher

import (
	"time"

	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

type DiscountType string

const (
	DiscountTypeFixed   DiscountType = "FIXED"
	DiscountTypePercent DiscountType = "PERCENT"
	DiscountTypeCustom  DiscountType = "CUSTOM"
)

type Voucher struct {
	VoucherID     string       `gorm:"column:voucher_id;primaryKey"` // Snowflake string ID
	TenantID      string       `gorm:"column:tenant_id;index;not null"`
	CampaignID    string       `gorm:"column:campaign_id;index"`
	Code          string       `gorm:"column:code;uniqueIndex;not null"`
	Name          string       `gorm:"column:name;not null"`
	DiscountType  DiscountType `gorm:"column:discount_type;not null"` // fixed, percent, free_item, etc.
	DiscountValue float64      `gorm:"column:discount_value;not null;default:0"`
	CurrencyCode  string       `gorm:"column:currency_code;default:'IDR'"`
	ExpiryDate    *time.Time   `gorm:"column:expiry_date"`
	NeverExpires  bool         `gorm:"column:never_expires;default:false"`
	IsActive      bool         `gorm:"column:is_active;default:true"`
	CreatedAt     time.Time    `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time    `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Issuances []VoucherIssuance `gorm:"foreignKey:VoucherID;constraint:OnDelete:CASCADE"`
}

func (v *Voucher) ToProto() *voucherv1.Voucher {
	return &voucherv1.Voucher{
		VoucherId:     v.VoucherID,
		TenantId:      v.TenantID,
		CampaignId:    v.CampaignID,
		VoucherCode:   v.Code,
		VoucherName:   v.Name,
		DiscountType:  voucherv1.DiscountType(voucherv1.DiscountType_value[string(v.DiscountType)]),
		DiscountValue: v.DiscountValue,
		CurrencyCode:  v.CurrencyCode,
		ExpiryDate:    timestamppb.New(*v.ExpiryDate),
		IsActive:      v.IsActive,
		CreatedAt:     timestamppb.New(v.CreatedAt),
		UpdatedAt:     timestamppb.New(v.UpdatedAt),
	}
}

type VoucherIssueStatus string

// 'issued', 'redeemed', 'expired', 'canceled'
var (
	Issued   VoucherIssueStatus = "ISSUED"
	Redeemed VoucherIssueStatus = "REDEEMED"
	Expired  VoucherIssueStatus = "EXPIRED"
	Canceled VoucherIssueStatus = "CANCELLED"
)

func (s VoucherIssueStatus) String() string {
	switch s {
	case Issued, Redeemed, Expired, Canceled:
		return string(s)
	default:
		return ""
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
