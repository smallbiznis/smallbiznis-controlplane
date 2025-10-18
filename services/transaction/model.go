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

type PaymentMethod string

var (
	CASH     PaymentMethod = "CASH"
	CARD     PaymentMethod = "CARD"
	QRIS     PaymentMethod = "QRIS"
	TRANSFER PaymentMethod = "TRANSFER"
	WALLET   PaymentMethod = "WALLET"
)

func (m PaymentMethod) String() string {
	switch m {
	case CASH, CARD, QRIS, TRANSFER, WALLET:
		return string(m)
	default:
		return ""
	}
}

type Channel string

var (
	ONLINE Channel = "ONLINE"
	POS    Channel = "POS"
	API    Channel = "API"
)

func (m Channel) String() string {
	switch m {
	case ONLINE, POS, API:
		return string(m)
	default:
		return ""
	}
}

type Status string

var (
	PENDING   Status = "PENDING"
	SUCCESS   Status = "SUCCESS"
	FAILED    Status = "FAILED"
	CANCELLED Status = "CANCALLED"
	REFUNDED  Status = "REFUNDED"
)

func (m Status) String() string {
	switch m {
	case PENDING, SUCCESS, FAILED, CANCELLED, REFUNDED:
		return string(m)
	default:
		return ""
	}
}

// Transaction represents financial transaction per tenant
type Transaction struct {
	ID            string            `gorm:"column:id;primaryKey;autoIncrement:false"`
	TransactionID string            `gorm:"column:transaction_id;uniqueIndex"`
	TenantID      string            `gorm:"column:tenant_id;index;not null"`
	UserID        string            `gorm:"column:user_id;index;not null"`
	OrderID       string            `gorm:"column:order_id;index"`
	Amount        float64           `gorm:"column:amount;not null"`
	CurrencyCode  string            `gorm:"column:currency_code;default:'IDR'"`
	Status        Status            `gorm:"column:status;default:'pending'"` // pending, success, failed, cancelled, refunded
	PaymentMethod PaymentMethod     `gorm:"column:payment_method"`
	Channel       Channel           `gorm:"column:channel"`
	Metadata      datatypes.JSON    `gorm:"column:metadata;type:jsonb"`
	CreatedAt     time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time         `gorm:"column:updated_at;autoUpdateTime"`
	ItemLines     []TransactionItem `gorm:"foreignKey:TransactionID"`
}

// ToProto converts Transaction to gRPC proto message
func (t *Transaction) ToProto() *transactionv1.Transaction {
	meta := &structpb.Struct{}
	if len(t.Metadata) > 0 {
		_ = meta.UnmarshalJSON(t.Metadata)
	}

	return &transactionv1.Transaction{
		Id:            t.ID,
		TenantId:      t.TenantID,
		UserId:        t.UserID,
		TransactionId: t.TransactionID,
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
	TransactionID string         `gorm:"column:transaction_id;index;not null"`
	TenantID      string         `gorm:"column:tenant_id;index;not null"`
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
		TransactionId: i.TransactionID,
		TenantId:      i.TenantID,
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
