package transaction

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/go-genproto/smallbiznis/common"
	transactionv1 "github.com/smallbiznis/go-genproto/smallbiznis/transaction/v1"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"smallbiznis-controlplane/pkg/repository"
)

type Service struct {
	transactionv1.UnimplementedTransactionServiceServer

	db           *gorm.DB
	node         *snowflake.Node
	transactions repository.Repository[Transaction]
}

type Params struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
}

func NewService(p Params) *Service {
	return &Service{
		db:           p.DB,
		node:         p.Node,
		transactions: repository.ProvideStore[Transaction](p.DB),
	}
}

// ======================================================
// RPC IMPLEMENTATION
// ======================================================

// CreateTransaction implements the proto RPC
func (s *Service) CreateTransaction(ctx context.Context, req *transactionv1.CreateTransactionRequest) (*transactionv1.CreateTransactionResponse, error) {
	tx := &Transaction{
		TransactionID:   s.node.Generate(),
		TransactionCode: fmt.Sprintf("TXN-%s-%d", time.Now().Format("20060102"), rand.Intn(99999)),
		TenantID:        parseID(req.TenantId),
		UserID:          parseID(req.UserId),
		OrderID:         req.OrderId,
		Amount:          float64(req.Amount.Amount),
		CurrencyCode:    req.Amount.CurrencyCode,
		Status:          "pending",
		PaymentMethod:   req.PaymentMethod.String(),
		Channel:         "api",
		Metadata:        structToJSON(req.Metadata),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.transactions.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return &transactionv1.CreateTransactionResponse{
		Transaction: toProto(tx),
	}, nil
}

// GetTransaction implements proto RPC
func (s *Service) GetTransaction(ctx context.Context, req *transactionv1.GetTransactionRequest) (*transactionv1.GetTransactionResponse, error) {
	var tx Transaction
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND transaction_id = ?", parseID(req.TenantId), parseID(req.TransactionId)).
		First(&tx).Error; err != nil {
		return nil, err
	}
	return &transactionv1.GetTransactionResponse{
		Transaction: toProto(&tx),
	}, nil
}

// ListTransactions implements proto RPC
func (s *Service) ListTransactions(ctx context.Context, req *transactionv1.ListTransactionsRequest) (*transactionv1.ListTransactionsResponse, error) {
	var results []Transaction
	query := s.db.WithContext(ctx).
		Where("tenant_id = ?", parseID(req.TenantId)).
		Order("created_at DESC").
		Limit(int(req.Limit))

	if req.FilterStatus != transactionv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		query = query.Where("status = ?", req.FilterStatus.String())
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	out := make([]*transactionv1.Transaction, 0, len(results))
	for _, r := range results {
		out = append(out, toProto(&r))
	}

	return &transactionv1.ListTransactionsResponse{
		Transactions: out,
		NextCursor:   "",
	}, nil
}

// UpdateTransactionStatus implements proto RPC
func (s *Service) UpdateTransactionStatus(ctx context.Context, req *transactionv1.UpdateTransactionStatusRequest) (*transactionv1.UpdateTransactionStatusResponse, error) {
	if err := s.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("tenant_id = ? AND transaction_id = ?", parseID(req.TenantId), parseID(req.TransactionId)).
		Update("status", req.Status.String()).
		Error; err != nil {
		return nil, err
	}

	var tx Transaction
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND transaction_id = ?", parseID(req.TenantId), parseID(req.TransactionId)).
		First(&tx).Error; err != nil {
		return nil, err
	}

	return &transactionv1.UpdateTransactionStatusResponse{
		Transaction: toProto(&tx),
	}, nil
}

// ======================================================
// Helper Functions
// ======================================================

func parseID(s string) snowflake.ID {
	id, _ := snowflake.ParseString(s)
	return id
}

func structToJSON(st *structpb.Struct) datatypes.JSON {
	if st == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, _ := st.MarshalJSON()
	return datatypes.JSON(b)
}

func toProto(tx *Transaction) *transactionv1.Transaction {
	md := &structpb.Struct{}
	_ = md.UnmarshalJSON([]byte(tx.Metadata))
	return &transactionv1.Transaction{
		TransactionId: fmt.Sprintf("%d", tx.TransactionID),
		TenantId:      fmt.Sprintf("%d", tx.TenantID),
		UserId:        fmt.Sprintf("%d", tx.UserID),
		OrderId:       tx.OrderID,
		Amount: &common.Money{
			CurrencyCode: tx.CurrencyCode,
			Amount:       int64(tx.Amount),
		},
		Status:        mapStatus(tx.Status),
		PaymentMethod: mapPayment(tx.PaymentMethod),
		Channel:       mapChannel(tx.Channel),
		Metadata:      md,
		CreatedAt:     timestamppb.New(tx.CreatedAt),
		UpdatedAt:     timestamppb.New(tx.UpdatedAt),
	}
}

func mapStatus(status string) transactionv1.TransactionStatus {
	switch status {
	case "success":
		return transactionv1.TransactionStatus_TRANSACTION_STATUS_SUCCESS
	case "failed":
		return transactionv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	case "cancelled":
		return transactionv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED
	case "refunded":
		return transactionv1.TransactionStatus_TRANSACTION_STATUS_REFUNDED
	default:
		return transactionv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	}
}

func mapPayment(method string) transactionv1.PaymentMethod {
	switch method {
	case "cash":
		return transactionv1.PaymentMethod_PAYMENT_METHOD_CASH
	case "card":
		return transactionv1.PaymentMethod_PAYMENT_METHOD_CARD
	case "qris":
		return transactionv1.PaymentMethod_PAYMENT_METHOD_QRIS
	case "transfer":
		return transactionv1.PaymentMethod_PAYMENT_METHOD_TRANSFER
	case "wallet":
		return transactionv1.PaymentMethod_PAYMENT_METHOD_WALLET
	default:
		return transactionv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

func mapChannel(ch string) transactionv1.ChannelType {
	switch ch {
	case "online":
		return transactionv1.ChannelType_CHANNEL_TYPE_ONLINE
	case "pos":
		return transactionv1.ChannelType_CHANNEL_TYPE_POS
	case "api":
		return transactionv1.ChannelType_CHANNEL_TYPE_API
	default:
		return transactionv1.ChannelType_CHANNEL_TYPE_UNSPECIFIED
	}
}
