package voucher

import (
	"context"
	"smallbiznis-controlplane/pkg/repository"

	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"gorm.io/gorm"
)

// VoucherIssuanceRepository handles voucher issuance operations
type VoucherIssuanceRepository struct {
	repository.Repository[VoucherIssuance]
	db   *gorm.DB
	repo repository.Repository[VoucherIssuance]
}

// VoucherIssuanceRepositoryParams defines the parameters for creating a VoucherIssuanceRepository
type VoucherIssuanceRepositoryParams struct {
	DB *gorm.DB
}

// NewRepository creates a new instance of VoucherIssuanceRepository
func NewRepository(p VoucherIssuanceRepositoryParams) *VoucherIssuanceRepository {
	return &VoucherIssuanceRepository{
		db:   p.DB,
		repo: repository.ProvideStore[VoucherIssuance](p.DB),
	}
}

// IssueVoucher decrements stock and creates issuance atomically
func (r *VoucherIssuanceRepository) IssueVoucher(ctx context.Context, tenantID, userID, code string) (*VoucherIssuance, error) {
	var voucher Voucher
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND code = ? AND is_active = TRUE", tenantID, code).
		First(&voucher).Error; err != nil {
		return nil, err
	}

	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	res := tx.Model(&Voucher{}).
		Where("voucher_id = ? AND remaining_stock > 0", voucher.VoucherID).
		Update("remaining_stock", gorm.Expr("remaining_stock - 1"))
	if res.Error != nil || res.RowsAffected == 0 {
		tx.Rollback()
		return nil, gorm.ErrRecordNotFound
	}

	issuance := &VoucherIssuance{
		TenantID:  tenantID,
		VoucherID: voucher.VoucherID,
		UserID:    userID,
		Status:    voucherv1.IssuanceStatus_ISSUED,
	}
	if err := tx.Create(&issuance).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return issuance, tx.Commit().Error
}
