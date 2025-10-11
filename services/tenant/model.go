package tenant

import (
	"time"

	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TenantType string

var (
	Personal TenantType = "personal"
	Company  TenantType = "company"
)

func (t TenantType) String() string {
	switch t {
	case Personal, Company:
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
	Slug        string       `gorm:"column:slug"`
	CountryCode string       `gorm:"column:country_code"`
	Timezone    string       `gorm:"column:timezone"`
	Status      TenantStatus `gorm:"column:status"`
}

func (m *Tenant) ToProto() *tenantv1.Tenant {
	return &tenantv1.Tenant{
		TenantId:    m.ID,
		Name:        m.Name,
		Slug:        m.Slug,
		CountryCode: m.CountryCode,
		Timezone:    m.Timezone,
		CreatedAt:   timestamppb.New(m.CreatedAt),
		UpdatedAt:   timestamppb.New(m.UpdatedAt),
	}
}
