package domain

import (
	"time"

	domainv1 "github.com/smallbiznis/go-genproto/smallbiznis/domain/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DomainType string

var (
	System DomainType = "system"
	Custom DomainType = "custom"
)

func (t DomainType) String() string {
	switch t {
	case System, Custom:
		return string(t)
	default:
		return ""
	}
}

type VerificationMethodType string

var (
	DNS    VerificationMethodType = "dns"
	File   VerificationMethodType = "file"
	Manual VerificationMethodType = "manual"
)

func (t VerificationMethodType) String() string {
	switch t {
	case DNS, File, Manual:
		return string(t)
	default:
		return ""
	}
}

type CertificateStatusType string

var (
	Pending CertificateStatusType = "pending"
	Active  CertificateStatusType = "active"
	Failed  CertificateStatusType = "failed"
)

func (t CertificateStatusType) String() string {
	switch t {
	case Pending, Active, Failed:
		return string(t)
	default:
		return ""
	}
}

type Domain struct {
	ID                 string                 `gorm:"column:id;primaryKey"`
	CreatedAt          time.Time              `gorm:"column:created_at"`
	UpdatedAt          time.Time              `gorm:"column:updated_at"`
	TenantID           string                 `gorm:"column:tenant_id"`
	Type               DomainType             `gorm:"column:type"`
	Hostname           string                 `gorm:"column:hostname"`
	VerificationMethod VerificationMethodType `gorm:"column:verification_method"`
	VerificationCode   string                 `gorm:"column:verification_code"`
	CertificateStatus  CertificateStatusType  `gorm:"column:certificate_status"`
	Verified           bool                   `gorm:"column:verified"`
	VerifiedAt         *time.Time             `gorm:"column:verified_at"`
	IsPrimary          bool                   `gorm:"column:is_primary"`
}

func (m *Domain) ToProto() *domainv1.Domain {
	return &domainv1.Domain{
		DomainId:         m.ID,
		TenantId:         m.TenantID,
		Hostname:         m.Hostname,
		Verified:         m.Verified,
		VerificationCode: m.VerificationCode,
		CreatedAt:        timestamppb.New(m.CreatedAt),
	}
}
