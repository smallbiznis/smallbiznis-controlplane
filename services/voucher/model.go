package voucher

import (
	"time"

	"gorm.io/datatypes"
)

type DiscountType string
type IssuanceStatus string

const (
	DiscountTypeFixed   DiscountType = "FIXED"
	DiscountTypePercent DiscountType = "PERCENT"

	IssuanceStatusIssued   IssuanceStatus = "ISSUED"
	IssuanceStatusRedeemed IssuanceStatus = "REDEEMED"
	IssuanceStatusExpired  IssuanceStatus = "EXPIRED"
	IssuanceStatusCanceled IssuanceStatus = "CANCELED"
)

// ========================================================
// TABLE: vouchers
// ========================================================
type Voucher struct {
	VoucherID     string         `gorm:"column:voucher_id;primaryKey;type:char(26)"`
	TenantID      string         `gorm:"column:tenant_id;index;type:char(26);not null"`
	CampaignID    *string        `gorm:"column:campaign_id;index;type:char(26)"`
	Code          string         `gorm:"column:code;uniqueIndex;type:varchar(100);not null"`
	Name          string         `gorm:"column:name;type:varchar(255);not null"`
	DiscountType  DiscountType   `gorm:"column:discount_type;type:varchar(20);not null"`
	DiscountValue float64        `gorm:"column:discount_value;not null;default:0"`
	CurrencyCode  string         `gorm:"column:currency_code;type:varchar(10);default:'IDR'"`
	ExpiryDate    *time.Time     `gorm:"column:expiry_date"`
	NeverExpires  bool           `gorm:"column:never_expires;default:false"`
	IsActive      bool           `gorm:"column:is_active;default:true"`
	Metadata      datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Issuances []VoucherIssuance `gorm:"foreignKey:VoucherID;constraint:OnDelete:SET NULL"`
}

// ========================================================
// TABLE: voucher_pools
// ========================================================
type VoucherPool struct {
	PoolID         string    `gorm:"column:pool_id;primaryKey;type:char(26)"`
	TenantID       string    `gorm:"column:tenant_id;index;type:char(26);not null"`
	CampaignID     *string   `gorm:"column:campaign_id;index;type:char(26)"`
	Name           string    `gorm:"column:name;type:varchar(255);not null"`
	TotalStock     int32     `gorm:"column:total_stock;default:0"`
	RemainingStock int32     `gorm:"column:remaining_stock;default:0"`
	IsActive       bool      `gorm:"column:is_active;default:true"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// Relations
	Items []VoucherPoolItem `gorm:"foreignKey:PoolID;constraint:OnDelete:CASCADE"`
}

// ========================================================
// TABLE: voucher_pool_items
// ========================================================
type VoucherPoolItem struct {
	ItemID          string         `gorm:"column:item_id;primaryKey;type:char(26)"`
	TenantID        string         `gorm:"column:tenant_id;index;type:char(26);not null"`
	PoolID          string         `gorm:"column:pool_id;index;type:char(26);not null"`
	VoucherCodeHash string         `gorm:"column:voucher_code_hash;type:char(64);not null"`
	VoucherCodeEnc  string         `gorm:"column:voucher_code_enc;type:text;not null"`
	KeyVersion      string         `gorm:"column:key_version;type:varchar(32)"`
	IsIssued        bool           `gorm:"column:is_issued;index;default:false"`
	IssuedTo        *string        `gorm:"column:issued_to;type:char(26)"`
	IssuedAt        *time.Time     `gorm:"column:issued_at"`
	Metadata        datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime"`

	// Constraints
	// Unique per pool & hash agar tidak duplikat
	// gorm:"uniqueIndex:ux_pool_code_hash,unique,columns:pool_id,voucher_code_hash"
}

// ========================================================
// TABLE: voucher_issuances
// ========================================================
type VoucherIssuance struct {
	IssuanceID     string         `gorm:"column:issuance_id;primaryKey;type:char(26)"`
	TenantID       string         `gorm:"column:tenant_id;index;type:char(26);not null"`
	PoolItemID     *string        `gorm:"column:pool_item_id;index;type:char(26)"`
	VoucherID      *string        `gorm:"column:voucher_id;index;type:char(26)"`
	UserID         string         `gorm:"column:user_id;index;type:char(26);not null"`
	Status         IssuanceStatus `gorm:"column:status;type:varchar(20);not null;default:'ISSUED'"`
	IssuedAt       time.Time      `gorm:"column:issued_at;autoCreateTime"`
	RedeemedAt     *time.Time     `gorm:"column:redeemed_at"`
	OrderID        *string        `gorm:"column:order_id;type:char(26)"`
	IdempotencyKey string         `gorm:"column:idempotency_key;type:varchar(64);uniqueIndex:ux_tenant_idempotent,priority:2"`
	Metadata       datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime"`

	// Constraints
	// Idempotency: unik per tenant + idempotency key
	// gorm:"uniqueIndex:ux_tenant_idempotent,priority:1"

	// Relations
	Voucher  *Voucher         `gorm:"foreignKey:VoucherID;references:VoucherID"`
	PoolItem *VoucherPoolItem `gorm:"foreignKey:PoolItemID;references:ItemID"`
}
