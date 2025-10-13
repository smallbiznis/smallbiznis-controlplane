package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	domainv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/domain/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/services/testutil"
)

func init() {
	zap.ReplaceGlobals(zap.NewNop())
}

type mockDomainRepository struct {
	findFn    func(ctx context.Context, query *Domain, opts ...option.QueryOption) ([]*Domain, error)
	findOneFn func(ctx context.Context, query *Domain, opts ...option.QueryOption) (*Domain, error)
}

func (m *mockDomainRepository) WithTrx(tx *gorm.DB) repository.Repository[Domain] {
	return m
}

func (m *mockDomainRepository) Find(ctx context.Context, query *Domain, opts ...option.QueryOption) ([]*Domain, error) {
	if m.findFn != nil {
		return m.findFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *mockDomainRepository) FindOne(ctx context.Context, query *Domain, opts ...option.QueryOption) (*Domain, error) {
	if m.findOneFn != nil {
		return m.findOneFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *mockDomainRepository) Create(context.Context, *Domain) error         { return nil }
func (m *mockDomainRepository) Update(context.Context, string, any) error     { return nil }
func (m *mockDomainRepository) BatchCreate(context.Context, []*Domain) error  { return nil }
func (m *mockDomainRepository) BatchUpdate(context.Context, []*Domain) error  { return nil }
func (m *mockDomainRepository) Count(context.Context, *Domain) (int64, error) { return 0, nil }

func TestNewService(t *testing.T) {
	db := testutil.NewTestDB(t)
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	svc := NewService(ServiceParams{DB: db, Node: node})

	require.NotNil(t, svc.repo)
}

func TestListDomainsSuccess(t *testing.T) {
	db := testutil.NewTestDB(t, &Domain{})
	code := "verify-code"
	record := &Domain{
		ID:               "dom_1",
		TenantID:         "tenant_1",
		Hostname:         "example.com",
		VerificationCode: &code,
		Verified:         true,
		CreatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(record).Error)

	node, err := snowflake.NewNode(2)
	require.NoError(t, err)
	svc := NewService(ServiceParams{DB: db, Node: node})

	resp, err := svc.ListDomains(context.Background(), &domainv1.ListDomainsRequest{TenantId: "tenant_1"})
	require.NoError(t, err)
	require.Len(t, resp.GetDomains(), 1)
	require.Equal(t, record.ID, resp.GetDomains()[0].GetDomainId())
	require.Equal(t, record.Hostname, resp.GetDomains()[0].GetHostname())
}

func TestListDomainsRepositoryError(t *testing.T) {
	svc := &Service{repo: &mockDomainRepository{findFn: func(ctx context.Context, _ *Domain, opts ...option.QueryOption) ([]*Domain, error) {
		return nil, errors.New("boom")
	}}}

	_, err := svc.ListDomains(context.Background(), &domainv1.ListDomainsRequest{TenantId: "tenant_1"})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestGetDomainInvalidID(t *testing.T) {
	svc := &Service{}
	_, err := svc.GetDomain(context.Background(), &domainv1.GetDomainRequest{DomainId: "   "})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetDomainNotFound(t *testing.T) {
	svc := &Service{repo: &mockDomainRepository{findOneFn: func(ctx context.Context, _ *Domain, opts ...option.QueryOption) (*Domain, error) {
		return nil, nil
	}}}

	_, err := svc.GetDomain(context.Background(), &domainv1.GetDomainRequest{DomainId: "dom_1"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetDomainSuccess(t *testing.T) {
	code := "verify"
	now := time.Now()
	svc := &Service{repo: &mockDomainRepository{findOneFn: func(ctx context.Context, _ *Domain, opts ...option.QueryOption) (*Domain, error) {
		return &Domain{ID: "dom_1", TenantID: "tenant_1", Hostname: "example.com", VerificationCode: &code, Verified: true, CreatedAt: now}, nil
	}}}

	resp, err := svc.GetDomain(context.Background(), &domainv1.GetDomainRequest{DomainId: "dom_1"})
	require.NoError(t, err)
	require.Equal(t, "dom_1", resp.GetDomainId())
	require.Equal(t, "example.com", resp.GetHostname())
	require.True(t, resp.GetVerified())
}

func TestVerifyDomainAlreadyVerified(t *testing.T) {
	db := testutil.NewTestDB(t, &Domain{})
	code := "verified"
	record := &Domain{
		ID:                 "dom_1",
		TenantID:           "tenant_1",
		Hostname:           "example.com",
		VerificationMethod: DNS,
		VerificationCode:   &code,
		Verified:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	require.NoError(t, db.Create(record).Error)

	svc := &Service{db: db}

	resp, err := svc.VerifyDomain(context.Background(), &domainv1.VerifyDomainRequest{TenantId: "tenant_1", Hostname: "example.com"})
	require.NoError(t, err)
	require.True(t, resp.GetSuccess())
	require.Contains(t, resp.GetMessage(), "already verified")
}

func TestVerifyDomainNotFound(t *testing.T) {
	db := testutil.NewTestDB(t, &Domain{})
	svc := &Service{db: db}

	_, err := svc.VerifyDomain(context.Background(), &domainv1.VerifyDomainRequest{TenantId: "tenant_1", Hostname: "missing"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestVerifyDomainUnsupportedMethod(t *testing.T) {
	db := testutil.NewTestDB(t, &Domain{})
	code := "code"
	record := &Domain{
		ID:                 "dom_1",
		TenantID:           "tenant_1",
		Hostname:           "example.com",
		VerificationMethod: File,
		VerificationCode:   &code,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	require.NoError(t, db.Create(record).Error)

	svc := &Service{db: db}

	_, err := svc.VerifyDomain(context.Background(), &domainv1.VerifyDomainRequest{TenantId: "tenant_1", Hostname: "example.com"})
	require.Equal(t, codes.Unimplemented, status.Code(err))
}
