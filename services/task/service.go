package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/hibiken/asynq"
	tenantv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/tenant/v1"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Service struct {
	db    *gorm.DB
	node  *snowflake.Node
	asynq *asynq.Client

	tenant  tenantv1.TenantServiceClient
	loyalty loyaltyv1.PointServiceClient
}

type Params struct {
	fx.In
	DB    *gorm.DB
	Node  *snowflake.Node
	Asynq *asynq.Client

	Tenant  tenantv1.TenantServiceClient `optional:"true"`
	Loyalty loyaltyv1.PointServiceClient `optional:"true"`
}

func NewService(p Params) *Service {
	return &Service{
		db:    p.DB,
		node:  p.Node,
		asynq: p.Asynq,

		tenant:  p.Tenant,
		loyalty: p.Loyalty,
	}
}

// EnqueueTenantExpiryJob creates a new Job record and sends it to Asynq queue.
func (s *Service) EnqueueTenantExpiryJob(ctx context.Context, tenantID string) error {
	taskType := "loyalty:expiry:run"

	payload, _ := json.Marshal(map[string]string{
		"tenant_id": tenantID,
	})
	task := asynq.NewTask(taskType, payload)

	// 1️⃣ Buat record job di DB
	job := Job{
		ID:        s.node.Generate().String(),
		TaskID:    "expiry_point", // static reference ke tasks.name
		TenantID:  tenantID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&job).Error; err != nil {
		return err
	}

	// 2️⃣ Enqueue job ke Asynq (queue khusus tenant)
	queueName := fmt.Sprintf("expiry:%s", tenantID)
	_, err := s.asynq.Enqueue(task, asynq.Queue(queueName))
	if err != nil {
		s.db.Model(&job).Update("status", "failed")
		return err
	}

	zap.L().Info("enqueued expiry job",
		zap.String("tenant_id", tenantID),
		zap.String("queue", queueName),
		zap.String("job_id", job.ID),
	)
	return nil
}

// HandleExpiryTask digunakan oleh Asynq worker.
// Ia hanya decode payload & delegasikan ke RunExpiryJob.
func (s *Service) HandleExpiryTask(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		zap.L().Error("invalid expiry payload", zap.Error(err))
		return err
	}

	zap.L().Info("Processing expiry task", zap.String("tenant_id", payload.TenantID))

	if err := s.RunExpiryJob(ctx, payload.TenantID); err != nil {
		zap.L().Error("failed to process expiry job",
			zap.String("tenant_id", payload.TenantID),
			zap.Error(err),
		)
		return err
	}

	zap.L().Info("Finished expiry task", zap.String("tenant_id", payload.TenantID))
	return nil
}

func (s *Service) RunExpiryJob(ctx context.Context, tenantID string) error {
	now := time.Now()
	jobID := s.node.Generate().String()

	job := Job{
		ID:        jobID,
		TaskID:    "expiry_point",
		TenantID:  tenantID,
		Status:    "running",
		StartedAt: &now,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return err
	}

	if _, err := s.loyalty.RunExpiryJob(ctx, &loyaltyv1.RunExpiryJobRequest{
		TenantId: tenantID,
	}); err != nil {
		zap.L().Error("failed to run expiry job", zap.String("tenant_id", tenantID), zap.Error(err))
		return err
	}

	s.db.Model(&Job{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":       "success",
		"completed_at": time.Now(),
	})
	zap.L().Info("expiry job finished", zap.String("tenant_id", tenantID))
	return nil
}

func (s *Service) EnqueueAllTenantsExpiryJobs(ctx context.Context) error {
	cursor := ""
	totalTenants := 0

	for {
		tenant, err := s.tenant.ListTenants(ctx, &tenantv1.ListTenantsRequest{
			Limit:  250,
			Cursor: cursor,
		})
		if err != nil {
			return err
		}

		if len(tenant.Tenants) == 0 {
			zap.L().Info("no tenants found in this batch", zap.String("cursor", cursor))
			break
		}

		wg := errgroup.Group{}
		for _, t := range tenant.Tenants {
			if err := s.EnqueueTenantExpiryJob(ctx, t.TenantId); err != nil {
				zap.L().Error("failed enqueue expiry job", zap.String("tenant_id", t.TenantId), zap.Error(err))
			}
		}
		_ = wg.Wait()

		if tenant.Page == nil || !tenant.Page.HasMore {
			break
		}

		cursor = tenant.Page.NextCursor
	}

	zap.L().Info("finished enqueue all expiry jobs", zap.Int("total_tenants", totalTenants))
	return nil
}
