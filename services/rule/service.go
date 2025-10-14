package rule

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	common "github.com/smallbiznis/go-genproto/smallbiznis/common"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	metadataOrgIDKey = "x-org-id"
	defaultPageSize  = 20
	maxPageSize      = 100
)

// Service implements rulev1.RuleServiceServer.
type Service struct {
	rulev1.UnimplementedRuleServiceServer

	repo      Repository
	evaluator *Evaluator
	logger    *zap.Logger
	node      *snowflake.Node
}

// ServiceParams defines dependencies for Service construction.
type ServiceParams struct {
	fx.In

	Repository Repository
	Evaluator  *Evaluator
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
	if p.Evaluator == nil {
		p.Evaluator = NewEvaluator()
	}
	return &Service{
		repo:      p.Repository,
		evaluator: p.Evaluator,
		logger:    logger,
		node:      p.Node,
	}
}

// CreateRule handles the CreateRule RPC.
func (s *Service) CreateRule(ctx context.Context, req *rulev1.CreateRuleRequest) (*rulev1.Rule, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	orgID, err := orgIDFromContext(ctx)
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
		TenantID:      orgID,
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

	return protoRule, nil
}

// GetRule handles the GetRule RPC.
func (s *Service) GetRule(ctx context.Context, req *rulev1.GetRuleRequest) (*rulev1.Rule, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}
	orgID, err := orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rule, err := s.repo.GetByID(ctx, orgID, req.GetRuleId())
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

	return protoRule, nil
}

// ListRules handles the ListRules RPC.
func (s *Service) ListRules(ctx context.Context, req *rulev1.ListRulesRequest) (*rulev1.ListRulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	orgID, err := orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	limit := defaultPageSize
	cursor := ""
	if req.GetPaging() != nil {
		cursor = req.GetPaging().GetCursor()
		if req.GetPaging().GetLimit() > 0 {
			limit = int(req.GetPaging().GetLimit())
		}
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

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
		IncludeInactive: req.GetIncludeInactive(),
		Triggers:        req.GetTriggers(),
	}

	rules, err := s.repo.List(ctx, orgID, params)
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

	paging := &common.CursorResponse{
		NextCursor: nextCursor,
		HasMore:    hasMore,
		TotalCount: int32(len(protoRules)),
	}

	return &rulev1.ListRulesResponse{
		Rules:  protoRules,
		Paging: paging,
	}, nil
}

// UpdateRule handles the UpdateRule RPC.
func (s *Service) UpdateRule(ctx context.Context, req *rulev1.UpdateRuleRequest) (*rulev1.Rule, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}
	orgID, err := orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, orgID, req.GetRuleId())
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

	return protoRule, nil
}

// DeleteRule handles the DeleteRule RPC.
func (s *Service) DeleteRule(ctx context.Context, req *rulev1.DeleteRuleRequest) (*rulev1.DeleteRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	if strings.TrimSpace(req.GetRuleId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}
	orgID, err := orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Delete(ctx, orgID, req.GetRuleId()); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	} else if err != nil {
		s.logger.Error("failed to delete rule", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to delete rule")
	}

	return &rulev1.DeleteRuleResponse{}, nil
}

// EvaluateRules handles the EvaluateRules RPC.
func (s *Service) EvaluateRules(ctx context.Context, req *rulev1.EvaluateRuleRequest) (*rulev1.EvaluateRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}
	orgID, err := orgIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	trigger, err := parseTrigger(req.GetTrigger())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	rules, err := s.repo.ListActiveByTrigger(ctx, orgID, trigger)
	if err != nil {
		s.logger.Error("failed to load rules for evaluation", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to evaluate rules")
	}

	ctxMap := convertAttributes(req.GetAttributes())

	matchedRules := make([]*rulev1.Rule, 0)
	aggregatedActions := make([]*rulev1.RuleAction, 0)
	metadata := make(map[string]string)
	var totalPoints int32
	var totalDiscount float64
	var vouchers []string

	for _, rule := range rules {
		matched, evalErr := s.evaluator.Evaluate(rule.DSLExpression, ctxMap)
		if evalErr != nil {
			s.logger.Warn("rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.Error(evalErr))
			continue
		}
		if !matched {
			continue
		}

		protoRule, err := rule.ToProto()
		if err != nil {
			s.logger.Error("failed to convert matched rule", zap.String("rule_id", rule.RuleID), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to marshal matched rule")
		}
		matchedRules = append(matchedRules, protoRule)

		actions, err := rule.ActionsList()
		if err != nil {
			s.logger.Error("failed to decode actions", zap.String("rule_id", rule.RuleID), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to evaluate rule actions")
		}
		protoActions, err := ProtoActionsFromModel(actions)
		if err != nil {
			s.logger.Error("failed to convert actions", zap.String("rule_id", rule.RuleID), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to evaluate rule actions")
		}
		aggregatedActions = append(aggregatedActions, protoActions...)

		accumulateMetrics(actions, &totalPoints, &totalDiscount, &vouchers, metadata)
	}

	if len(metadata) == 0 {
		metadata = nil
	}

	return &rulev1.EvaluateRuleResponse{
		MatchedRules:      matchedRules,
		Actions:           aggregatedActions,
		TotalPoints:       totalPoints,
		TotalDiscount:     totalDiscount,
		GeneratedVouchers: vouchers,
		Metadata:          metadata,
	}, nil
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
		return "", status.Error(codes.InvalidArgument, "missing organisation context")
	}
	values := md.Get(metadataOrgIDKey)
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return "", status.Error(codes.InvalidArgument, "missing organisation context")
	}
	return values[0], nil
}

func parseTrigger(raw string) (rulev1.RuleTriggerType, error) {
	if strings.TrimSpace(raw) == "" {
		return rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_UNSPECIFIED, fmt.Errorf("trigger is required")
	}
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if !strings.HasPrefix(normalized, "RULE_TRIGGER_TYPE_") {
		normalized = "RULE_TRIGGER_TYPE_" + normalized
	}
	if v, ok := rulev1.RuleTriggerType_value[normalized]; ok {
		return rulev1.RuleTriggerType(v), nil
	}
	if idx, err := strconv.Atoi(raw); err == nil {
		return rulev1.RuleTriggerType(idx), nil
	}
	return rulev1.RuleTriggerType_RULE_TRIGGER_TYPE_UNSPECIFIED, fmt.Errorf("unknown trigger %q", raw)
}

func convertAttributes(attrs map[string]string) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		if iv, err := strconv.ParseInt(v, 10, 64); err == nil {
			out[k] = iv
			continue
		}
		if uv, err := strconv.ParseUint(v, 10, 64); err == nil {
			out[k] = uv
			continue
		}
		if fv, err := strconv.ParseFloat(v, 64); err == nil {
			out[k] = fv
			continue
		}
		if bv, err := strconv.ParseBool(v); err == nil {
			out[k] = bv
			continue
		}
		out[k] = v
	}
	return out
}

func accumulateMetrics(actions []RuleAction, points *int32, discount *float64, vouchers *[]string, metadata map[string]string) {
	for _, action := range actions {
		switch action.Type {
		case rulev1.RuleActionType_RULE_ACTION_TYPE_EARN_POINT:
			if action.PointAction != nil {
				*points += action.PointAction.Points
				mergeMetadata(metadata, action.PointAction.Metadata)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_REDEEM_POINT:
			if action.PointAction != nil {
				*points -= action.PointAction.Points
				mergeMetadata(metadata, action.PointAction.Metadata)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_CASHBACK:
			if action.CashbackAction != nil {
				*discount += action.CashbackAction.Amount
				mergeMetadata(metadata, action.CashbackAction.Metadata)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_ISSUE_VOUCHER:
			if action.VoucherAction != nil {
				*discount += action.VoucherAction.DiscountValue
				if action.VoucherAction.VoucherCode != "" {
					*vouchers = append(*vouchers, action.VoucherAction.VoucherCode)
				}
				mergeMetadata(metadata, action.VoucherAction.Metadata)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_NOTIFY:
			if action.NotifyAction != nil {
				mergeMetadata(metadata, action.NotifyAction.Metadata)
			}
		case rulev1.RuleActionType_RULE_ACTION_TYPE_TAG_CUSTOMER:
			if action.TagAction != nil && action.TagAction.TagKey != "" {
				metadata[action.TagAction.TagKey] = action.TagAction.TagValue
			}
		default:
			if action.PointAction != nil {
				mergeMetadata(metadata, action.PointAction.Metadata)
			}
		}
	}
}

func mergeMetadata(dst map[string]string, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
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
