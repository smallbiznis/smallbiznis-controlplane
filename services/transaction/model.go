package transaction

import (
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	commonv1 "github.com/smallbiznis/go-genproto/smallbiznis/common"
	transactionv1 "github.com/smallbiznis/go-genproto/smallbiznis/transaction/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

// Transaction represents financial transaction per tenant
type Transaction struct {
	TransactionID   snowflake.ID   `gorm:"column:transaction_id;primaryKey;autoIncrement:false"`
	TransactionCode string         `gorm:"column:transaction_code;uniqueIndex"`
	TenantID        snowflake.ID   `gorm:"column:tenant_id;index;not null"`
	UserID          snowflake.ID   `gorm:"column:user_id;index;not null"`
	OrderID         string         `gorm:"column:order_id;index"`
	Amount          float64        `gorm:"column:amount;not null"`
	CurrencyCode    string         `gorm:"column:currency_code;default:'IDR'"`
	Status          string         `gorm:"column:status;default:'pending'"` // pending, success, failed, cancelled, refunded
	PaymentMethod   string         `gorm:"column:payment_method"`
	Channel         string         `gorm:"column:channel"`
	Metadata        datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

// ToProto converts Transaction to gRPC proto message
func (t *Transaction) ToProto() *transactionv1.Transaction {
	meta := &structpb.Struct{}
	if len(t.Metadata) > 0 {
		_ = meta.UnmarshalJSON(t.Metadata)
	}

	return &transactionv1.Transaction{
		TransactionId: t.TransactionID.String(),
		TenantId:      t.TenantID.String(),
		UserId:        t.UserID.String(),
		OrderId:       t.OrderID,
		Amount: &commonv1.Money{
			CurrencyCode: t.CurrencyCode,
			Amount:       int64(t.Amount * 100), // convert float → minor unit
		},
		Status:        mapStatus(t.Status),
		PaymentMethod: mapPayment(t.PaymentMethod),
		Channel:       mapChannel(t.Channel),
		Metadata:      meta,
		CreatedAt:     timestamppb.New(t.CreatedAt),
		UpdatedAt:     timestamppb.New(t.UpdatedAt),
	}
}

type TransactionItem struct {
	ItemID        snowflake.ID   `gorm:"column:item_id;primaryKey;autoIncrement"`
	TransactionID snowflake.ID   `gorm:"column:transaction_id;index;not null"`
	TenantID      snowflake.ID   `gorm:"column:tenant_id;index;not null"`
	SKU           string         `gorm:"column:sku;not null"`
	Name          string         `gorm:"column:name;not null"`
	Category      string         `gorm:"column:category"`
	Quantity      int            `gorm:"column:quantity;not null;default:1"`
	Price         float64        `gorm:"column:price;not null"`
	Total         float64        `gorm:"column:total;->"` // generated column (read-only)
	Currency      string         `gorm:"column:currency;default:'IDR'"`
	Metadata      datatypes.JSON `gorm:"column:metadata;type:jsonb"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (i *TransactionItem) ToProto() *transactionv1.TransactionItem {
	meta := &structpb.Struct{}
	if len(i.Metadata) > 0 {
		_ = meta.UnmarshalJSON(i.Metadata)
	}

	return &transactionv1.TransactionItem{
		ItemId:        fmt.Sprintf("%d", i.ItemID),
		TransactionId: fmt.Sprintf("%d", i.TransactionID),
		TenantId:      fmt.Sprintf("%d", i.TenantID),
		Sku:           i.SKU,
		Name:          i.Name,
		Category:      i.Category,
		Qty:           int32(i.Quantity),
		Price: &commonv1.Money{
			CurrencyCode: i.Currency,
			Amount:       int64(i.Price * 100),
		},
		Total: &commonv1.Money{
			CurrencyCode: i.Currency,
			Amount:       int64(i.Total * 100),
		},
		Metadata:  meta,
		CreatedAt: timestamppb.New(i.CreatedAt),
		UpdatedAt: timestamppb.New(i.UpdatedAt),
	}
}
