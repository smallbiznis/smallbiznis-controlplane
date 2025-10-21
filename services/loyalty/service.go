package loyalty

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/hibiken/asynq"
	ledgerv1 "github.com/smallbiznis/go-genproto/smallbiznis/ledger/v1"
	loyaltyv1 "github.com/smallbiznis/go-genproto/smallbiznis/loyalty/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	loyaltyv1.UnimplementedPointServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db     *gorm.DB
	node   *snowflake.Node
	asynq  *asynq.Client
	ledger ledgerv1.LedgerServiceClient
}

type ServiceParams struct {
	fx.In
	DB     *gorm.DB
	Node   *snowflake.Node
	Asynq  *asynq.Client
	Ledger ledgerv1.LedgerServiceClient `optional:"true"`
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:     p.DB,
		node:   p.Node,
		asynq:  p.Asynq,
		ledger: p.Ledger,
	}
}

func (s *Service) GetBalance(ctx context.Context, req *loyaltyv1.GetBalanceRequest) (*loyaltyv1.GetBalanceResponse, error) {
	resp, err := s.ledger.GetBalance(ctx, &ledgerv1.GetBalanceRequest{
		TenantId: req.TenantId,
		MemberId: req.MemberId,
	})
	if err != nil {
		return nil, err
	}
	return &loyaltyv1.GetBalanceResponse{
		Balance: resp.Balance,
	}, nil
}

func (s *Service) GetExpiringPoints(ctx context.Context, req *loyaltyv1.GetExpiringPointsRequest) (*loyaltyv1.GetExpiringPointsResponse, error) {
	days := int(req.WithinDays)
	if days <= 0 {
		days = 7
	}

	var results []struct {
		ExpireDate string
		Total      int64
	}

	err := s.db.WithContext(ctx).
		Table("point_transactions").
		Select("expire_date, SUM(point_delta) AS total").
		Where("tenant_id = ? AND member_id = ? AND type = ? AND status = 'success' AND expire_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '? days'",
			req.TenantId, req.MemberId, Earning, days).
		Group("expire_date").
		Order("expire_date ASC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	var totalExpiring int64
	var batches []*loyaltyv1.ExpiringPointBatch

	for _, r := range results {
		totalExpiring += r.Total
		batches = append(batches, &loyaltyv1.ExpiringPointBatch{
			ExpireDate:  r.ExpireDate,
			TotalPoints: r.Total,
		})
	}

	return &loyaltyv1.GetExpiringPointsResponse{
		TotalExpiringPoints: totalExpiring,
		Batches:             batches,
	}, nil
}

func (s *Service) Earning(ctx context.Context, req *loyaltyv1.EarningRequest) (*loyaltyv1.EarningResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
		zap.String("tenant_id", req.GetTenantId()),
	}

	zapLog := zap.L().With(traceOpt...)

	earningID := s.node.Generate().String()
	earning := PointTransaction{
		ID:          earningID,
		TenantID:    req.TenantId,
		MemberID:    req.UserId,
		ReferenceID: req.ReferenceId,
		Type:        Earning,
		PointDelta:  0,
		Status:      loyaltyv1.Status_PENDING.String(),
	}
	if attr, err := protojson.Marshal(req); err == nil {
		earning.Metadata = datatypes.JSON(attr)
	}

	if err := s.db.WithContext(ctx).Create(&earning).Error; err != nil {
		zapLog.Error("failed to insert earning record", zap.Error(err))
		return nil, err
	}

	p, _ := json.Marshal(ProcessEarningPayload{
		EarningID:   earningID,
		TenantID:    req.GetTenantId(),
		UserID:      req.GetUserId(),
		ReferenceID: req.GetReferenceId(),
		EventType:   req.GetEventType(),
		TraceID:     traceID,
	})
	if _, err := s.asynq.EnqueueContext(ctx,
		asynq.NewTask(LoyaltyProcessEarning, p),
		asynq.Queue("loyalty"),
		asynq.MaxRetry(3),
	); err != nil {
		zapLog.Error("failed to enqueue loyalty earning", zap.Error(err))
		return nil, err
	}

	return &loyaltyv1.EarningResponse{
		TransactionId: earning.ID,
		Status:        loyaltyv1.Status_PENDING,
		CreatedAt:     timestamppbNow(),
	}, nil
}

func (s *Service) Redemption(ctx context.Context, req *loyaltyv1.RedeemRequest) (*loyaltyv1.RedeemResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
		zap.String("tenant_id", req.GetTenantId()),
	}

	zapLog := zap.L().With(traceOpt...)

	redemptionID := s.node.Generate().String()
	red := PointTransaction{
		ID:          redemptionID,
		TenantID:    req.TenantId,
		MemberID:    req.UserId,
		ReferenceID: req.ReferenceId,
		Type:        Redemption,
		RewardID:    req.RewardId,
		RewardType:  req.RewardType.String(),
		PointDelta:  0,
		Status:      loyaltyv1.Status_PENDING.String(),
	}
	if meta, err := json.Marshal(req); err == nil {
		red.Metadata = meta
	}
	if err := s.db.WithContext(ctx).Create(&red).Error; err != nil {
		return nil, err
	}

	// cek saldo via LedgerService
	bal, err := s.ledger.GetBalance(ctx, &ledgerv1.GetBalanceRequest{
		TenantId: req.TenantId,
		MemberId: req.UserId,
	})
	if err != nil {
		zapLog.Error("failed to get ledger balance", zap.Error(err))
		return nil, err
	}

	required := int64(100) // dummy rule sementara
	if bal.GetBalance() < required {
		s.db.WithContext(ctx).Model(&red).Update("status", "failed")
		return nil, fmt.Errorf("insufficient points")
	}

	// call ledger service: deduct points
	_, err = s.ledger.AddEntry(ctx, &ledgerv1.AddEntryRequest{
		TenantId:    req.GetTenantId(),
		MemberId:    req.GetUserId(),
		ReferenceId: req.GetReferenceId(),
		Type:        ledgerv1.EntryType_DEBIT,
		Amount:      required,
	})
	if err != nil {
		s.db.WithContext(ctx).Model(&red).Update("status", "failed")
		return nil, err
	}

	now := time.Now()
	s.db.WithContext(ctx).Model(&red).Updates(map[string]interface{}{
		"status":       Success,
		"processed_at": now,
		"point_delta":  required,
	})

	return &loyaltyv1.RedeemResponse{
		ReferenceId:   req.ReferenceId,
		TransactionId: red.ID,
		Status:        loyaltyv1.Status_SUCCESS,
		RedeemedAt:    timestamppbNow(),
	}, nil
}

func (s *Service) RunExpiryJob(ctx context.Context, req *loyaltyv1.RunExpiryJobRequest) (*loyaltyv1.RunExpiryJobResponse, error) {
	now := time.Now()
	if req.RunAt != nil {
		now = req.RunAt.AsTime()
	}

	jobID := s.node.Generate().String()
	zap.L().Info("Running expiry job", zap.String("tenant_id", req.TenantId))

	// ambil poin yang expire hari ini
	var results []struct {
		MemberID string
		Total    int64
	}
	err := s.db.WithContext(ctx).
		Table("point_transactions").
		Select("member_id, SUM(point_delta) as total").
		Where(`
			tenant_id = ? AND type = 'earning' AND status = 'success'
			AND expire_date <= CURRENT_DATE
		`, req.TenantId).
		Group("member_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	var totalPoints, totalMembers int64
	expiredMembers := make([]*loyaltyv1.ExpiredMember, 0, len(results))

	for _, r := range results {
		totalMembers++
		totalPoints += r.Total
		expiredMembers = append(expiredMembers, &loyaltyv1.ExpiredMember{
			MemberId:      r.MemberID,
			ExpiredPoints: r.Total,
		})

		if req.DryRun {
			continue
		}

		_, err := s.ledger.AddEntry(ctx, &ledgerv1.AddEntryRequest{
			TenantId:    req.TenantId,
			MemberId:    r.MemberID,
			Type:        ledgerv1.EntryType_DEBIT,
			Amount:      -r.Total,
			ReferenceId: fmt.Sprintf("expiry:%s", jobID),
			Description: fmt.Sprintf("Expired points on %s", now.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Error("ledger debit failed", zap.String("member_id", r.MemberID), zap.Error(err))
			continue
		}
	}

	status := "success"
	if totalPoints == 0 {
		status = "no_data"
	}

	return &loyaltyv1.RunExpiryJobResponse{
		JobId:                jobID,
		TotalExpiredPoints:   totalPoints,
		TotalMembersAffected: totalMembers,
		Status:               status,
		CompletedAt:          timestamppb.New(time.Now()),
		Members:              expiredMembers,
	}, nil
}

// ──────────────────────────────
//  Helpers
// ──────────────────────────────

func timestamppbNow() *timestamppb.Timestamp {
	return &timestamppb.Timestamp{Seconds: time.Now().Unix()}
}

func DefaultEndOfYearDate() *time.Time {
	now := time.Now()
	endOfYear := time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
	if now.After(endOfYear) {
		endOfYear = time.Date(now.Year()+1, time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	return &endOfYear
}
