package rule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"smallbiznis-controlplane/pkg/celengine"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

const (
	metadataTenantIDKey = "x-tenant-id"
	defaultPageSize     = 20
	maxPageSize         = 100
)

// Service implements rulev1.RuleServiceServer.
type Service struct {
	rulev1.UnimplementedRuleServiceServer

	repo   Repository
	logger *zap.Logger
	node   *snowflake.Node
}

// ServiceParams defines dependencies for Service construction.
type ServiceParams struct {
	fx.In

	Repository Repository
	Logger     *zap.Logger
	Node       *snowflake.Node
}

// NewService constructs a new Service instance.
func NewService(p ServiceParams) *Service {
	logger := p.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	if p.Repository == nil {
		panic("rule service requires repository dependency")
	}

	return &Service{
		repo:   p.Repository,
		logger: logger,
		node:   p.Node,
	}
}

// CreateRule handles the CreateRule RPC.
func (s *Service) CreateRule(ctx context.Context, req *rulev1.CreateRuleRequest) (*rulev1.CreateRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	tenantID, err := s.resolveTenantID(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if strings.TrimSpace(req.GetDslExpression()) == "" {
		return nil, status.Error(codes.InvalidArgument, "dsl_expression is required")
	}

	actions, err := ModelActionsFromProto(req.GetActions())
	if err != nil {
		s.logger.Error("failed to convert actions", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "invalid actions payload")
	}

	now := time.Now().UTC()
	rule := &Rule{
		RuleID:        s.nextRuleID(),
		TenantID:      tenantID,
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		IsActive:      req.GetIsActive(),
		Priority:      req.GetPriority(),
		Trigger:       req.GetTrigger(),
		DSLExpression: req.GetDslExpression(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := rule.SetActions(actions); err != nil {
		s.logger.Error("failed to encode actions", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "invalid actions payload")
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		s.logger.Error("failed to create rule", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to create rule")
	}

	protoRule, err := rule.ToProto()
	if err != nil {
		s.logger.Error("failed to convert rule to proto", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to marshal rule")
	}

	return &rulev1.CreateRuleResponse{Rule: protoRule}, nil
}

// GetRule handles the GetRule RPC.
func (s *Service) GetRule(ctx context.Context, req *rulev1.GetRuleRequest) (*rulev1.GetRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	tenantID, err := s.resolveTenantID(ctx, "")
	if err != nil {
		return nil, err
	}

	rule, err := s.repo.GetByID(ctx, tenantID, req.GetRuleId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	}
	if err != nil {
		s.logger.Error("failed to get rule", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get rule")
	}

	protoRule, err := rule.ToProto()
	if err != nil {
		s.logger.Error("failed to convert rule", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to marshal rule")
	}

	return &rulev1.GetRuleResponse{Rule: protoRule}, nil
}

// ListRules handles the ListRules RPC.
func (s *Service) ListRules(ctx context.Context, req *rulev1.ListRulesRequest) (*rulev1.ListRulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	tenantID, err := s.resolveTenantID(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	cursor := strings.TrimSpace(req.GetCursor())

	var afterPriority *int32
	afterRuleID := ""
	if cursor != "" {
		priority, ruleID, decodeErr := decodeCursor(cursor)
		if decodeErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid paging cursor")
		}
		afterPriority = &priority
		afterRuleID = ruleID
	}

	params := ListParams{
		AfterPriority:   afterPriority,
		AfterRuleID:     afterRuleID,
		Limit:           limit + 1,
		IncludeInactive: false,
		Triggers:        []rulev1.RuleTriggerType{},
	}

	rules, err := s.repo.List(ctx, tenantID, params)
	if err != nil {
		s.logger.Error("failed to list rules", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list rules")
	}

	hasMore := len(rules) > limit
	if hasMore {
		rules = rules[:limit]
	}

	protoRules := make([]*rulev1.Rule, 0, len(rules))
	for _, rule := range rules {
		protoRule, err := rule.ToProto()
		if err != nil {
			s.logger.Error("failed to convert rule", zap.String("rule_id", rule.RuleID), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to marshal rule")
		}
		protoRules = append(protoRules, protoRule)
	}

	var nextCursor string
	if hasMore && len(rules) > 0 {
		last := rules[len(rules)-1]
		nextCursor = encodeCursor(last.Priority, last.RuleID)
	}

	return &rulev1.ListRulesResponse{
		Rules:      protoRules,
		NextCursor: nextCursor,
	}, nil
}

// UpdateRule handles the UpdateRule RPC.
func (s *Service) UpdateRule(ctx context.Context, req *rulev1.UpdateRuleRequest) (*rulev1.UpdateRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	tenantID, err := s.resolveTenantID(ctx, "")
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, tenantID, req.GetRuleId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	}
	if err != nil {
		s.logger.Error("failed to fetch rule", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to update rule")
	}

	actions, err := ModelActionsFromProto(req.GetActions())
	if err != nil {
		s.logger.Error("failed to convert actions", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "invalid actions payload")
	}

	existing.Name = req.GetName()
	existing.Description = req.GetDescription()
	existing.IsActive = req.GetIsActive()
	if statusValue := req.GetStatus(); statusValue != rulev1.RuleStatus_RULE_STATUS_UNSPECIFIED {
		existing.IsActive = statusValue == rulev1.RuleStatus_RULE_STATUS_ACTIVE
	}
	existing.Priority = req.GetPriority()
	existing.Trigger = req.GetTrigger()
	existing.DSLExpression = req.GetDslExpression()
	existing.UpdatedAt = time.Now().UTC()
	if err := existing.SetActions(actions); err != nil {
		s.logger.Error("failed to encode actions", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, "invalid actions payload")
	}

	if err := s.repo.Update(ctx, existing); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	} else if err != nil {
		s.logger.Error("failed to update rule", zap.String("rule_id", existing.RuleID), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to update rule")
	}

	protoRule, err := existing.ToProto()
	if err != nil {
		s.logger.Error("failed to convert rule", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to marshal rule")
	}

	return &rulev1.UpdateRuleResponse{Rule: protoRule}, nil
}

// DeleteRule handles the DeleteRule RPC.
func (s *Service) DeleteRule(ctx context.Context, req *rulev1.DeleteRuleRequest) (*rulev1.DeleteRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	tenantID, err := s.resolveTenantID(ctx, "")
	if err != nil {
		return nil, err
	}

	if err := s.repo.Delete(ctx, tenantID, req.GetRuleId()); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	} else if err != nil {
		s.logger.Error("failed to delete rule", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to delete rule")
	}

	return &rulev1.DeleteRuleResponse{Success: true}, nil
}

// EvaluateRule handles the EvaluateRule RPC.
func (s *Service) EvaluateRule(ctx context.Context, req *rulev1.EvaluateRuleRequest) (*rulev1.EvaluateRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	tenantID, err := s.resolveTenantID(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	rule, err := s.repo.GetByID(ctx, tenantID, req.GetRuleId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	}
	if err != nil {
		s.logger.Error("failed to load rule for evaluation", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to evaluate rule")
	}

	ctxMap := structToMap(req.GetContext())

	matched, actionValue, evalErr := s.executeRule(rule, ctxMap)
	if evalErr != nil {
		s.logger.Error("rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.Error(evalErr))
		return nil, status.Error(codes.Internal, "failed to evaluate rule")
	}

	return &rulev1.EvaluateRuleResponse{
		Matched:     matched,
		ActionValue: actionValue,
	}, nil
}

// BatchEvaluate handles the BatchEvaluate RPC.
func (s *Service) BatchEvaluate(ctx context.Context, req *rulev1.BatchEvaluateRequest) (*rulev1.BatchEvaluateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	tenantID, err := s.resolveTenantID(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	ctxMap := structToMap(req.GetContext())

	results, evalErr := s.evaluateRulesBatch(ctx, tenantID, req.GetRuleIds(), ctxMap)
	if evalErr != nil {
		s.logger.Error("batch evaluation failed", zap.Error(evalErr))
		return nil, status.Error(codes.Internal, "failed to evaluate rules")
	}

	var totalMatched int32
	for _, res := range results {
		if res.GetStatus() == rulev1.EvaluationStatus_EVALUATION_STATUS_SUCCESS && res.GetMatched() {
			totalMatched++
		}
	}

	return &rulev1.BatchEvaluateResponse{
		Results:      results,
		TotalMatched: totalMatched,
		TotalRules:   int32(len(results)),
		ExecutionId:  s.newExecutionID(),
		TenantId:     tenantID,
	}, nil
}

// StreamEvaluate handles the bidirectional streaming evaluation RPC.
func (s *Service) StreamEvaluate(stream rulev1.RuleService_StreamEvaluateServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		tenantID, tenantErr := s.resolveTenantID(stream.Context(), req.GetTenantId())
		if tenantErr != nil {
			return tenantErr
		}

		ctxMap := structToMap(req.GetContext())
		results, evalErr := s.evaluateRulesBatch(stream.Context(), tenantID, req.GetRuleIds(), ctxMap)
		if evalErr != nil {
			s.logger.Error("stream evaluation batch failed", zap.Error(evalErr))
			return status.Error(codes.Internal, "failed to evaluate rules")
		}

		for _, res := range results {
			if sendErr := stream.Send(&rulev1.StreamEvaluateResponse{Result: res}); sendErr != nil {
				return sendErr
			}
		}
	}
}

func (s *Service) evaluateRulesBatch(ctx context.Context, tenantID string, ruleIDs []string, ctxMap map[string]any) ([]*rulev1.RuleEvaluationResult, error) {
	results := make([]*rulev1.RuleEvaluationResult, 0)

	if len(ruleIDs) > 0 {
		for _, rawID := range ruleIDs {
			ruleID := strings.TrimSpace(rawID)
			if ruleID == "" {
				continue
			}

			rule, err := s.repo.GetByID(ctx, tenantID, ruleID)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				results = append(results, &rulev1.RuleEvaluationResult{
					RuleId:       ruleID,
					Status:       rulev1.EvaluationStatus_EVALUATION_STATUS_ERROR,
					ErrorMessage: "rule not found",
				})
				continue
			}
			if err != nil {
				return nil, err
			}

			results = append(results, s.evaluateRuleResult(rule, ctxMap))
		}
		return results, nil
	}

	listParams := ListParams{
		Limit:           0,
		IncludeInactive: false,
		Triggers:        []rulev1.RuleTriggerType{},
	}

	rules, err := s.repo.List(ctx, tenantID, listParams)
	if err != nil {
		return nil, err
	}

	for i := range rules {
		results = append(results, s.evaluateRuleResult(&rules[i], ctxMap))
	}

	return results, nil
}

func (s *Service) evaluateRuleResult(rule *Rule, ctxMap map[string]any) *rulev1.RuleEvaluationResult {
	result := &rulev1.RuleEvaluationResult{
		RuleId: rule.RuleID,
		Status: rulev1.EvaluationStatus_EVALUATION_STATUS_SUCCESS,
	}

	matched, actionValue, err := s.executeRule(rule, ctxMap)
	if err != nil {
		s.logger.Error("rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.Error(err))
		result.Status = rulev1.EvaluationStatus_EVALUATION_STATUS_ERROR
		result.ErrorMessage = "failed to evaluate rule"
		return result
	}

	result.Matched = matched
	if matched {
		result.ActionValue = actionValue
	}

	return result
}

func (s *Service) executeRule(rule *Rule, ctxMap map[string]any) (bool, *structpb.Struct, error) {

	env, err := celengine.BuildCelEnvFromAttributes(ctxMap)
	if err != nil {
		return false, nil, err
	}

	matched, err := celengine.Evaluate(env, rule.DSLExpression, ctxMap)
	if err != nil {
		return false, nil, err
	}

	if !matched {
		return false, nil, nil
	}

	actions, err := rule.ActionsList()
	if err != nil {
		return false, nil, err
	}

	summary, err := buildActionSummary(actions)
	if err != nil {
		return false, nil, err
	}

	return true, summary, nil
}

func buildActionSummary(actions []RuleAction) (*structpb.Struct, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	var totalPoints int64
	var totalCashback float64
	var totalVoucherDiscount float64

	vouchers := make([]any, 0)
	notifications := make([]any, 0)
	tags := make(map[string]string)
	metadata := make(map[string]string)
	actionEntries := make([]any, 0, len(actions))

	for _, action := range actions {
		entry := map[string]any{
			"type": action.Type.String(),
		}

		switch action.Type {
		case rulev1.RuleActionType_RULE_ACTION_TYPE_REWARD_POINT:
			if payload := action.PointAction; payload != nil {
				entry["points"] = payload.Points
				if payload.Reference != "" {
					entry["reference"] = payload.Reference
				}
				if len(payload.Metadata) > 0 {
					entry["metadata"] = stringMapToAny(payload.Metadata)
					metadata = mergeStringMaps(metadata, payload.Metadata)
				}
				totalPoints += payload.Points
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_VOUCHER:
			if payload := action.VoucherAction; payload != nil {
				detail := map[string]any{
					"voucher_code":   payload.VoucherCode,
					"discount_value": payload.DiscountValue,
					"discount_type":  payload.DiscountType,
				}
				if payload.ExpiryDate != nil {
					detail["expiry_date"] = payload.ExpiryDate.UTC().Format(time.RFC3339)
				}
				if len(payload.Metadata) > 0 {
					detail["metadata"] = stringMapToAny(payload.Metadata)
					metadata = mergeStringMaps(metadata, payload.Metadata)
				}
				vouchers = append(vouchers, detail)
				entry["voucher"] = detail
				totalVoucherDiscount += payload.DiscountValue
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_CASHBACK:
			if payload := action.CashbackAction; payload != nil {
				detail := map[string]any{
					"amount": payload.Amount,
				}
				if payload.Currency != "" {
					detail["currency"] = payload.Currency
				}
				if payload.TargetWalletID != "" {
					detail["target_wallet_id"] = payload.TargetWalletID
				}
				if len(payload.Metadata) > 0 {
					detail["metadata"] = stringMapToAny(payload.Metadata)
					metadata = mergeStringMaps(metadata, payload.Metadata)
				}
				totalCashback += payload.Amount
				entry["cashback"] = detail
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_NOTIFY:
			if payload := action.NotifyAction; payload != nil {
				detail := map[string]any{}
				if payload.Channel != "" {
					detail["channel"] = payload.Channel
				}
				if payload.TemplateID != "" {
					detail["template_id"] = payload.TemplateID
				}
				if len(payload.Metadata) > 0 {
					detail["metadata"] = stringMapToAny(payload.Metadata)
					metadata = mergeStringMaps(metadata, payload.Metadata)
				}
				entry["notification"] = detail
				notifications = append(notifications, detail)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_TAG:
			if payload := action.TagAction; payload != nil {
				tag := map[string]any{
					"tag_key":   payload.TagKey,
					"tag_value": payload.TagValue,
				}
				entry["tag"] = tag
				if payload.TagKey != "" {
					tags[payload.TagKey] = payload.TagValue
				}
			}
		default:
			if payload := action.PointAction; payload != nil && len(payload.Metadata) > 0 {
				metadata = mergeStringMaps(metadata, payload.Metadata)
			}
		}

		actionEntries = append(actionEntries, entry)
	}

	summary := make(map[string]any)

	if totalPoints != 0 {
		summary["total_reward_points"] = totalPoints
	}
	if totalCashback != 0 {
		summary["total_cashback_amount"] = totalCashback
	}
	if totalVoucherDiscount != 0 {
		summary["total_voucher_discount"] = totalVoucherDiscount
	}
	if len(vouchers) > 0 {
		summary["vouchers"] = vouchers
	}
	if len(notifications) > 0 {
		summary["notifications"] = notifications
	}
	if len(tags) > 0 {
		summary["tags"] = stringMapToAny(tags)
	}
	if len(metadata) > 0 {
		summary["metadata"] = stringMapToAny(metadata)
	}
	if len(actionEntries) > 0 {
		summary["actions"] = actionEntries
	}
	summary["actions_count"] = int64(len(actionEntries))

	if len(actionEntries) == 0 && len(summary) == 1 {
		return nil, nil
	}

	return structpb.NewStruct(summary)
}

func stringMapToAny(src map[string]string) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func mergeStringMaps(dst map[string]string, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

func (s *Service) resolveTenantID(ctx context.Context, candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate != "" {
		return candidate, nil
	}
	return orgIDFromContext(ctx)
}

func (s *Service) newExecutionID() string {
	return fmt.Sprintf("exec-%s", s.nextRuleID())
}

func (s *Service) nextRuleID() string {
	if s.node == nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	id := s.node.Generate()
	return id.String()
}

func orgIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	values := md.Get(metadataTenantIDKey)
	if len(values) == 0 {
		return "", status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	if candidate := strings.TrimSpace(values[0]); candidate != "" {
		return candidate, nil
	}

	return "", status.Error(codes.InvalidArgument, "tenant_id is required")
}

func encodeCursor(priority int32, ruleID string) string {
	return fmt.Sprintf("%d:%s", priority, ruleID)
}

func decodeCursor(cursor string) (int32, string, error) {
	parts := strings.SplitN(cursor, ":", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid cursor format")
	}
	value, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, "", err
	}
	return int32(value), parts[1], nil
}
