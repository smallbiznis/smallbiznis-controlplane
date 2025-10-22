package campaign

import (
	"context"
	"time"

	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/pkg/sequence"

	campaignv1 "github.com/smallbiznis/go-genproto/smallbiznis/campaign/v1"

	"github.com/bwmarrin/snowflake"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

// ========================================================
// Service Definition
// ========================================================

type Service struct {
	campaignv1.UnimplementedCampaignServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db   *gorm.DB
	node *snowflake.Node
	seq  sequence.Generator

	campaign repository.Repository[Campaign]
}

type ServiceParams struct {
	fx.In

	DB   *gorm.DB
	Node *snowflake.Node
	Seq  sequence.Generator
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:       p.DB,
		node:     p.Node,
		seq:      p.Seq,
		campaign: repository.ProvideStore[Campaign](p.DB),
	}
}

// ========================================================
// gRPC Implementation
// ========================================================

func (s *Service) CreateCampaign(ctx context.Context, req *campaignv1.CreateCampaignRequest) (*campaignv1.Campaign, error) {
	id := s.node.Generate().String()
	campaignCode, err := s.seq.NextCampaignCode(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	c := Campaign{
		CampaignID:      id,
		TenantID:        req.TenantId,
		Code:            campaignCode,
		Name:            req.Name,
		Description:     req.Description,
		RewardType:      RewardType(req.RewardType.String()),
		EligibilityMode: EligibilityMode(req.EligibilityMode.String()),
		RuleID:          "",
		Status:          CampaignStatusDraft,
	}

	if req.StartAt != nil {
		start := req.StartAt.AsTime()
		c.StartAt = &start
	}
	if req.EndAt != nil {
		end := req.EndAt.AsTime()
		c.EndAt = &end
	}

	if req.RewardValue != nil {
		jsonData, _ := req.RewardValue.MarshalJSON()
		c.RewardValue = jsonData
	}
	if req.Metadata != nil {
		jsonData, _ := req.Metadata.MarshalJSON()
		c.Metadata = jsonData
	}

	if err := s.db.WithContext(ctx).Create(&c).Error; err != nil {
		zap.L().Error("failed to create campaign", zap.Error(err))
		return nil, err
	}

	return c.ToProto()
}

// ========================================================

func (s *Service) UpdateCampaign(ctx context.Context, req *campaignv1.UpdateCampaignRequest) (*campaignv1.Campaign, error) {
	var c Campaign
	if err := s.db.WithContext(ctx).
		Where("campaign_id = ? AND tenant_id = ?", req.CampaignId, req.TenantId).
		First(&c).Error; err != nil {
		return nil, err
	}

	if req.Name != "" {
		c.Name = req.Name
	}

	if req.Description != "" {
		c.Description = req.Description
	}

	if req.DslExpression != "" {
		c.RuleID = ""
	}

	if req.RewardType != campaignv1.CampaignRewardType_CAMPAIGN_REWARD_TYPE_UNSPECIFIED {
		c.RewardType = RewardType(req.RewardType.String())
	}

	if req.EligibilityMode != campaignv1.CampaignEligibilityMode_CAMPAIGN_ELIGIBILITY_MODE_UNSPECIFIED {
		c.EligibilityMode = EligibilityMode(req.EligibilityMode.String())
	}

	if req.Status != campaignv1.CampaignStatus_CAMPAIGN_STATUS_UNSPECIFIED {
		c.Status = CampaignStatus(req.Status.String())
	}

	if req.StartAt != nil {
		start := req.StartAt.AsTime()
		c.StartAt = &start
	}

	if req.EndAt != nil {
		end := req.EndAt.AsTime()
		c.EndAt = &end
	}

	if req.RewardValue != nil {
		jsonData, _ := req.RewardValue.MarshalJSON()
		c.RewardValue = jsonData
	}

	if req.Metadata != nil {
		jsonData, _ := req.Metadata.MarshalJSON()
		c.Metadata = jsonData
	}

	if err := s.db.WithContext(ctx).
		Model(&Campaign{}).
		Where("campaign_id = ? AND tenant_id = ?", c.CampaignID, c.TenantID).
		Updates(&c).Error; err != nil {
		zap.L().Error("failed to update campaign", zap.Error(err))
		return nil, err
	}

	return c.ToProto()
}

// ========================================================

func (s *Service) GetCampaign(ctx context.Context, req *campaignv1.GetCampaignRequest) (*campaignv1.Campaign, error) {
	var c Campaign
	if err := s.db.WithContext(ctx).
		Where("campaign_id = ? AND tenant_id = ?", req.CampaignId, req.TenantId).
		First(&c).Error; err != nil {
		return nil, err
	}
	return c.ToProto()
}

// ========================================================

func (s *Service) ListCampaigns(ctx context.Context, req *campaignv1.ListCampaignsRequest) (*campaignv1.ListCampaignsResponse, error) {
	var campaigns []Campaign
	q := s.db.WithContext(ctx).
		Where("tenant_id = ?", req.TenantId)

	if req.OnlyActive {
		q = q.Where("status = ?", CampaignStatusActive)
	}

	if err := q.Order("created_at DESC").Find(&campaigns).Error; err != nil {
		return nil, err
	}

	resp := &campaignv1.ListCampaignsResponse{}
	for _, c := range campaigns {
		protoC, _ := c.ToProto()
		resp.Campaigns = append(resp.Campaigns, protoC)
	}

	return resp, nil
}

// ========================================================

func (s *Service) DeleteCampaign(ctx context.Context, req *campaignv1.DeleteCampaignRequest) (*structpb.Struct, error) {
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND campaign_id = ?", req.TenantId, req.CampaignId).
		Delete(&Campaign{}).Error; err != nil {
		return nil, err
	}
	return structpb.NewStruct(map[string]interface{}{"deleted": true})
}

// ========================================================

func (s *Service) CloneCampaign(ctx context.Context, req *campaignv1.CloneCampaignRequest) (*campaignv1.Campaign, error) {
	var original Campaign
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND campaign_id = ?", req.TenantId, req.CampaignId).
		First(&original).Error; err != nil {
		return nil, err
	}

	cloned := original
	cloned.CampaignID = s.node.Generate().String()
	cloned.Name = req.NewName
	cloned.Status = CampaignStatusDraft
	cloned.CreatedAt = time.Now()
	cloned.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(&cloned).Error; err != nil {
		zap.L().Error("failed to clone campaign", zap.Error(err))
		return nil, err
	}

	return cloned.ToProto()
}

// ========================================================

func (s *Service) GetActiveCampaigns(ctx context.Context, req *campaignv1.GetActiveCampaignsRequest) (*campaignv1.GetActiveCampaignsResponse, error) {
	var active []Campaign
	now := time.Now()

	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", req.TenantId).
		Where("status = ?", CampaignStatusActive).
		Where("start_at <= ? AND (end_at IS NULL OR end_at >= ?)", now, now).
		Find(&active).Error; err != nil {
		return nil, err
	}

	resp := &campaignv1.GetActiveCampaignsResponse{}
	for _, c := range active {
		protoC, _ := c.ToProto()
		resp.Campaigns = append(resp.Campaigns, protoC)
	}
	return resp, nil
}
