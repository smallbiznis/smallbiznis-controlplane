package task

import (
	"time"

	"gorm.io/datatypes"
)

type Task struct {
	ID          string    `gorm:"column:id;primaryKey;type:char(26)"`
	Name        string    `gorm:"column:name;uniqueIndex;type:varchar(100);not null"`
	Description string    `gorm:"column:description;type:text"`
	Schedule    string    `gorm:"column:schedule;type:varchar(50)"` // cron format (optional)
	IsActive    bool      `gorm:"column:is_active;default:true"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	Jobs        []Job     `gorm:"foreignKey:TaskID"`
}

// Job is an execution record for a task (per tenant)
type Job struct {
	ID          string         `gorm:"column:id;primaryKey;type:char(26)"`
	TaskID      string         `gorm:"column:task_id;index;not null"`
	TenantID    string         `gorm:"column:tenant_id;index;not null"`
	Status      string         `gorm:"column:status;type:varchar(20);default:'pending'"` // pending|running|success|failed
	ErrorMsg    string         `gorm:"column:error_msg;type:text"`
	StartedAt   *time.Time     `gorm:"column:started_at"`
	CompletedAt *time.Time     `gorm:"column:completed_at"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	Metadata    datatypes.JSON `gorm:"column:metadata"`
}
