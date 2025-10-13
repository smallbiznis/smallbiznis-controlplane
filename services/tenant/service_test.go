package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/hibiken/asynq"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"smallbiznis-controlplane/pkg/config"
	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/services/apikey"
	"smallbiznis-controlplane/services/domain"
	"smallbiznis-controlplane/services/testutil"
)

func init() {
	zap.ReplaceGlobals(zap.NewNop())
}

type mockTenantRepository struct {
	findFn    func(ctx context.Context, query *Tenant, opts ...option.QueryOption) ([]*Tenant, error)
	findOneFn func(ctx context.Context, query *Tenant, opts ...option.QueryOption) (*Tenant, error)
}

func (m *mockTenantRepository) WithTrx(tx *gorm.DB) repository.Repository[Tenant] {
	return m
}

func (m *mockTenantRepository) Find(ctx context.Context, query *Tenant, opts ...option.QueryOption) ([]*Tenant, error) {
	if m.findFn != nil {
		return m.findFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *mockTenantRepository) FindOne(ctx context.Context, query *Tenant, opts ...option.QueryOption) (*Tenant, error) {
	if m.findOneFn != nil {
		return m.findOneFn(ctx, query, opts...)
	}
	return nil, nil
}

func (m *mockTenantRepository) Create(context.Context, *Tenant) error         { return nil }
func (m *mockTenantRepository) Update(context.Context, string, any) error     { return nil }
func (m *mockTenantRepository) BatchCreate(context.Context, []*Tenant) error  { return nil }
func (m *mockTenantRepository) BatchUpdate(context.Context, []*Tenant) error  { return nil }
func (m *mockTenantRepository) Count(context.Context, *Tenant) (int64, error) { return 0, nil }

type fakeEnqueuer struct {
	tasks []*asynq.Task
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return nil, nil
}

func TestListTenantsSuccess(t *testing.T) {
	now := time.Now()
	repo := &mockTenantRepository{}
	repo.findFn = func(ctx context.Context, _ *Tenant, _ ...option.QueryOption) ([]*Tenant, error) {
		return []*Tenant{
			{ID: "tenant-1", Name: "Tenant One", Slug: "tenant-one", CreatedAt: now, UpdatedAt: now},
			{ID: "tenant-2", Name: "Tenant Two", Slug: "tenant-two", CreatedAt: now, UpdatedAt: now},
		}, nil
	}
	svc := &Service{repo: repo}

	resp, err := svc.ListTenants(context.Background(), &tenantv1.ListTenantsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetTenants(), 2)
	require.Equal(t, "tenant-one", resp.GetTenants()[0].GetSlug())
}

func TestListTenantsRepositoryError(t *testing.T) {
	repo := &mockTenantRepository{}
	repo.findFn = func(ctx context.Context, _ *Tenant, _ ...option.QueryOption) ([]*Tenant, error) {
		return nil, errors.New("boom")
	}
	svc := &Service{repo: repo}

	_, err := svc.ListTenants(context.Background(), &tenantv1.ListTenantsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestCreateTenantRootDomainMissing(t *testing.T) {
	svc := &Service{config: &config.Config{RootDomain: ""}}
	_, err := svc.CreateTenant(context.Background(), &tenantv1.CreateTenantRequest{Name: "Tenant"})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestCreateTenantSlugExists(t *testing.T) {
	repo := &mockTenantRepository{}
	repo.findOneFn = func(ctx context.Context, _ *Tenant, _ ...option.QueryOption) (*Tenant, error) {
		return &Tenant{ID: "existing"}, nil
	}
	svc := &Service{
		config: &config.Config{RootDomain: "example.com"},
		repo:   repo,
	}

	_, err := svc.CreateTenant(context.Background(), &tenantv1.CreateTenantRequest{Name: "Tenant", Slug: "tenant"})
	require.Error(t, err)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestGetTenantNotFound(t *testing.T) {
	repo := &mockTenantRepository{}
	repo.findOneFn = func(ctx context.Context, _ *Tenant, _ ...option.QueryOption) (*Tenant, error) {
		return nil, nil
	}
	svc := &Service{repo: repo}

	_, err := svc.GetTenant(context.Background(), &tenantv1.GetTenantRequest{TenantId: "unknown"})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateTenantSuccess(t *testing.T) {
	db := testutil.NewTestDB(t, &Tenant{}, &domain.Domain{}, &apikey.APIKey{})
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	enqueuer := &fakeEnqueuer{}
	cfg := &config.Config{RootDomain: "example.com"}

	svc := NewService(ServiceParams{DB: db, Node: node, Config: cfg, Asynq: enqueuer})

	resp, err := svc.CreateTenant(context.Background(), &tenantv1.CreateTenantRequest{
		Name:        "Tenant Name",
		CountryCode: "ID",
		Timezone:    "Asia/Jakarta",
		Type:        "company",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "tenant-name", resp.GetSlug())
	require.Equal(t, string(Active), resp.GetStatus())

	var count int64
	require.NoError(t, db.Model(&Tenant{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.Len(t, enqueuer.tasks, 5)
}
