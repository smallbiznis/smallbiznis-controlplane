package campaign

import (
	"time"

	campaignv1 "github.com/smallbiznis/go-genproto/smallbiznis/campaign/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

// ENUM-LIKE constants (mirror from proto)
type RewardType string
type EligibilityMode string
type CampaignStatus string

const (
	RewardTypeVoucher  RewardType = "VOUCHER"
	RewardTypePoint    RewardType = "POINT"
	RewardTypeCashback RewardType = "CASHBACK"
	RewardTypeFreeItem RewardType = "FREE_ITEM"
	RewardTypeCustom   RewardType = "CUSTOM"

	EligibilityModeManual EligibilityMode = "MANUAL"
	EligibilityModeRule   EligibilityMode = "RULE"

	CampaignStatusDraft    CampaignStatus = "DRAFT"
	CampaignStatusActive   CampaignStatus = "ACTIVE"
	CampaignStatusInactive CampaignStatus = "INACTIVE"
	CampaignStatusExpired  CampaignStatus = "EXPIRED"
)

// Campaign represents a marketing or reward campaign definition
type Campaign struct {
	CampaignID      string          `gorm:"column:campaign_id;primaryKey;type:char(26)"`
	TenantID        string          `gorm:"column:tenant_id;index;not null"`
	Code            string          `gorm:"column:code"`
	Name            string          `gorm:"column:name;type:varchar(255);not null"`
	Description     string          `gorm:"column:description;type:text"`
	RewardType      RewardType      `gorm:"column:reward_type;type:varchar(50);not null;default:'VOUCHER'"`
	EligibilityMode EligibilityMode `gorm:"column:eligibility_mode;type:varchar(50);not null;default:'RULE'"`
	RuleID          string          `gorm:"column:rule_id"`
	Status          CampaignStatus  `gorm:"column:status;type:varchar(50);not null;default:'DRAFT'"`
	StartAt         *time.Time      `gorm:"column:start_at"`
	EndAt           *time.Time      `gorm:"column:end_at"`
	RewardValue     datatypes.JSON  `gorm:"column:reward_value;type:jsonb"`
	Metadata        datatypes.JSON  `gorm:"column:metadata;type:jsonb"`
	CreatedAt       time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// ========================================================
// Helper methods
// ========================================================

// IsActive checks if campaign is currently active based on time range & status.
func (c *Campaign) IsActive(now time.Time) bool {
	if c.Status != CampaignStatusActive {
		return false
	}
	if c.StartAt != nil && now.Before(*c.StartAt) {
		return false
	}
	if c.EndAt != nil && now.After(*c.EndAt) {
		return false
	}
	return true
}

// ToProto converts the model to protobuf Campaign message.
func (c *Campaign) ToProto() (*campaignv1.Campaign, error) {
	var rewardStruct, metaStruct *structpb.Struct
	var err error

	if len(c.RewardValue) > 0 {
		rewardStruct = &structpb.Struct{}
		if err = rewardStruct.UnmarshalJSON(c.RewardValue); err != nil {
			return nil, err
		}
	}

	if len(c.Metadata) > 0 {
		metaStruct = &structpb.Struct{}
		if err = metaStruct.UnmarshalJSON(c.Metadata); err != nil {
			return nil, err
		}
	}

	return &campaignv1.Campaign{
		CampaignId:              c.CampaignID,
		TenantId:                c.TenantID,
		CampaignName:            c.Name,
		CampaignCode:            c.Code,
		CampaignDescription:     c.Description,
		RuleId:                  c.RuleID,
		CampaignRewardType:      toProtoRewardType(c.RewardType),
		CampaignEligibilityMode: toProtoEligibilityMode(c.EligibilityMode),
		Status:                  toProtoStatus(c.Status),
		StartAt:                 toProtoTimestamp(c.StartAt),
		EndAt:                   toProtoTimestamp(c.EndAt),
		RewardValue:             rewardStruct,
		Metadata:                metaStruct,
		CreatedAt:               toProtoTimestamp(&c.CreatedAt),
		UpdatedAt:               toProtoTimestamp(&c.UpdatedAt),
	}, nil
}

// ========================================================
// Converters between internal enums ↔ proto enums
// ========================================================

func toProtoRewardType(rt RewardType) campaignv1.CampaignRewardType {
	switch rt {
	case RewardTypePoint:
		return campaignv1.CampaignRewardType_VOUCHER
	case RewardTypeCashback:
		return campaignv1.CampaignRewardType_CASHBACK
	case RewardTypeFreeItem:
		return campaignv1.CampaignRewardType_FREE_ITEM
	case RewardTypeCustom:
		return campaignv1.CampaignRewardType_CUSTOM
	default:
		return campaignv1.CampaignRewardType_CAMPAIGN_REWARD_TYPE_UNSPECIFIED
	}
}

func toProtoEligibilityMode(m EligibilityMode) campaignv1.CampaignEligibilityMode {
	switch m {
	case EligibilityModeManual:
		return campaignv1.CampaignEligibilityMode_MANUAL
	default:
		return campaignv1.CampaignEligibilityMode_RULE
	}
}

func toProtoStatus(s CampaignStatus) campaignv1.CampaignStatus {
	switch s {
	case CampaignStatusActive:
		return campaignv1.CampaignStatus_ACTIVE
	case CampaignStatusInactive:
		return campaignv1.CampaignStatus_INACTIVE
	case CampaignStatusExpired:
		return campaignv1.CampaignStatus_EXPIRED
	default:
		return campaignv1.CampaignStatus_DRAFT
	}
}

func toProtoTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}

	return timestamppb.New(*t)
}
