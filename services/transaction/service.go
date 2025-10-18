package transaction

import (
	"context"
	"fmt"
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
	"smallbiznis-controlplane/pkg/sequence"
)

type Service struct {
	transactionv1.UnimplementedTransactionServiceServer

	db           *gorm.DB
	seq          sequence.Generator
	node         *snowflake.Node
	transactions repository.Repository[Transaction]
}

type Params struct {
	fx.In
	DB   *gorm.DB
	Seq  sequence.Generator
	Node *snowflake.Node
}

func NewService(p Params) *Service {
	return &Service{
		db:           p.DB,
		seq:          p.Seq,
		node:         p.Node,
		transactions: repository.ProvideStore[Transaction](p.DB),
	}
}

// ======================================================
// RPC IMPLEMENTATION
// ======================================================

// CreateTransaction implements the proto RPC
func (s *Service) CreateTransaction(ctx context.Context, req *transactionv1.CreateTransactionRequest) (*transactionv1.CreateTransactionResponse, error) {
	primaryKey := s.node.Generate().String()
	transactionID, _ := s.seq.NextTransactionCode(ctx, "T002")
	tx := &Transaction{
		ID:            primaryKey,
		TransactionID: transactionID,
		TenantID:      req.GetTenantId(),
		UserID:        req.GetUserId(),
		OrderID:       req.GetOrderId(),
		Amount:        float64(req.Amount.Amount),
		CurrencyCode:  req.Amount.CurrencyCode,
		Status:        PENDING,
		PaymentMethod: PaymentMethod(req.PaymentMethod.String()),
		Channel:       API,
		Metadata:      structToJSON(req.Metadata),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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
		Id:            tx.ID,
		TransactionId: tx.TransactionID,
		TenantId:      tx.TenantID,
		UserId:        tx.UserID,
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

func mapStatus(status Status) transactionv1.TransactionStatus {
	switch status {
	case SUCCESS:
		return transactionv1.TransactionStatus_SUCCESS
	case FAILED:
		return transactionv1.TransactionStatus_FAILED
	case CANCELLED:
		return transactionv1.TransactionStatus_CANCELLED
	case REFUNDED:
		return transactionv1.TransactionStatus_REFUNDED
	default:
		return transactionv1.TransactionStatus_PENDING
	}
}

func mapPayment(method PaymentMethod) transactionv1.PaymentMethod {
	switch method {
	case CASH:
		return transactionv1.PaymentMethod_CASH
	case CARD:
		return transactionv1.PaymentMethod_CARD
	case QRIS:
		return transactionv1.PaymentMethod_QRIS
	case TRANSFER:
		return transactionv1.PaymentMethod_TRANSFER
	case WALLET:
		return transactionv1.PaymentMethod_WALLET
	default:
		return transactionv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

func mapChannel(ch Channel) transactionv1.ChannelType {
	switch ch {
	case ONLINE:
		return transactionv1.ChannelType_ONLINE
	case POS:
		return transactionv1.ChannelType_POS
	case API:
		return transactionv1.ChannelType_API
	default:
		return transactionv1.ChannelType_CHANNEL_TYPE_UNSPECIFIED
	}
}
