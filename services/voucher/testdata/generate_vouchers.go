package testdata

import (
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	"go.uber.org/fx"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"smallbiznis-controlplane/services/voucher"
)

var SeedVoucher = fx.Module("seed.voucher",
	fx.Invoke(GenerateTestVoucher),
)

// GenerateTestVoucher inserts sample vouchers for testing
func GenerateTestVoucher(db *gorm.DB) error {

	now := time.Now()
	tenantID := "1977705426556293120"
	campaignID := "1978471833447436288"

	// newCampaign := voucher.VoucherCampaign{
	// 	CampaignID:     campaignID,
	// 	TenantID:       tenantID,
	// 	Name:           "Example Campaign",
	// 	Description:    "Example Description",
	// 	Type:           "discount",
	// 	TotalStock:     10,
	// 	RemainingStock: 0,
	// 	IsActive:       true,
	// 	StartAt:        &now,
	// 	EndAt:          ptrTime(now.Add(24 * time.Hour)),
	// }

	// if err := db.Model(&voucher.VoucherCampaign{}).Create(&newCampaign).Error; err != nil {
	// 	return err
	// }

	node, err := snowflake.NewNode(1)
	if err != nil {
		return fmt.Errorf("failed to init snowflake node: %w", err)
	}
	vouchers := []voucher.Voucher{
		{
			VoucherID:      node.Generate().String(),
			TenantID:       tenantID,
			CampaignID:     campaignID,
			Code:           "FOOD10",
			Name:           "Diskon 10% untuk kategori Food",
			DiscountType:   "percent",
			DiscountValue:  10,
			Currency:       "IDR",
			Stock:          100,
			RemainingStock: 100,
			IsActive:       true,
			DslExpression:  datatypes.JSON([]byte(`"order.category == \"food\" && order.amount >= 50000"`)),
			ExpiryDate:     ptrTime(now.AddDate(0, 0, 30)), // 30 hari dari sekarang
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			VoucherID:      node.Generate().String(),
			TenantID:       tenantID,
			CampaignID:     campaignID,
			Code:           "VIP25",
			Name:           "Diskon 25% khusus Tier Gold",
			DiscountType:   "percent",
			DiscountValue:  25,
			Currency:       "IDR",
			Stock:          50,
			RemainingStock: 50,
			IsActive:       true,
			DslExpression:  datatypes.JSON([]byte(`"user.tier == \"gold\" && order.amount >= 100000"`)),
			ExpiryDate:     ptrTime(now.AddDate(0, 1, 0)), // 1 bulan
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			VoucherID:      node.Generate().String(),
			TenantID:       tenantID,
			CampaignID:     campaignID,
			Code:           "CASHBACK20",
			Name:           "Cashback 20K minimal belanja 150K",
			DiscountType:   "fixed",
			DiscountValue:  20000,
			Currency:       "IDR",
			Stock:          200,
			RemainingStock: 200,
			IsActive:       true,
			DslExpression:  datatypes.JSON([]byte(`"order.amount >= 150000"`)),
			ExpiryDate:     ptrTime(now.AddDate(0, 0, 15)),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			VoucherID:      node.Generate().String(),
			TenantID:       tenantID,
			CampaignID:     campaignID,
			Code:           "EXPIREDPROMO",
			Name:           "Promo expired (untuk testing)",
			DiscountType:   "fixed",
			DiscountValue:  50000,
			Currency:       "IDR",
			Stock:          10,
			RemainingStock: 5,
			IsActive:       false,
			DslExpression:  datatypes.JSON([]byte(`"order.amount >= 100000"`)),
			ExpiryDate:     ptrTime(now.AddDate(0, -1, 0)), // expired sebulan lalu
			CreatedAt:      now.AddDate(0, -2, 0),
			UpdatedAt:      now.AddDate(0, -1, 0),
		},
	}

	// Insert if not exists
	for _, v := range vouchers {
		var count int64
		if err := db.Model(&voucher.Voucher{}).
			Where("tenant_id = ? AND code = ?", v.TenantID, v.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := db.Create(&v).Error; err != nil {
				return fmt.Errorf("failed to seed voucher %s: %w", v.Code, err)
			}
		}
	}

	fmt.Printf("✅ Seeded %d vouchers for tenant %s\n", len(vouchers), tenantID)
	return nil
}

func ptrTime(t time.Time) *time.Time { return &t }
