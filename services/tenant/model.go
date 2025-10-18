package tenant

import (
	"time"

	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TenantType string

var (
	Platform TenantType = "platform"
	Personal TenantType = "personal"
	Company  TenantType = "company"
)

func (t TenantType) String() string {
	switch t {
	case Platform, Personal, Company:
		return string(t)
	default:
		return ""
	}
}

type TenantStatus string

var (
	Pending      TenantStatus = "pending"
	Provisioning TenantStatus = "provisioning"
	Active       TenantStatus = "active"
	Suspended    TenantStatus = "suspended"
	Archived     TenantStatus = "archived"
)

func (t TenantStatus) String() string {
	switch t {
	case Pending, Provisioning, Active, Suspended, Archived:
		return string(t)
	default:
		return ""
	}
}

type Tenant struct {
	ID          string       `gorm:"column:id;primaryKey"`
	CreatedAt   time.Time    `gorm:"column:created_at"`
	UpdatedAt   time.Time    `gorm:"column:updated_at"`
	Type        TenantType   `gorm:"column:type"`
	Name        string       `gorm:"column:name"`
	Code        string       `gorm:"column:code"`
	Slug        string       `gorm:"column:slug"`
	CountryCode string       `gorm:"column:country_code"`
	Timezone    string       `gorm:"column:timezone"`
	IsDefault   bool         `gorm:"column:is_default;"`
	Status      TenantStatus `gorm:"column:status"`
}

func (m *Tenant) ToProto() *tenantv1.Tenant {
	return &tenantv1.Tenant{
		TenantId:    m.ID,
		Type:        string(m.Type),
		Code:        m.Code,
		Name:        m.Name,
		Slug:        m.Slug,
		CountryCode: m.CountryCode,
		Timezone:    m.Timezone,
		Status:      string(m.Status),
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
	}
}
