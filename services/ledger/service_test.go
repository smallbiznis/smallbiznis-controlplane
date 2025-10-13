package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/errutil"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/services/testutil"
)

func init() {
	zap.ReplaceGlobals(zap.NewNop())
}

type repoMock[T any] struct {
	withTrxFn     func(tx *gorm.DB) repository.Repository[T]
	findFn        func(ctx context.Context, query *T, opts ...option.QueryOption) ([]*T, error)
	findOneFn     func(ctx context.Context, query *T, opts ...option.QueryOption) (*T, error)
	createFn      func(ctx context.Context, resource *T) error
	updateFn      func(ctx context.Context, resourceID string, resource any) error
	batchCreateFn func(ctx context.Context, resources []*T) error
	batchUpdateFn func(ctx context.Context, resources []*T) error
	countFn       func(ctx context.Context, query *T) (int64, error)
}

func (m *repoMock[T]) WithTrx(tx *gorm.DB) repository.Repository[T] {
	if m.withTrxFn != nil {
		return m.withTrxFn(tx)
	}
	return m
}

func (m *repoMock[T]) Find(ctx context.Context, query *T, opts ...option.QueryOption) ([]*T, error) {
	if m.findFn != nil {
		return m.findFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *repoMock[T]) FindOne(ctx context.Context, query *T, opts ...option.QueryOption) (*T, error) {
	if m.findOneFn != nil {
		return m.findOneFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *repoMock[T]) Create(ctx context.Context, resource *T) error {
	if m.createFn != nil {
		return m.createFn(ctx, resource)
	}
	return nil
}

func (m *repoMock[T]) Update(ctx context.Context, resourceID string, resource any) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, resourceID, resource)
	}
	return nil
}

func (m *repoMock[T]) BatchCreate(ctx context.Context, resources []*T) error {
	if m.batchCreateFn != nil {
		return m.batchCreateFn(ctx, resources)
	}
	return nil
}

func (m *repoMock[T]) BatchUpdate(ctx context.Context, resources []*T) error {
	if m.batchUpdateFn != nil {
		return m.batchUpdateFn(ctx, resources)
	}
	return nil
}

func (m *repoMock[T]) Count(ctx context.Context, query *T) (int64, error) {
	if m.countFn != nil {
		return m.countFn(ctx, query)
	}
	return 0, nil
}

func TestNewService(t *testing.T) {
	db := testutil.NewTestDB(t)
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	svc := NewService(ServiceParams{DB: db, Node: node})

	require.NotNil(t, svc.ledger)
	require.NotNil(t, svc.balance)
	require.NotNil(t, svc.credit)
}

func TestGetBalanceSuccess(t *testing.T) {
	now := time.Now()
	svc := &Service{
		balance: &repoMock[Balance]{
			findOneFn: func(ctx context.Context, _ *Balance, opts ...option.QueryOption) (*Balance, error) {
				return &Balance{Balance: 150, CreatedAt: now}, nil
			},
		},
	}

	resp, err := svc.GetBalance(context.Background(), &ledgerv1.GetBalanceRequest{TenantId: "tenant", MemberId: "member"})
	require.NoError(t, err)
	require.Equal(t, int64(150), resp.GetBalance())
	require.True(t, resp.GetLastUpdatedAt().AsTime().Equal(now))
}

func TestAddEntryDuplicateReference(t *testing.T) {
	svc := &Service{
		ledger: &repoMock[LedgerEntry]{
			findOneFn: func(ctx context.Context, _ *LedgerEntry, opts ...option.QueryOption) (*LedgerEntry, error) {
				return &LedgerEntry{ID: "existing"}, nil
			},
		},
	}

	entry, err := svc.AddEntry(context.Background(), &ledgerv1.AddEntryRequest{
		TenantId:    "tenant",
		MemberId:    "member",
		ReferenceId: "ref-1",
	})

	require.Nil(t, entry)
	require.Error(t, err)
	var be errutil.BaseError
	require.True(t, errors.As(err, &be))
	require.Equal(t, errutil.StatusBadRequest, be.Status())
}

func TestVerifyChainValid(t *testing.T) {
	first := &LedgerEntry{
		ID:        "entry-1",
		TenantID:  "tenant",
		MemberID:  "member",
		Type:      ledgerv1.EntryType_CREDIT.String(),
		Amount:    100,
		CreatedAt: time.Now(),
	}
	first.Hash = first.GenerateHash()

	second := &LedgerEntry{
		ID:           "entry-2",
		TenantID:     "tenant",
		MemberID:     "member",
		Type:         ledgerv1.EntryType_DEBIT.String(),
		Amount:       50,
		PreviousHash: first.Hash,
		CreatedAt:    time.Now().Add(time.Minute),
	}
	second.Hash = second.GenerateHash()

	svc := &Service{
		ledger: &repoMock[LedgerEntry]{
			findFn: func(ctx context.Context, _ *LedgerEntry, opts ...option.QueryOption) ([]*LedgerEntry, error) {
				return []*LedgerEntry{first, second}, nil
			},
		},
	}

	resp, err := svc.VerifyChain(context.Background(), &ledgerv1.VerifyChainRequest{TenantId: "tenant", MemberId: "member"})
	require.NoError(t, err)
	require.True(t, resp.GetValid())
}

func TestVerifyChainInvalid(t *testing.T) {
	first := &LedgerEntry{
		ID:        "entry-1",
		TenantID:  "tenant",
		MemberID:  "member",
		Type:      ledgerv1.EntryType_CREDIT.String(),
		Amount:    100,
		CreatedAt: time.Now(),
	}
	first.Hash = first.GenerateHash()

	second := &LedgerEntry{
		ID:           "entry-2",
		TenantID:     "tenant",
		MemberID:     "member",
		Type:         ledgerv1.EntryType_DEBIT.String(),
		Amount:       50,
		PreviousHash: first.Hash,
		Hash:         "invalid",
		CreatedAt:    time.Now().Add(time.Minute),
	}

	svc := &Service{
		ledger: &repoMock[LedgerEntry]{
			findFn: func(ctx context.Context, _ *LedgerEntry, opts ...option.QueryOption) ([]*LedgerEntry, error) {
				return []*LedgerEntry{first, second}, nil
			},
		},
	}

	resp, err := svc.VerifyChain(context.Background(), &ledgerv1.VerifyChainRequest{TenantId: "tenant", MemberId: "member"})
	require.NoError(t, err)
	require.False(t, resp.GetValid())
}
