package voucher

import (
	"context"
	"encoding/json"
	"fmt"
	"smallbiznis-controlplane/pkg/celengine"
	"smallbiznis-controlplane/pkg/db/option"
	"smallbiznis-controlplane/pkg/db/pagination"
	"smallbiznis-controlplane/pkg/repository"
	"smallbiznis-controlplane/pkg/sequence"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gogo/status"
	voucherv1 "github.com/smallbiznis/go-genproto/smallbiznis/voucher/v1"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/gorm"
)

type Service struct {
	voucherv1.UnimplementedVoucherServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db           *gorm.DB
	node         *snowflake.Node
	seq          sequence.Generator
	campaign     repository.Repository[VoucherCampaign]
	voucher      repository.Repository[Voucher]
	voucherEvent repository.Repository[VoucherEvent]
	voucherIssue repository.Repository[VoucherIssuance]
}

type ServiceParams struct {
	fx.In
	DB   *gorm.DB
	Node *snowflake.Node
	Seq  sequence.Generator
}

func NewService(p ServiceParams) *Service {
	return &Service{
		db:           p.DB,
		node:         p.Node,
		seq:          p.Seq,
		campaign:     repository.ProvideStore[VoucherCampaign](p.DB),
		voucher:      repository.ProvideStore[Voucher](p.DB),
		voucherEvent: repository.ProvideStore[VoucherEvent](p.DB),
		voucherIssue: repository.ProvideStore[VoucherIssuance](p.DB),
	}
}

func (s *Service) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// You can optionally check database connectivity here:
	sqlDB, err := s.db.DB()
	if err != nil {
		return nil, status.Error(codes.Internal, "db not ready")
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *Service) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	// Optional: implement streaming health status (rarely used)
	return status.Error(codes.Unimplemented, "Watch method not implemented")
}

func (s *Service) CreateVoucher(ctx context.Context, req *voucherv1.CreateVoucherRequest) (*voucherv1.Voucher, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}

	zapLog := zap.L().With(traceOpt...)

	voucher := req.Voucher
	v := Voucher{
		VoucherID:      s.node.Generate().String(),
		TenantID:       req.GetTenantId(),
		CampaignID:     voucher.GetCampaignId(),
		Code:           voucher.GetCode(),
		Name:           voucher.GetName(),
		DiscountType:   voucher.GetDiscountType(),
		DiscountValue:  voucher.GetDiscountValue(),
		Currency:       voucher.GetCurrency(),
		Stock:          int32(voucher.GetStock()),
		RemainingStock: int32(voucher.GetRemainingStock()),
		IsActive:       voucher.GetIsActive(),
	}

	if err := s.voucher.Create(ctx, &v); err != nil {
		zapLog.Error("failed create voucher", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed create voucher")
	}

	return v.ToProto(), nil
}

// --- GetVoucher ---

func (s *Service) GetVoucher(ctx context.Context, req *voucherv1.GetVoucherRequest) (*voucherv1.Voucher, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
		zap.String("tenant_id", req.GetTenantId()),
		zap.String("voucher_code", req.GetCode()),
	}

	zapLog := zap.L().With(traceOpt...)

	voucher, err := s.voucher.FindOne(ctx, &Voucher{
		TenantID: req.GetTenantId(),
		Code:     req.GetCode(),
	})
	if err != nil {
		zapLog.Error("failed to check voucher", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to check voucher")
	}

	if voucher == nil {
		zapLog.Error("voucher not found")
		return nil, status.Error(codes.InvalidArgument, "voucher not found")
	}

	return voucher.ToProto(), nil
}

func (s *Service) ListVouchers(ctx context.Context, req *voucherv1.ListVouchersRequest) (*voucherv1.ListVouchersResponse, error) {
	span := trace.SpanFromContext(ctx)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	traceOpt := []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
		zap.String("tenant_id", req.GetTenantId()),
		zap.Bool("only_active", req.GetOnlyActive()),
	}

	zapLog := zap.L().With(traceOpt...)

	opts := []option.QueryOption{
		option.ApplyPagination(
			pagination.Pagination{
				Limit: int(req.Limit),
			},
		),
	}

	if req.OnlyActive {
		onlyOption := []option.QueryOption{
			option.ApplyOperator(
				option.Condition{
					Operator: option.OR,
					Conditions: []option.Condition{
						{
							Field:    "expiry_date",
							Operator: option.ISNULL,
						},
						{
							Field:    "expiry_date",
							Operator: option.GT,
							Value:    time.Now(),
						},
					},
				},
			),
		}
		opts = append(opts, onlyOption...)
	}

	vouchers, err := s.voucher.Find(ctx, &Voucher{
		TenantID: req.TenantId,
		IsActive: true,
	}, opts...)
	if err != nil {
		zapLog.Error("failed get list voucher", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed get list voucher")
	}

	resp := &voucherv1.ListVouchersResponse{}
	for _, v := range vouchers {
		resp.Vouchers = append(resp.Vouchers, v.ToProto())
	}

	return resp, nil
}

// --- EvaluateVouchers ---
func (s *Service) EvaluateVouchers(ctx context.Context, req *voucherv1.EvaluateVouchersRequest) (*voucherv1.EvaluateVouchersResponse, error) {
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

	var vouchers []Voucher
	if err := s.db.WithContext(ctx).
		Preload("Campaign").
		Where("tenant_id = ? AND is_active = TRUE AND (expiry_date IS NULL OR expiry_date > ?)",
			req.TenantId, time.Now()).
		Find(&vouchers).Error; err != nil {
		return nil, err
	}

	attrs := make(map[string]interface{})
	for k, v := range req.Context.Fields {
		attrs[k] = v
	}

	env, err := celengine.BuildCelEnvFromAttributes(attrs)
	if err != nil {
		zapLog.Error("failed to build CEL environment", zap.Error(err))
		return nil, err
	}

	results := make([]*voucherv1.VoucherEligibilityResult, 0, len(vouchers))
	for _, v := range vouchers {
		expr := v.Campaign.DslExpression.String()
		if expr == "" || expr == "{}" {
			results = append(results, &voucherv1.VoucherEligibilityResult{
				VoucherCode: v.Code,
				Eligible:    true, // no rule = always eligible
				Reason:      "no dsl_expression defined",
			})
			continue
		}

		var expStr string
		if json.Valid([]byte(expr)) {
			_ = json.Unmarshal([]byte(expr), &expStr)
			if expStr == "" {
				expStr = expr
			}
		} else {
			expStr = expr
		}

		ok, err := celengine.Evaluate(env, expStr, attrs)
		res := &voucherv1.VoucherEligibilityResult{
			VoucherCode: v.Code,
			Eligible:    ok,
		}

		if err != nil {
			zapLog.Warn("failed to evaluate CEL", zap.String("voucher_code", v.Code), zap.Error(err))
			res.Reason = err.Error()
		} else if !ok {
			res.Reason = "condition not met"
		}

		results = append(results, res)
	}

	return &voucherv1.EvaluateVouchersResponse{Results: results}, nil
}

// --- IssueVoucher ---
func (s *Service) IssueVoucher(ctx context.Context, req *voucherv1.IssueVoucherRequest) (*voucherv1.VoucherIssuance, error) {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var v Voucher
	if err := tx.
		Where("tenant_id = ? AND code = ? AND is_active = TRUE", req.TenantId, req.VoucherCode).
		First(&v).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Decrement stock atomically
	res := tx.Model(&Voucher{}).
		Where("voucher_id = ? AND remaining_stock > 0", v.VoucherID).
		Update("remaining_stock", gorm.Expr("remaining_stock - 1"))
	if res.Error != nil || res.RowsAffected == 0 {
		tx.Rollback()
		return nil, fmt.Errorf("voucher out of stock")
	}

	issuance := VoucherIssuance{
		IssuanceID: s.node.Generate().String(),
		TenantID:   req.TenantId,
		VoucherID:  v.VoucherID,
		UserID:     req.UserId,
		Status:     voucherv1.IssuanceStatus_ISSUED,
		IssuedAt:   time.Now(),
	}
	if err := tx.Create(&issuance).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return issuance.ToProto(), nil
}

// --- RedeemVoucher ---
func (s *Service) RedeemVoucher(ctx context.Context, req *voucherv1.RedeemVoucherRequest) (*voucherv1.VoucherIssuance, error) {

	opts := []option.QueryOption{
		option.WithPreloads(
			"Voucher",
			func(tx *gorm.DB) *gorm.DB {
				return tx.Where("code = ?", req.GetVoucherCode())
			},
		),
	}

	iss, err := s.voucherIssue.FindOne(ctx, &VoucherIssuance{
		TenantID: req.TenantId,
		UserID:   req.UserId,
		Status:   voucherv1.IssuanceStatus_ISSUED,
	}, opts...)
	if err != nil {
		zap.L().Error("failed redeem voucher", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed redeem voucher")
	}

	if iss == nil {
		return nil, status.Error(codes.InvalidArgument, "voucher not found")
	}

	if err := s.voucherIssue.Update(ctx, iss.IssuanceID, map[string]interface{}{
		"status":      Redeemed,
		"redeemed_at": time.Now(),
	}); err != nil {
		return nil, err
	}

	return iss.ToProto(), nil
}
