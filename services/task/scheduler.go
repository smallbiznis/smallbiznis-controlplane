package task

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Scheduler struct {
	service *Service
}

func NewScheduler(svc *Service) *Scheduler {
	return &Scheduler{service: svc}
}

// StartScheduler dipanggil otomatis oleh FX saat service start
func StartScheduler(lc fx.Lifecycle, s *Scheduler) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go s.run(ctx)
			return nil
		},
	})
}

// run menjalankan loop scheduler harian
func (s *Scheduler) run(ctx context.Context) {
	zap.L().Info("[Scheduler] started loyalty expiry scheduler")

	for {
		now := time.Now()
		next := nextRunTime(now, 1, 0) // jam 01:00 pagi

		sleepDuration := next.Sub(now)
		zap.L().Info("[Scheduler] next run scheduled",
			zap.Time("next_run", next),
			zap.Duration("sleep_for", sleepDuration),
		)
		select {
		case <-time.After(sleepDuration):
			s.runDaily(ctx)
		case <-ctx.Done():
			zap.L().Warn("[Scheduler] stopped")
			return
		}
	}
}

func (s *Scheduler) runDaily(ctx context.Context) {
	start := time.Now()
	zap.L().Info("[Scheduler] Running daily expiry enqueue job")

	err := s.service.EnqueueAllTenantsExpiryJobs(ctx)
	if err != nil {
		zap.L().Error("[Scheduler] failed enqueue all tenants", zap.Error(err))
		return
	}

	zap.L().Info("[Scheduler] Finished enqueue all tenants",
		zap.Duration("duration", time.Since(start)),
	)
}

// nextRunTime menghitung jam berikutnya pada jam tertentu
func nextRunTime(now time.Time, hour, minute int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
