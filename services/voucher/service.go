package voucher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/pkg/sequence"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/status"
	campaignv1 "github.com/smallbiznis/go-genproto/smallbiznis/campaign/v1"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type Service struct {
	voucherv1.UnimplementedVoucherServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db   *gorm.DB
	node *snowflake.Node
	seq  sequence.Generator

	// voucher         repository.Repository[Voucher]
	voucherPool     repository.Repository[VoucherPool]
	voucherPoolItem repository.Repository[VoucherPoolItem]
	voucherIssue    repository.Repository[VoucherIssuance]

	campaign campaignv1.CampaignServiceClient
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
	Seq  sequence.Generator

	Campaign campaignv1.CampaignServiceClient
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:   p.DB,
		node: p.Node,
		seq:  p.Seq,

		// voucher:         repository.ProvideStore[Voucher](p.DB),
		voucherPool:     repository.ProvideStore[VoucherPool](p.DB),
		voucherPoolItem: repository.ProvideStore[VoucherPoolItem](p.DB),
		voucherIssue:    repository.ProvideStore[VoucherIssuance](p.DB),

		campaign: p.Campaign,
	}
}

// =========================================================
// CreateVoucherPool
// =========================================================
func (s *Service) CreateVoucherPool(ctx context.Context, req *voucherv1.CreateVoucherPoolRequest) (*voucherv1.VoucherPool, error) {
	if req == nil || req.TenantId == "" || req.Name == "" || req.CampaignId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id, name, campaign_id are required")
	}

	campaign, err := s.campaign.GetCampaign(ctx, &campaignv1.GetCampaignRequest{
		TenantId:   req.GetTenantId(),
		CampaignId: req.GetCampaignId(),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, status.Errorf(codes.InvalidArgument, "campaign not found")
		}
		return nil, status.Errorf(codes.Internal, "failed fetch campaign: %v", err)
	}

	// 🔒 Optional: pastikan reward_type campaign = VOUCHER
	if campaign.GetCampaignRewardType() != campaignv1.CampaignRewardType_VOUCHER {
		return nil, status.Error(codes.FailedPrecondition, "campaign reward type must be VOUCHER")
	}

	if campaign.GetRewardValue() == nil && campaign.GetVoucherReward() == nil {
		return nil, status.Error(codes.FailedPrecondition, "campaign reward value is required")
	}

	pool := &VoucherPool{
		PoolID:         s.node.Generate().String(),
		TenantID:       req.GetTenantId(),
		CampaignID:     &req.CampaignId,
		Name:           req.Name,
		TotalStock:     0,
		RemainingStock: 0,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.voucherPool.Create(ctx, pool); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create pool: %v", err)
	}

	vr := campaign.GetVoucherReward()
	if vr.GetMode() == "GENERATED" {
		if vr.GetCount() > 0 {
			if err := s.generateVoucherPoolItems(ctx, campaign, pool, vr.GetCount()); err != nil {
				zap.L().Error("failed auto generate voucher items", zap.Error(err))
				return nil, status.Errorf(codes.Internal, "failed to create pool items: %v", err)
			}
		}
	}

	return &voucherv1.VoucherPool{
		PoolId:         pool.PoolID,
		TenantId:       pool.TenantID,
		CampaignId:     req.CampaignId,
		Name:           pool.Name,
		TotalStock:     pool.TotalStock,
		RemainingStock: pool.RemainingStock,
		IsActive:       pool.IsActive,
		CreatedAt:      timestamppb.New(pool.CreatedAt),
		UpdatedAt:      timestamppb.New(pool.UpdatedAt),
	}, nil
}

// =========================================================
// ImportVoucherPoolItems
// =========================================================
func (s *Service) ImportVoucherPoolItems(ctx context.Context, req *voucherv1.ImportVoucherPoolItemsRequest) (*voucherv1.ImportVoucherPoolItemsResponse, error) {
	if req == nil || req.TenantId == "" || req.PoolId == "" || len(req.VoucherCodes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "tenant_id, pool_id, and voucher_codes are required")
	}

	keyVersion := "v1"
	key := sha256.Sum256([]byte("dummy-key-32bytes-for-demo-00000000000"))

	var items []VoucherPoolItem
	for _, code := range req.VoucherCodes {
		hash := HashVoucherCode(code)
		enc, err := EncryptVoucherCode([]byte(code), key)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encrypt failed: %v", err)
		}

		items = append(items, VoucherPoolItem{
			ItemID:          s.node.Generate().String(),
			TenantID:        req.TenantId,
			PoolID:          req.PoolId,
			VoucherCodeHash: hash,
			VoucherCodeEnc:  enc,
			KeyVersion:      keyVersion,
			IsIssued:        false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		})
	}

	if err := s.db.Create(&items).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "insert pool items failed: %v", err)
	}

	if err := s.db.Model(&VoucherPool{}).
		Where("pool_id = ?", req.PoolId).
		Update("total_stock", gorm.Expr("total_stock + ?", len(items))).
		Update("remaining_stock", gorm.Expr("remaining_stock + ?", len(items))).
		Error; err != nil {
		return nil, status.Errorf(codes.Internal, "update stock failed: %v", err)
	}

	return &voucherv1.ImportVoucherPoolItemsResponse{Imported: int32(len(items))}, nil
}

func (s *Service) generateVoucherPoolItems(ctx context.Context, cmp *campaignv1.Campaign, pool *VoucherPool, count int32) error {
	keyVersion := "v1"
	key := sha256.Sum256([]byte("dummy-key-32bytes-for-demo-00000000000"))

	var items []VoucherPoolItem
	for i := 0; i < int(count); i++ {
		code, err := s.seq.NextVoucherCode(ctx, pool.TenantID, cmp.CampaignCode)
		if err != nil {
			zap.L().Warn("failed generate voucher code", zap.Error(err))
			return err
		}
		zap.L().Debug("voucher_code", zap.String("code", code))

		hash := HashVoucherCode(code)
		enc, err := EncryptVoucherCode([]byte(code), key)
		zap.L().Debug("encrypt", zap.Error(err))

		items = append(items, VoucherPoolItem{
			ItemID:          s.node.Generate().String(),
			TenantID:        pool.TenantID,
			PoolID:          pool.PoolID,
			VoucherCodeHash: hash,
			VoucherCodeEnc:  enc,
			KeyVersion:      keyVersion,
			IsIssued:        false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		})
	}

	if err := s.db.Create(&items).Error; err != nil {
		return fmt.Errorf("failed to generate voucher items: %w", err)
	}

	return s.db.Model(&VoucherPool{}).
		Where("pool_id = ?", pool.PoolID).
		Updates(map[string]interface{}{
			"total_stock":     gorm.Expr("total_stock + ?", len(items)),
			"remaining_stock": gorm.Expr("remaining_stock + ?", len(items)),
		}).Error
}

// =========================================================
// ListVoucherPools
// =========================================================
func (s *Service) ListVoucherPools(ctx context.Context, req *voucherv1.ListVoucherPoolsRequest) (*voucherv1.ListVoucherPoolsResponse, error) {
	if req == nil || req.TenantId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id required")
	}

	var pools []VoucherPool
	query := s.db.Where("tenant_id = ?", req.TenantId)
	if req.OnlyActive {
		query = query.Where("is_active = true")
	}
	if err := query.Find(&pools).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "query failed: %v", err)
	}

	resp := &voucherv1.ListVoucherPoolsResponse{}
	for _, p := range pools {
		resp.Pools = append(resp.Pools, &voucherv1.VoucherPool{
			PoolId:         p.PoolID,
			TenantId:       p.TenantID,
			CampaignId:     valueOrEmpty(p.CampaignID),
			Name:           p.Name,
			TotalStock:     p.TotalStock,
			RemainingStock: p.RemainingStock,
			IsActive:       p.IsActive,
			CreatedAt:      timestamppb.New(p.CreatedAt),
			UpdatedAt:      timestamppb.New(p.UpdatedAt),
		})
	}
	return resp, nil
}

// =========================================================
// ListVoucherPoolItems
// =========================================================
func (s *Service) ListVoucherPoolItems(ctx context.Context, req *voucherv1.ListVoucherPoolItemsRequest) (*voucherv1.ListVoucherPoolItemsResponse, error) {
	if req == nil || req.TenantId == "" || req.PoolId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and pool_id required")
	}

	var items []VoucherPoolItem
	limit := int(req.Limit)
	if limit == 0 {
		limit = 50
	}

	if err := s.db.Where("tenant_id = ? AND pool_id = ?", req.TenantId, req.PoolId).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "query failed: %v", err)
	}

	resp := &voucherv1.ListVoucherPoolItemsResponse{}
	for _, it := range items {
		resp.Items = append(resp.Items, &voucherv1.VoucherPoolItem{
			ItemId:      it.ItemID,
			PoolId:      it.PoolID,
			VoucherCode: maskCode(it.VoucherCodeEnc), // for security, mask
			IsIssued:    it.IsIssued,
			IssuedTo:    valueOrEmpty(it.IssuedTo),
			CreatedAt:   timestamppb.New(it.CreatedAt),
			UpdatedAt:   timestamppb.New(it.UpdatedAt),
		})
	}
	return resp, nil
}

// =========================================================
// EvaluateVouchers (stub for future rule integration)
// =========================================================
func (s *Service) EvaluateVouchers(ctx context.Context, req *voucherv1.EvaluateVouchersRequest) (*voucherv1.EvaluateVouchersResponse, error) {
	// Placeholder logic: always return empty eligibility
	return &voucherv1.EvaluateVouchersResponse{Results: []*voucherv1.VoucherEligibilityResult{}}, nil
}

// =========================================================
// IssueVoucher (auto assign from pool)
// =========================================================
func (s *Service) IssueVoucher(ctx context.Context, req *voucherv1.IssueVoucherRequest) (*voucherv1.IssueVoucherResponse, error) {
	if req == nil || req.TenantId == "" || req.UserId == "" || req.CampaignId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id, user_id, campaign_id required")
	}

	// Find campaign’s pool (simplified: one pool per campaign)
	var pool VoucherPool
	if err := s.db.Where("tenant_id = ? AND campaign_id = ?", req.TenantId, req.CampaignId).First(&pool).Error; err != nil {
		return nil, status.Errorf(codes.NotFound, "no voucher pool for campaign: %v", err)
	}

	// Ambil voucher stock
	var items []VoucherPoolItem
	if err := s.db.Where("tenant_id = ? AND pool_id = ? AND is_issued = false", req.TenantId, pool.PoolID).
		Limit(int(req.Count)).
		Find(&items).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "fetch stock failed: %v", err)
	}

	if len(items) == 0 {
		return nil, status.Error(codes.ResourceExhausted, "no voucher stock available")
	}

	now := time.Now()
	var issuances []VoucherIssuance

	for _, it := range items {
		it.IsIssued = true
		it.IssuedTo = &req.UserId
		it.IssuedAt = &now
		s.db.Save(&it)

		issuance := VoucherIssuance{
			IssuanceID: s.node.Generate().String(),
			TenantID:   req.TenantId,
			PoolItemID: &it.ItemID,
			UserID:     req.UserId,
			Status:     IssuanceStatusIssued,
			IssuedAt:   now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		issuances = append(issuances, issuance)
	}

	if err := s.db.Create(&issuances).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "insert issuances failed: %v", err)
	}

	// Update pool stock
	s.db.Model(&pool).Update("remaining_stock", gorm.Expr("remaining_stock - ?", len(items)))

	resp := &voucherv1.IssueVoucherResponse{}
	for _, i := range issuances {
		resp.Issuances = append(resp.Issuances, &voucherv1.VoucherIssuance{
			IssuanceId: i.IssuanceID,
			TenantId:   i.TenantID,
			UserId:     i.UserID,
			Status:     voucherv1.IssuanceStatus_ISSUED,
			IssuedAt:   timestamppb.New(i.IssuedAt),
		})
	}
	return resp, nil
}

// =========================================================
// RedeemVoucher
// =========================================================
func (s *Service) RedeemVoucher(ctx context.Context, req *voucherv1.RedeemVoucherRequest) (*voucherv1.VoucherIssuance, error) {
	if req == nil || req.TenantId == "" || req.UserId == "" || req.VoucherCode == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id, user_id, voucher_code required")
	}

	hash := HashVoucherCode(req.VoucherCode)

	var item VoucherPoolItem
	if err := s.db.Where("tenant_id = ? AND voucher_code_hash = ?", req.TenantId, hash).First(&item).Error; err != nil {
		return nil, status.Errorf(codes.NotFound, "voucher not found: %v", err)
	}

	if item.IsIssued && item.IssuedTo != nil && *item.IssuedTo != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "voucher belongs to another user")
	}

	now := time.Now()
	item.IsIssued = true
	item.IssuedTo = &req.UserId
	item.IssuedAt = &now
	s.db.Save(&item)

	issuance := VoucherIssuance{
		IssuanceID: s.node.Generate().String(),
		TenantID:   req.TenantId,
		PoolItemID: &item.ItemID,
		UserID:     req.UserId,
		Status:     IssuanceStatusRedeemed,
		IssuedAt:   now,
		RedeemedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.db.Create(&issuance)

	return &voucherv1.VoucherIssuance{
		IssuanceId: issuance.IssuanceID,
		TenantId:   issuance.TenantID,
		UserId:     issuance.UserID,
		Status:     voucherv1.IssuanceStatus_REDEEMED,
		IssuedAt:   timestamppb.New(issuance.IssuedAt),
		RedeemedAt: timestamppb.New(now),
	}, nil
}

// =========================================================
// ListMyVouchers
// =========================================================
func (s *Service) ListMyVouchers(ctx context.Context, req *voucherv1.ListMyVouchersRequest) (*voucherv1.ListMyVouchersResponse, error) {
	if req == nil || req.TenantId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and user_id required")
	}

	var list []VoucherIssuance
	query := s.db.Where("tenant_id = ? AND user_id = ?", req.TenantId, req.UserId)
	if req.Status != voucherv1.IssuanceStatus_ISSUANCE_STATUS_UNSPECIFIED {
		query = query.Where("status = ?", req.Status)
	}

	if err := query.Find(&list).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "query failed: %v", err)
	}

	resp := &voucherv1.ListMyVouchersResponse{}
	for _, v := range list {
		issueance := &voucherv1.VoucherIssuance{
			IssuanceId: v.IssuanceID,
			TenantId:   v.TenantID,
			UserId:     v.UserID,
			Status:     voucherv1.IssuanceStatus(voucherv1.IssuanceStatus_value[string(v.Status)]),
			IssuedAt:   timestamppb.New(v.IssuedAt),
			RedeemedAt: timestamppb.New(*v.RedeemedAt),
		}

		if v.Voucher != nil {
			if v.Voucher.ExpiryDate != nil {
				// expdate := v.Voucher.ExpiryDate.Format(time.DateOnly)
				// issueance.ExpiryDate = expdate
				// issueance.VoucherCode = v.Voucher.Code
			}
		}

		resp.Vouchers = append(resp.Vouchers, issueance)
	}
	return resp, nil
}
