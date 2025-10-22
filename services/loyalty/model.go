package loyalty

import (
	"time"

	"gorm.io/datatypes"
)

type Status string

var (
	Pending Status = "PENDING"
	Success Status = "SUCCESS"
	Failed  Status = "FAILED"
)

func (s Status) String() string {
	switch s {
	case Pending, Success, Failed:
		return string(s)
	default:
		return ""
	}
}

type TransactionType string

var (
	Earning    TransactionType = "EARNING"
	Cashback   TransactionType = "CASHBACK"
	Redemption TransactionType = "REDEMPTION"
	Adjustment TransactionType = "ADJUSTMENT"
	Expire     TransactionType = "EXPIRE"
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
	ID           string          `gorm:"column:id;primaryKey"`
	TenantID     string          `gorm:"column:tenant_id;index;not null"`
	UserID       string          `gorm:"column:user_id;index;not null"`
	ReferenceID  string          `gorm:"column:reference_id;index;not null"`
	Type         TransactionType `gorm:"column:type;type:varchar(20);not null"`
	RuleID       string          `gorm:"column:rule_id"`
	CampaignID   string          `gorm:"column:campaign_id"`
	RewardID     string          `gorm:"column:reward_id"`
	RewardType   string          `gorm:"column:reward_type"`
	RewardName   string          `gorm:"column:reward_name"`
	PointDelta   int64           `gorm:"column:point_delta"`
	BalanceAfter int64           `gorm:"column:balance_after"`
	Status       Status          `gorm:"column:status;default:'pending'"`
	Description  string          `gorm:"column:description"`
	Metadata     datatypes.JSON  `gorm:"column:metadata;type:jsonb"`
	EventTime    time.Time       `gorm:"column:event_time"`
	ProcessedAt  *time.Time      `gorm:"column:processed_at"`
	ExpireDate   *time.Time      `gorm:"column:expire_date"`
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}
