package loyalty

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"gorm.io/datatypes"
)

type TransactionType string

var (
	Earning    TransactionType = "earning"
	Cashback   TransactionType = "cashback"
	Redemption TransactionType = "redemption"
	Adjustment TransactionType = "adjustment"
	Expire     TransactionType = "expire"
)

func (t TransactionType) String() string {
	switch t {
	case Earning, Cashback, Redemption, Adjustment, Expire:
		return string(t)
	default:
		return ""
	}
}

type PointTransaction struct {
	ID          snowflake.ID    `gorm:"column:id;primaryKey;type:char(26)"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
	TenantID    string          `gorm:"column:tenant_id;index;not null"`
	MemberID    string          `gorm:"column:member_id;index;not null"`
	ReferenceID string          `gorm:"column:reference_id;index;not null"`
	Type        TransactionType `gorm:"column:type;type:varchar(20);not null"` // "earn" | "redeem" | "adjust" | "expire"
	RuleID      string          `gorm:"column:rule_id;index"`
	RewardID    string          `gorm:"column:reward_id;index"`
	RewardType  string          `gorm:"column:reward_type;type:varchar(30)"`
	PointDelta  int64           `gorm:"column:point_delta;not null"` // +ve for earn, -ve for redeem
	Status      string          `gorm:"column:status;type:varchar(20);default:'pending'"`
	Description string          `gorm:"column:description;type:text"`
	Metadata    datatypes.JSON  `gorm:"column:metadata;type:jsonb"`
	EventTime   time.Time       `gorm:"column:event_time;autoCreateTime"`
	ProcessedAt *time.Time      `gorm:"column:processed_at"`
	ExpireDate  *time.Time      `gorm:"column:expire_date;index"`
}
