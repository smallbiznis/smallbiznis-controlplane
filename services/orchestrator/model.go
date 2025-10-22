package orchestrator

import (
	"time"
)

type EarningEvent struct {
	ID         string    `gorm:"column:id;primaryKey"`
	TenantID   string    `gorm:"column:tenant_id;index"`
	UserID     string    `gorm:"column:user_id;index"`
	EventType  string    `gorm:"column:event_type;index"`
	Attributes []byte    `gorm:"column:attributes;type:jsonb"`
	Reference  string    `gorm:"column:reference"`
	Status     string    `gorm:"column:status;default:'pending'"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

type EarningResult struct {
	ID         string    `gorm:"column:id;primaryKey"`
	TenantID   string    `gorm:"column:tenant_id;index"`
	UserID     string    `gorm:"column:user_id;index"`
	CampaignID string    `gorm:"column:campaign_id;index"`
	RewardType string    `gorm:"column:reward_type"`
	RewardData []byte    `gorm:"column:reward_data;type:jsonb"`
	Status     string    `gorm:"column:status;default:'success'"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}
