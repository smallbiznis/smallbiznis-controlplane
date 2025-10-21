package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/errutil"
	"smallbiznis-controlplane/pkg/repository"
	"time"

	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/bwmarrin/snowflake"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	ledgerv1.UnimplementedLedgerServiceServer
	health.UnimplementedHealthServer

	db   *gorm.DB
	node *snowflake.Node

	ledger  repository.Repository[LedgerEntry]
	balance repository.Repository[Balance]
	credit  repository.Repository[CreditPool]
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:   p.DB,
		node: p.Node,

		ledger:  repository.ProvideStore[LedgerEntry](p.DB),
		balance: repository.ProvideStore[Balance](p.DB),
		credit:  repository.ProvideStore[CreditPool](p.DB),
	}
}

func (s *Service) GetBalance(ctx context.Context, req *ledgerv1.GetBalanceRequest) (*ledgerv1.GetBalanceResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	lastEntry, err := s.balance.FindOne(ctx, &Balance{TenantID: req.GetTenantId(), MemberID: req.GetMemberId()}, option.WithSortBy(option.QuerySortBy{OrderBy: "DESC"}))
	if err != nil {
		zap.L().With(opts...).Error("failed to query FindOne entry", zap.Error(err))
		return nil, err
	}

	var lastBalance int64 = 0
	if lastEntry != nil {
		lastBalance = lastEntry.Balance
	}

	return &ledgerv1.GetBalanceResponse{
		Balance:       lastBalance,
		LastUpdatedAt: timestamppb.New(lastEntry.CreatedAt),
	}, nil
}

func (s *Service) AddEntry(ctx context.Context, req *ledgerv1.AddEntryRequest) (*ledgerv1.LedgerEntry, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	opts := []zap.Field{zap.String("trace_id", traceID), zap.String("span_id", spanID)}

	// OPTIONAL: pre-check (UX), tapi jangan dijadikan satu-satunya pengaman
	if exist, _ := s.ledger.FindOne(ctx, &LedgerEntry{
		TenantID: req.GetTenantId(), ReferenceID: req.GetReferenceId(),
	}); exist != nil {
		zap.L().With(opts...).Warn("reference_id already exists", zap.String("reference_id", req.GetReferenceId()))
		return nil, errutil.BadRequest("reference_id already exists", nil)
	}

	if err := s.processAddEntry(ctx, req); err != nil {
		// Jika err duplicate key → balikan sebagai idempotent conflict
		// if s.db.IsUniqueViolation(err) {
		// 	return nil, errutil.BadRequest("reference_id already exists", nil)
		// }
		zap.L().With(opts...).Error("failed process add entry", zap.Error(err))
		return nil, err
	}

	entry, err := s.ledger.FindOne(ctx, &LedgerEntry{ReferenceID: req.ReferenceId})
	if err != nil {
		return nil, err
	}

	et, _ := ledgerv1.EntryType_value[entry.Type] // FIXME: idealnya simpan enum int
	return &ledgerv1.LedgerEntry{
		Id: entry.ID, TenantId: entry.TenantID, MemberId: entry.MemberID,
		Type: ledgerv1.EntryType(et), Amount: entry.Amount,
		TransactionId: entry.TransactionID, ReferenceId: entry.ReferenceID, Description: entry.Description,
	}, nil
}

func (s *Service) ReverEntry(ctx context.Context, req *ledgerv1.RevertEntryRequest) (*ledgerv1.LedgerEntry, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {

		tx = tx.Scopes(option.LockingUpdate)

		originalEntry, err := s.ledger.WithTrx(tx).FindOne(ctx, &LedgerEntry{
			ID: req.EntryId,
		})
		if err != nil {
			zap.L().With(opts...).Error("failed to query FindOne entry", zap.Error(err))
			return err
		}

		return s.processRevertCredit(ctx, tx, originalEntry)

	}); err != nil {
		return nil, err
	}

	current, err := s.ledger.FindOne(ctx, &LedgerEntry{ID: req.EntryId})
	if err != nil {
		return nil, err
	}

	return &ledgerv1.LedgerEntry{
		Id:            current.ID,
		TenantId:      current.TenantID,
		MemberId:      current.MemberID,
		Type:          ledgerv1.EntryType(ledgerv1.EntryType_value[current.Type]),
		Amount:        current.Amount,
		TransactionId: current.TransactionID,
		ReferenceId:   current.ReferenceID,
		Description:   current.Description,
	}, nil
}

func (s *Service) getLastEntry(tx *gorm.DB, ctx context.Context, req *LedgerEntry) (*LedgerEntry, error) {
	lastEntry, err := s.ledger.WithTrx(tx).FindOne(ctx, &LedgerEntry{
		TenantID: req.TenantID,
		MemberID: req.MemberID,
	}, option.WithSortBy(
		option.QuerySortBy{
			SortBy:  "created_at",
			OrderBy: "desc",
			Allow: map[string]bool{
				"created_at": true,
			},
		},
	), option.WithLockingUpdate())
	if err != nil {
		return nil, err
	}

	return lastEntry, nil
}

func (s *Service) processAddEntry(ctx context.Context, req *ledgerv1.AddEntryRequest) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 🔒 aktifkan row-level locking untuk semua query di tx ini
		tx = tx.Scopes(option.LockingUpdate)

		lastEntry, err := s.getLastEntry(tx, ctx, &LedgerEntry{
			TenantID: req.TenantId, MemberID: req.MemberId,
		})
		if err != nil {
			return err
		}

		switch req.Type {
		case ledgerv1.EntryType_DEBIT:
			return s.processDebit(ctx, tx, lastEntry, req)
		case ledgerv1.EntryType_CREDIT:
			return s.processCredit(ctx, tx, lastEntry, req)
		default:
			return errutil.BadRequest("unsupported entry type", nil)
		}
	})
}

func (s *Service) processDebit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry, req *ledgerv1.AddEntryRequest) error {

	creditTx := s.credit.WithTrx(tx)
	balanceTx := s.balance.WithTrx(tx)
	ledgerTx := s.ledger.WithTrx(tx)

	entries, err := creditTx.Find(ctx, &CreditPool{
		TenantID: req.TenantId,
		MemberID: req.MemberId,
	},
		option.ApplyOperator(option.Condition{
			Field:    "remaining",
			Operator: option.GT,
			Value:    0,
		}),
		option.WithSortBy(
			option.QuerySortBy{
				SortBy:  "created_at",
				OrderBy: "asc",
				Allow: map[string]bool{
					"created_at": true,
				},
			},
		),
	)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("insufficient points")
	}

	transactionID, err := GenerateTransactionID()
	if err != nil {
		zap.L().Error("failed to generate transactionId", zap.Error(err))
		return err
	}

	balance, err := balanceTx.FindOne(ctx, &Balance{
		TenantID: req.GetTenantId(),
		MemberID: req.GetMemberId(),
	})
	if err != nil {
		return err
	}

	if balance == nil {
		return fmt.Errorf("balance not found")
	}

	var totalAvailable int64
	for _, e := range entries {
		totalAvailable += e.Remaining
	}
	if totalAvailable < req.Amount {
		return fmt.Errorf("insufficient points: need=%d available=%d", req.Amount, totalAvailable)
	}

	remaining := req.Amount
	allocations := make([]RedeemAllocation, 0, len(entries))
	for _, entry := range entries {
		if remaining == 0 {
			break
		}

		allocatable := min(entry.Remaining, remaining)
		allocations = append(allocations, RedeemAllocation{
			CreditPoolID:    entry.ID,
			SourceID:        entry.LedgerEntryID,
			Amount:          allocatable,
			RemainingAmount: entry.Remaining - allocatable,
		})

		remaining -= allocatable
	}
	if remaining > 0 {
		return fmt.Errorf("insufficient points")
	}

	metadebit := make([]MetaDebit, 0, len(allocations))
	for _, a := range allocations {
		metadebit = append(metadebit, MetaDebit{
			LedgerEntryID: a.SourceID,
			Amount:        a.Amount,
		})
	}

	meta := make(map[string]any, len(req.Metadata)+1)
	for k, v := range req.Metadata {
		meta[k] = v
	}
	meta["sources"] = metadebit

	ledgerEntryID := s.node.Generate().String()
	metaBytes, _ := json.Marshal(req.Metadata)
	entry := NewLedgerEntry(LedgerParams{
		LedgerID:      ledgerEntryID,
		Type:          ledgerv1.EntryType_DEBIT.String(),
		TenantID:      req.GetTenantId(),
		MemberID:      req.GetMemberId(),
		Amount:        req.GetAmount(),
		TransactionID: transactionID,
		ReferenceID:   req.GetReferenceId(),
		Description:   req.GetDescription(),
		PreviousHash:  lastEntry.Hash,
		Metadata:      datatypes.JSON(metaBytes),
	})
	entry.Hash = entry.GenerateHash()

	if err := ledgerTx.Create(ctx, entry); err != nil {
		return err
	}

	for _, alloc := range allocations {
		updates := map[string]any{
			"remaining":   gorm.Expr("remaining - ?", alloc.Amount),
			"consumed_at": time.Now(),
		}

		if err := creditTx.Update(ctx, alloc.CreditPoolID, &updates); err != nil {
			zap.L().Error("failed to update credit pools", zap.Error(err))
			return err
		}
	}

	if balance.Balance < req.Amount {
		return fmt.Errorf("insufficient points (balance changed)")
	}

	updates := map[string]any{
		"balance":    gorm.Expr("balance - ?", req.Amount),
		"updated_at": time.Now(),
	}
	if err := balanceTx.Update(ctx, balance.ID, &updates); err != nil {
		return err
	}

	return nil
}

func (s *Service) processCredit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry, req *ledgerv1.AddEntryRequest) error {
	if req.Amount <= 0 {
		return errutil.BadRequest("amount must be > 0 for CREDIT", nil)
	}

	var (
		previousHash          = "GENESIS"
		previousBalance int64 = 0
	)

	balanceTx := s.balance.WithTrx(tx)
	creditTx := s.credit.WithTrx(tx)
	ledgerTx := s.ledger.WithTrx(tx)

	balance, err := balanceTx.FindOne(ctx, &Balance{
		TenantID: req.GetTenantId(), MemberID: req.GetMemberId(),
	})
	if err != nil {
		zap.L().Error("failed to query balance", zap.Error(err))
		return err
	}

	transactionID, err := GenerateTransactionID()
	if err != nil {
		return err
	}

	ledgerEntryID := s.node.Generate().String()
	metaBytes, _ := json.Marshal(req.Metadata)
	entry := NewLedgerEntry(LedgerParams{
		LedgerID: ledgerEntryID, TenantID: req.GetTenantId(), MemberID: req.GetMemberId(),
		Type: req.Type.String(), Amount: req.Amount, TransactionID: transactionID,
		ReferenceID: req.ReferenceId, Description: req.Description, Metadata: datatypes.JSON(metaBytes),
	})

	if lastEntry != nil {
		previousHash = lastEntry.Hash
	}

	if balance != nil {
		previousBalance = balance.Balance
	}

	entry.PreviousHash = previousHash
	entry.Hash = entry.GenerateHash()

	if err := ledgerTx.Create(ctx, entry); err != nil {
		return err
	}

	// credit pool
	if err := creditTx.Create(ctx, &CreditPool{
		ID: s.node.Generate().String(), TenantID: req.GetTenantId(), MemberID: req.GetMemberId(),
		LedgerEntryID: entry.ID, Remaining: req.Amount, CreatedAt: time.Now(),
	}); err != nil {
		return err
	}

	// upsert/update balance (masih di tx + locked)
	if balance == nil {
		if err := balanceTx.Create(ctx, &Balance{
			ID: s.node.Generate().String(), TenantID: req.GetTenantId(), MemberID: req.GetMemberId(),
			Balance: previousBalance + entry.Amount, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}); err != nil {
			return err
		}
	} else {
		if err := balanceTx.Update(ctx, balance.ID, &Balance{
			Balance: previousBalance + entry.Amount, UpdatedAt: time.Now(),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) processRevertCredit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry) error {

	balanceTx := s.balance.WithTrx(tx)
	balance, err := balanceTx.FindOne(ctx, &Balance{
		TenantID: lastEntry.TenantID,
		MemberID: lastEntry.MemberID,
	})
	if err != nil {
		return err
	}

	if balance == nil {
		return fmt.Errorf("balance not found")
	}

	transactionID, err := GenerateTransactionID()
	if err != nil {
		zap.L().Error("failed to generate transactionId", zap.Error(err))
		return err
	}

	ledgerEntryID := s.node.Generate().String()

	entry := NewLedgerEntry(LedgerParams{
		LedgerID:      ledgerEntryID,
		TenantID:      lastEntry.TenantID,
		MemberID:      lastEntry.MemberID,
		Type:          lastEntry.Type,
		Amount:        lastEntry.Amount,
		TransactionID: transactionID,
		ReferenceID:   lastEntry.TransactionID,
		Description:   fmt.Sprintf("Revert of %s", lastEntry.ID),
		Metadata:      lastEntry.Metadata,
	})

	entry.PreviousHash = lastEntry.Hash
	entry.Hash = entry.GenerateHash()

	if err := s.ledger.WithTrx(tx).Create(ctx, entry); err != nil {
		return err
	}

	return balanceTx.Update(ctx, balance.ID, &Balance{
		Balance:   balance.Balance - lastEntry.Amount,
		UpdatedAt: time.Now(),
	})
}

func (s *Service) ListEntries(ctx context.Context, req *ledgerv1.ListEntriesRequest) (*ledgerv1.ListEntriesResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	entries, err := s.ledger.Find(ctx, &LedgerEntry{
		TenantID: req.GetTenantId(),
		MemberID: req.GetMemberId(),
	})
	if err != nil {
		zap.L().With(opts...).Error("failed to query list entries", zap.Error(err))
		return nil, err
	}

	newEntries := make([]*ledgerv1.LedgerEntry, 0)
	for _, entry := range entries {
		newEntries = append(newEntries, &ledgerv1.LedgerEntry{
			Id:            entry.ID,
			TenantId:      entry.TenantID,
			MemberId:      entry.MemberID,
			Type:          ledgerv1.EntryType(ledgerv1.EntryType_value[entry.Type]),
			Amount:        entry.Amount,
			TransactionId: entry.TransactionID,
			ReferenceId:   entry.ReferenceID,
			Description:   entry.Description,
		})
	}

	return &ledgerv1.ListEntriesResponse{
		Data: newEntries,
	}, nil
}

func (s *Service) GetEntry(ctx context.Context, req *ledgerv1.GetEntryRequest) (*ledgerv1.LedgerEntry, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	entry, err := s.ledger.FindOne(ctx, &LedgerEntry{
		ID: req.Id,
	})
	if err != nil {
		zap.L().With(opts...).Error("failed to FindOne entry", zap.Error(err))
		return nil, err
	}

	return &ledgerv1.LedgerEntry{
		Id:            entry.ID,
		TenantId:      entry.TenantID,
		MemberId:      entry.MemberID,
		Type:          ledgerv1.EntryType(ledgerv1.EntryType_value[entry.Type]),
		Amount:        entry.Amount,
		TransactionId: entry.TransactionID,
		ReferenceId:   entry.ReferenceID,
		Description:   entry.Description,
	}, nil
}

func (s *Service) VerifyChain(ctx context.Context, req *ledgerv1.VerifyChainRequest) (*ledgerv1.VerifyChainResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	entries, err := s.ledger.Find(ctx, &LedgerEntry{
		TenantID: req.GetTenantId(),
		MemberID: req.GetMemberId(),
	})
	if err != nil {
		zap.L().With(opts...).Error("failed to query Find entries", zap.Error(err))
		return nil, err
	}

	var lastHash string
	for _, entry := range entries {
		expectedHash := entry.GenerateHash()
		if entry.Hash != expectedHash || entry.PreviousHash != lastHash {
			return &ledgerv1.VerifyChainResponse{
				Valid: false,
			}, nil
		}
		lastHash = entry.Hash
	}

	return &ledgerv1.VerifyChainResponse{
		Valid: true,
	}, nil
}
