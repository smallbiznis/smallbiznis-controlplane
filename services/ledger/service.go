package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/errutil"
	"smallbiznis-controlplane/pkg/repository"
	"time"

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
	db      *gorm.DB
	node    *snowflake.Node
	ledger  repository.Repository[LedgerEntry]
	balance repository.Repository[Balance]
	credit  repository.Repository[CreditPool]
	ledgerv1.UnimplementedLedgerServiceServer
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:      p.DB,
		node:    p.Node,
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

	opts := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	exist, err := s.ledger.FindOne(ctx, &LedgerEntry{
		TenantID:    req.GetTenantId(),
		ReferenceID: req.GetReferenceId(),
	})
	if err != nil {
		zap.L().With(opts...).Error("failed to query FindOne entry", zap.Error(err))
		return nil, err
	}

	if exist != nil {
		zap.L().With(opts...).Error("failed to create new entry", zap.Error(fmt.Errorf("reference_id %s already exists", req.ReferenceId)))
		return nil, errutil.BadRequest("failed to create new entry; reference_id already exists", nil)
	}

	if err := s.processAddEntry(ctx, req); err != nil {
		zap.L().Error("failed process add entry", zap.Error(err))
		return nil, err
	}

	entry, err := s.ledger.FindOne(ctx, &LedgerEntry{
		ReferenceID: req.ReferenceId,
	})
	if err != nil {
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

		originalEntry, err := s.ledger.WithTrx(tx).FindOne(ctx, &LedgerEntry{
			ID: req.EntryId,
		}, option.WithLockingUpdate())
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

		lastEntry, err := s.getLastEntry(tx, ctx, &LedgerEntry{
			TenantID: req.TenantId,
			MemberID: req.MemberId,
		})
		if err != nil {
			return err
		}

		// Handle DEBIT
		if req.Type == ledgerv1.EntryType_DEBIT {
			return s.processDebit(ctx, tx, lastEntry, req)
		}

		// Handle CREDIT
		return s.processCredit(ctx, tx, lastEntry, req)
	})
}

func (s *Service) processDebit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry, req *ledgerv1.AddEntryRequest) error {

	entries, err := s.credit.WithTrx(tx).Find(ctx, &CreditPool{
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
		option.WithLockingUpdate(),
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

	balance, err := s.balance.FindOne(ctx, &Balance{
		TenantID: req.GetTenantId(),
		MemberID: req.GetMemberId(),
	},
		option.WithLockingUpdate(),
	)
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

	if err := s.ledger.WithTrx(tx).Create(ctx, entry); err != nil {
		return err
	}

	for _, alloc := range allocations {
		updates := map[string]any{
			"remaining":   gorm.Expr("remaining - ?", alloc.Amount),
			"consumed_at": time.Now(),
		}
		if err := s.credit.WithTrx(tx).Update(ctx, alloc.CreditPoolID, &updates); err != nil {
			zap.L().Error("failed to update credit pools", zap.Error(err))
			return err
		}
	}

	updates := map[string]any{
		"balance":    gorm.Expr("balance - ?", req.Amount),
		"updated_at": time.Now(),
	}
	if err := s.balance.WithTrx(tx).Update(ctx, balance.ID, &updates); err != nil {
		return err
	}

	return nil
}

func (s *Service) processCredit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry, req *ledgerv1.AddEntryRequest) error {
	var (
		previousHash    string = "GENESIS"
		previousBalance int64  = 0
	)

	balance, err := s.balance.WithTrx(tx).FindOne(ctx, &Balance{
		TenantID: req.GetTenantId(),
		MemberID: req.GetMemberId(),
	}, option.WithLockingUpdate())
	if err != nil {
		zap.L().Error("failed to query balance", zap.Error(err))
		return err
	}

	transactionID, err := GenerateTransactionID()
	if err != nil {
		zap.L().Error("failed to generate transactionId", zap.Error(err))
		return err
	}

	ledgerEntryID := s.node.Generate().String()
	metaBytes, _ := json.Marshal(req.Metadata)
	entry := NewLedgerEntry(LedgerParams{
		LedgerID:      ledgerEntryID,
		TenantID:      req.GetTenantId(),
		MemberID:      req.GetMemberId(),
		Type:          req.Type.String(),
		Amount:        req.Amount,
		TransactionID: transactionID,
		ReferenceID:   req.ReferenceId,
		Description:   req.Description,
		Metadata:      datatypes.JSON(metaBytes),
	})

	if lastEntry != nil {
		previousHash = lastEntry.Hash
		previousBalance = balance.Balance
	}

	entry.PreviousHash = previousHash
	entry.Hash = entry.GenerateHash()

	if err := s.ledger.WithTrx(tx).Create(ctx, entry); err != nil {
		zap.L().Error("failed to create entry", zap.Error(err))
		return err
	}

	creditPoolID := s.node.Generate().String()
	if err := s.credit.WithTrx(tx).Create(ctx, &CreditPool{
		ID:            creditPoolID,
		TenantID:      req.GetTenantId(),
		MemberID:      req.GetMemberId(),
		LedgerEntryID: entry.ID,
		Remaining:     req.Amount,
		CreatedAt:     time.Now(),
	}); err != nil {
		zap.L().Error("failed to create credit pools", zap.Error(err))
		return err
	}

	if balance == nil {
		balanceID := s.node.Generate().String()
		if err := s.balance.WithTrx(tx).Create(ctx, &Balance{
			ID:        balanceID,
			TenantID:  req.GetTenantId(),
			MemberID:  req.GetMemberId(),
			Balance:   entry.Amount,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}); err != nil {
			zap.L().Error("failed to create balance", zap.Error(err))
			return err
		}
	} else {
		if err := s.balance.WithTrx(tx).Update(ctx, balance.ID, &Balance{
			Balance:   entry.Amount + previousBalance,
			UpdatedAt: time.Now(),
		}); err != nil {
			zap.L().Error("failed to update balance", zap.Error(err))
			return err
		}
	}

	return nil
}

func (s *Service) processRevertCredit(ctx context.Context, tx *gorm.DB, lastEntry *LedgerEntry) error {

	balance, err := s.balance.FindOne(ctx, &Balance{
		TenantID: lastEntry.TenantID,
		MemberID: lastEntry.MemberID,
	},
		option.WithLockingUpdate(),
	)
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

	return s.balance.WithTrx(tx).Update(ctx, balance.ID, &Balance{
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
