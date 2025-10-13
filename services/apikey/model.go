package apikey

import (
	"time"

	"github.com/lib/pq"
)

type APIKeyType string

const (
	APIKeyTypeServer      APIKeyType = "server"
	APIKeyTypePublishable APIKeyType = "publishable"
	APIKeyTypeWebhook     APIKeyType = "webhook"
)

type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "active"
	APIKeyStatusRevoked APIKeyStatus = "revoked"
	APIKeyStatusExpired APIKeyStatus = "expired"
)

type APIKey struct {
	ID         string         `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   string         `gorm:"column:tenant_id;not null;index"`
	KeyID      string         `gorm:"column:key_id;uniqueIndex;not null"` // e.g. sbsk_live_xxx
	KeyType    APIKeyType     `gorm:"column:key_type;type:api_key_type;not null"`
	SecretHash string         `gorm:"column:secret_hash;not null"`        // argon2/bcrypt hash (BUKAN plaintext)
	Scopes     pq.StringArray `gorm:"column:scopes;type:text[];not null"` // e.g. {'rules.evaluate','loyalty.write'}
	Status     string         `gorm:"column:status;default:'active';not null"`
	CreatedBy  *string        `gorm:"column:created_by"` // optional
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime"`
	ExpiresAt  *time.Time     `gorm:"column:expires_at"` // optional
}
