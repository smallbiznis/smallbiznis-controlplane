package rule

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/snowflake"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	rulev1 "github.com/smallbiznis/smallbiznisapis/smallbiznis/rule/v1"
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
		panic("rule service requires a repository dependency")
	}

	evaluator := p.Evaluator
	if evaluator == nil {
		evaluator = NewEvaluator()
	}

	return &Service{
		repo:      p.Repository,
		evaluator: evaluator,
		logger:    logger,
		node:      p.Node,
	}
}

// EvaluateRule evaluates a single rule by ID.
func (s *Service) EvaluateRule(ctx context.Context, req *rulev1.EvaluateRuleRequest) (*rulev1.EvaluateRuleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetTenantId() == "" || req.GetRuleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and rule_id are required")
	}

	rule, err := s.repo.GetByID(ctx, req.GetTenantId(), req.GetRuleId())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "rule not found")
	}
	if err != nil {
		s.logger.Error("failed to fetch rule", zap.String("rule_id", req.GetRuleId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to fetch rule")
	}

	contextMap := structToMap(req.GetContext())
	matched, evalErr := s.evaluator.Evaluate(rule.Expression, contextMap)
	if evalErr != nil {
		s.logger.Warn("rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.Error(evalErr))
		return &rulev1.EvaluateRuleResponse{Error: evalErr.Error()}, nil
	}

	response := &rulev1.EvaluateRuleResponse{Matched: matched}
	if matched {
		actionValue, err := jsonMapToStruct(rule.ActionValue)
		if err != nil {
			s.logger.Error("failed to marshal action value", zap.String("rule_id", rule.RuleID), zap.Error(err))
			return nil, status.Error(codes.Internal, "failed to marshal action value")
		}
		response.ActionValue = actionValue
	}

	return response, nil
}

// EvaluateRules evaluates a list of rules provided in the request payload.
func (s *Service) EvaluateRules(ctx context.Context, req *rulev1.EvaluateRulesRequest) (*rulev1.EvaluateRulesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	contextMap := structToMap(req.GetContext())
	responses := make([]*rulev1.EvaluateRuleResponse, 0, len(req.GetRules()))
	for _, rule := range req.GetRules() {
		matched, err := s.evaluator.Evaluate(rule.GetExpression(), contextMap)
		res := &rulev1.EvaluateRuleResponse{Matched: matched}
		if err != nil {
			res.Matched = false
			res.Error = err.Error()
		} else if matched {
			res.ActionValue = rule.GetActionValue()
		}
		responses = append(responses, res)
	}

	return &rulev1.EvaluateRulesResponse{Results: responses}, nil
}

// BatchEvaluate evaluates all active rules for a tenant.
func (s *Service) BatchEvaluate(ctx context.Context, req *rulev1.BatchEvaluateRequest) (*rulev1.BatchEvaluateResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	rules, err := s.repo.ListByTenant(ctx, req.GetTenantId(), req.GetIncludeDisabled(), req.GetRuleIds())
	if err != nil {
		s.logger.Error("failed to list rules", zap.String("tenant_id", req.GetTenantId()), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to list rules")
	}

	contextMap := structToMap(req.GetContext())
	results := make([]*rulev1.RuleEvaluationResult, 0, len(rules))
	var matchedCount int32
	for _, rule := range rules {
		matched, evalErr := s.evaluator.Evaluate(rule.Expression, contextMap)
		result := &rulev1.RuleEvaluationResult{
			RuleId: rule.RuleID,
		}

		if evalErr != nil {
			result.Status = rulev1.EvaluationStatus_EVALUATION_STATUS_ERROR
			result.ErrorMessage = evalErr.Error()
			s.logger.Warn("batch rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.Error(evalErr))
		} else {
			result.Status = rulev1.EvaluationStatus_EVALUATION_STATUS_SUCCESS
			result.Matched = matched
			if matched {
				matchedCount++
				actionValue, err := jsonMapToStruct(rule.ActionValue)
				if err != nil {
					s.logger.Error("failed to marshal action value", zap.String("rule_id", rule.RuleID), zap.Error(err))
					result.Status = rulev1.EvaluationStatus_EVALUATION_STATUS_ERROR
					result.Matched = false
					result.ErrorMessage = fmt.Sprintf("failed to marshal action value: %v", err)
				} else {
					result.ActionValue = actionValue
				}
			}
		}

		results = append(results, result)
	}

	response := &rulev1.BatchEvaluateResponse{
		Results:      results,
		TotalMatched: matchedCount,
		TotalRules:   int32(len(rules)),
		ExecutionId:  s.executionID(),
		TenantId:     req.GetTenantId(),
	}

	return response, nil
}

func (s *Service) executionID() string {
	if s.node == nil {
		return ""
	}
	return s.node.Generate().String()
}

func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return s.AsMap()
}

func jsonMapToStruct(data datatypes.JSONMap) (*structpb.Struct, error) {
	if len(data) == 0 {
		return nil, nil
	}
	return structpb.NewStruct(map[string]any(data))
}
