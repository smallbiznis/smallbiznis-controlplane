package license

import "time"

type License struct {
	ID         string     `gorm:"column:id;primaryKey"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at"`
	TenantID   string     `gorm:"column:tenant_id"`
	LicenseKey string     `gorm:"column:license_key"`
	ValidFrom  time.Time  `gorm:"column:valid_from"`
	ValidUntil time.Time  `gorm:"column:valid_until"`
	Status     string     `gorm:"column:status"`
	IssuedAt   time.Time  `gorm:"column:issued_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
}
