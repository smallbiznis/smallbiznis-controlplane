package rule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"smallbiznis-controlplane/pkg/celengine"
	"smallbiznis-controlplane/pkg/repository"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/google/cel-go/cel"
	"github.com/redis/go-redis/v9"
	rulev1 "github.com/smallbiznis/go-genproto/smallbiznis/rule/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

// Cache global: rules & env
var (
	rulesByTrigger = sync.Map{} // map[string][]*CompiledRule
	loadGroup      singleflight.Group
)

// Service implements rulev1.RuleServiceServer.
type Service struct {
	rulev1.UnimplementedRuleServiceServer
	grpc_health_v1.UnimplementedHealthServer

	db     *gorm.DB
	rdb    *redis.Client
	node   *snowflake.Node
	logger *zap.Logger

	cache *RuleCache

	repo    Repository
	ruleSet repository.Repository[RuleSet]
}

// ServiceParams defines dependencies for Service construction.
type ServiceParams struct {
	fx.In

	DB     *gorm.DB
	RDB    *redis.Client
	Node   *snowflake.Node
	Logger *zap.Logger

	Repository Repository
}

// NewService constructs a new Service instance.
func NewService(lc fx.Lifecycle, p ServiceParams) *Service {
	logger := p.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	if p.Repository == nil {
		panic("rule service requires repository dependency")
	}

	svc := &Service{
		db:     p.DB,
		rdb:    p.RDB,
		node:   p.Node,
		logger: logger,

		cache: NewRuleCache(10 * time.Minute),

		repo:    p.Repository,
		ruleSet: repository.ProvideStore[RuleSet](p.DB),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go svc.startRuleInvalidationSubscriber(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})

	return svc
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
		Trigger:       req.GetTrigger().String(),
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
		limit = 10
	}

	if limit > 250 {
		limit = 250
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
		Triggers:        []string{},
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
		existing.IsActive = statusValue == rulev1.RuleStatus_ACTIVE
	}
	existing.Priority = req.GetPriority()
	existing.Trigger = req.GetTrigger().String()
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

	attrs := structToMap(req.GetContext())
	matched, actionValue, evalErr := s.executeRule(rule, attrs)
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

	attrs := structToMap(req.GetContext())
	trigger := req.Trigger.String()

	// Ambil compiled rules dari cache
	rules, err := s.GetCompiledOrRefresh(ctx, tenantID, trigger)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(rules) == 0 {
		// Singleflight biar concurrent request dengan trigger sama tidak compile bareng
		_, err, _ := loadGroup.Do(trigger, func() (any, error) {
			env, err := celengine.GetOrBuildEnv(attrs)
			if err != nil {
				return nil, err
			}

			allRules, err := s.repo.List(ctx, tenantID, ListParams{
				IncludeInactive: false,
				Triggers:        []string{req.Trigger.String()},
			})
			if err != nil {
				return nil, err
			}

			return nil, s.CompileAndCacheRules(env, allRules)
		})
		if err != nil {
			s.logger.Error("failed to compile rules", zap.Error(err))
			return nil, status.Errorf(codes.Internal, "failed to compile rules: %v", err)
		}

		rules, _ = s.GetCompiledOrRefresh(ctx, tenantID, trigger)
	}

	// Parallel evaluation
	results := make([]*rulev1.RuleEvaluationResult, 0, len(rules))
	resultCh := make(chan *rulev1.RuleEvaluationResult, len(rules))
	sem := make(chan struct{}, 10) // concurrency limit
	var wg sync.WaitGroup

	for _, compiled := range rules {
		sem <- struct{}{}
		wg.Add(1)

		go func(r *CompiledRule) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// Context cancel / timeout per rule
			ruleCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()

			select {
			case <-ruleCtx.Done():
				resultCh <- &rulev1.RuleEvaluationResult{
					RuleId:       r.ID,
					Status:       rulev1.EvaluationStatus_ERROR,
					ErrorMessage: "context canceled or timeout",
				}
				return
			default:
			}

			matched, err := r.evaluate(attrs)
			if err != nil {
				resultCh <- &rulev1.RuleEvaluationResult{
					RuleId:       r.ID,
					Status:       rulev1.EvaluationStatus_ERROR,
					ErrorMessage: err.Error(),
				}
				return
			}

			if matched {
				acts, _ := r.Rule.ActionsList()
				summary, err := s.buildActionSummary(acts)
				if err != nil {
					s.logger.Error("failed to build action summary", zap.Error(err))
				}

				resultCh <- &rulev1.RuleEvaluationResult{
					RuleId:      r.ID,
					Matched:     matched,
					ActionValue: summary,
					Status:      rulev1.EvaluationStatus_SUCCESS,
				}
			}

		}(compiled)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for r := range resultCh {
		results = append(results, r)
	}

	// 🔹 Hitung hasil evaluasi
	var totalMatched int32
	for _, r := range results {
		if r.Matched {
			totalMatched++
		}
	}

	s.logger.Info("batch evaluation completed",
		zap.String("tenant_id", tenantID),
		zap.String("trigger", trigger),
		zap.Int("total_rules", len(results)),
		zap.Int32("total_matched", totalMatched),
	)

	return &rulev1.BatchEvaluateResponse{
		TenantId:     tenantID,
		Results:      results,
		TotalMatched: totalMatched,
		TotalRules:   int32(len(results)),
		ExecutionId:  s.newExecutionID(),
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

		attrs := structToMap(req.GetContext())
		results, evalErr := s.evaluateRulesBatch(stream.Context(), tenantID, req.GetRuleIds(), attrs)
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

func (s *Service) CompileAndCacheRules(env *cel.Env, rules []Rule) error {
	if env == nil {
		return fmt.Errorf("cel environment is nil")
	}

	temp := make(map[string][]*CompiledRule)
	for _, r := range rules {
		if strings.TrimSpace(r.DSLExpression) == "" {
			continue
		}

		ast, issues := env.Compile(r.DSLExpression)
		if issues != nil && issues.Err() != nil {
			s.logger.Warn("invalid rule expr", zap.String("rule_id", r.RuleID), zap.Error(issues.Err()))
			continue
		}

		prog, err := env.Program(ast)
		if err != nil {
			s.logger.Error("failed to build cel program", zap.String("rule_id", r.RuleID), zap.Error(err))
			continue
		}

		trigger := r.Trigger
		temp[trigger] = append(temp[trigger], &CompiledRule{ID: r.RuleID, Rule: r, Program: prog})
	}

	for k, v := range temp {
		key := makeRuleKey(rules[0].TenantID, k)
		rulesByTrigger.Store(key, v)
		s.logger.Info("compiled & cached rules", zap.String("trigger", k), zap.Int("count", len(v)))
	}

	return nil
}

func (s *Service) GetRulesByTrigger(tenantID, trigger string) ([]*CompiledRule, bool) {
	key := makeRuleKey(tenantID, trigger)
	v, ok := rulesByTrigger.Load(key)
	if !ok {
		return nil, false
	}
	return v.([]*CompiledRule), true
}

func (s *Service) GetCompiledOrRefresh(ctx context.Context, tenantID, trigger string) ([]*CompiledRule, error) {
	key := RuleSetKey{TenantID: tenantID, Trigger: trigger}
	if cached, ok := s.cache.Get(key); ok && cached != nil && len(cached.Rules) > 0 {
		cacheHits.Inc()
		// Non-blocking version check setiap N detik (anti hot-path call Redis tiap request)
		if time.Since(cached.UpdatedAt) > 2*time.Second {
			go s.softVersionCheck(context.Background(), key, cached.Version, cached.Hash)
		}
		return cached.Rules, nil
	} else {
		cacheMiss.Inc()
	}

	// Cold start → blocking refresh (sekali, berkat singleflight)
	if err := s.refreshRuleSet(ctx, key, 0, ""); err != nil {
		return nil, err
	}
	if fresh, ok := s.cache.Get(key); ok && fresh != nil {
		return fresh.Rules, nil
	}
	return nil, status.Error(codes.NotFound, "no compiled rules available")
}

func (s *Service) softVersionCheck(ctx context.Context, key RuleSetKey, localVer int64, localHash string) {
	verKey := fmt.Sprintf("ruleset:v2:%s:%s:ver", key.TenantID, key.Trigger)
	hasKey := fmt.Sprintf("ruleset:v2:%s:%s:hash", key.TenantID, key.Trigger)
	redisVer, _ := s.rdb.Get(ctx, verKey).Int64()
	redisHash, _ := s.rdb.Get(ctx, hasKey).Result()
	if redisVer > localVer || (redisHash != "" && redisHash != localHash) {
		_ = s.refreshRuleSet(ctx, key, redisVer, redisHash) // revalidate async
	}
}

func (s *Service) refreshRuleSet(ctx context.Context, key RuleSetKey, expectedVer int64, expectedHash string) error {
	// Hindari thundering herd
	_, err, _ := s.cache.group.Do(fmt.Sprintf("%s:%s", key.TenantID, key.Trigger), func() (any, error) {
		// Double-check latest Redis version/hash
		verKey := fmt.Sprintf("ruleset:v2:%s:%s:ver", key.TenantID, key.Trigger)
		hasKey := fmt.Sprintf("ruleset:v2:%s:%s:hash", key.TenantID, key.Trigger)

		curVer, _ := s.rdb.Get(ctx, verKey).Int64()
		curHash, _ := s.rdb.Get(ctx, hasKey).Result()

		// Pakai yang paling baru dari Redis/param
		if curVer == 0 {
			curVer = expectedVer
		}
		if curHash == "" {
			curHash = expectedHash
		}

		// 1) Load rules dari DB
		rules, err := s.repo.List(ctx, key.TenantID, ListParams{
			IncludeInactive: false,
			Triggers:        []string{key.Trigger},
		})
		if err != nil {
			return nil, err
		}

		// 2) Recalc hash untuk verifikasi
		hash := CalcRuleSetHash(rules)
		if curHash != "" && curHash != hash {
			// Redis hash tidak sinkron dengan DB → update Redis biar konsisten
			pipe := s.rdb.TxPipeline()
			pipe.Set(ctx, hasKey, hash, 0)
			if curVer == 0 {
				pipe.Incr(ctx, verKey) // kalau belum ada, bump minimal sekali
			}
			_, _ = pipe.Exec(ctx)
		}

		// 3) Compile
		env, err := celengine.GetOrBuildEnv(nil) // atau dari context contoh
		if err != nil {
			return nil, err
		}

		if err := s.CompileAndCacheRules(env, rules); err != nil {
			return nil, err
		}

		// 4) Simpan cache lokal lengkap versi & hash
		comp, _ := s.GetRulesByTrigger(key.TenantID, key.Trigger) // ambil hasil compile
		s.cache.Set(key, &CompiledRuleSet{
			Version:   curVer,
			Hash:      hash,
			Rules:     comp,
			UpdatedAt: time.Now(),
		})
		return nil, nil
	})
	return err
}

func (s *Service) bumpVersionAndBroadcast(ctx context.Context, tenantID, trigger string) error {
	// 1) Ambil rules terbaru & normalisasi → hash
	rules, err := s.repo.List(ctx, tenantID, ListParams{
		IncludeInactive: false,
		Triggers:        []string{trigger},
	})
	if err != nil {
		return err
	}

	hash := CalcRuleSetHash(rules) // SHA256 dari "rule_id|priority|dsl|actions|..."
	verKey := fmt.Sprintf("ruleset:v2:%s:%s:ver", tenantID, trigger)
	hasKey := fmt.Sprintf("ruleset:v2:%s:%s:hash", tenantID, trigger)

	// 2) Cek hash lama agar idempotent
	oldHash, _ := s.rdb.Get(ctx, hasKey).Result()
	if oldHash == hash {
		// Tidak ada perubahan material → tidak perlu broadcast
		return nil
	}

	// 3) Bump version di Redis (atomic)
	ver, err := s.rdb.Incr(ctx, verKey).Result()
	if err != nil {
		return err
	}

	if err := s.rdb.Set(ctx, hasKey, hash, 0).Err(); err != nil {
		return err
	}

	// 4) Publish event
	msg := RuleSet{
		TenantID: tenantID,
		Trigger:  trigger,
		Version:  ver,
		Hash:     hash,
	}
	payload, _ := json.Marshal(msg)
	if err := s.rdb.Publish(ctx, "chan:ruleset:v2:invalidate", payload).Err(); err != nil {
		return err
	}

	// 5) (Opsional) update table rule_sets untuk audit
	_ = s.ruleSet.Save(ctx, msg)

	return nil
}

func (s *Service) startRuleInvalidationSubscriber(ctx context.Context) {
	go func() {
		sub := s.rdb.Subscribe(ctx, "chan:ruleset:v2:invalidate")
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				_ = sub.Close()
				return
			case m := <-ch:
				if m == nil {
					continue
				}
				var evt struct {
					TenantID string `json:"tenant_id"`
					Trigger  string `json:"trigger"`
					Version  int64  `json:"version"`
					Hash     string `json:"hash"`
				}
				if err := json.Unmarshal([]byte(m.Payload), &evt); err != nil {
					s.logger.Warn("invalid invalidate payload", zap.Error(err))
					continue
				}

				key := RuleSetKey{TenantID: evt.TenantID, Trigger: evt.Trigger}
				s.cache.Invalidate(key)

				// Prewarm async (tidak blok request)
				go func() {
					if err := s.refreshRuleSet(context.Background(), key, evt.Version, evt.Hash); err != nil {
						s.logger.Error("prewarm failed", zap.Error(err), zap.Any("key", key))
					}
				}()

				s.logger.Info("ruleset invalidated", zap.Any("key", key), zap.Int64("new_version", evt.Version))
			}
		}
	}()
}

func (s *Service) evaluateRulesBatch(ctx context.Context, tenantID string, ruleIDs []string, attrs map[string]any) ([]*rulev1.RuleEvaluationResult, error) {
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
					Status:       rulev1.EvaluationStatus_ERROR,
					ErrorMessage: "rule not found",
				})
				continue
			}
			if err != nil {
				return nil, err
			}

			results = append(results, s.evaluateRuleResult(rule, attrs))
		}
		return results, nil
	}

	listParams := ListParams{
		Limit:           0,
		IncludeInactive: false,
		Triggers:        []string{},
	}

	rules, err := s.repo.List(ctx, tenantID, listParams)
	if err != nil {
		return nil, err
	}

	env, err := celengine.GetOrBuildEnv(attrs)
	if err != nil {
		return nil, err
	}

	triggerMap := map[string][]*CompiledRule{}
	for _, r := range rules {
		ast, issues := env.Compile(r.DSLExpression)
		if issues != nil && issues.Err() != nil {
			zap.L().Warn("invalid rule expr", zap.String("rule_id", r.RuleID), zap.Error(issues.Err()))
			continue
		}

		prog, _ := env.Program(ast)
		triggerMap[r.Trigger] = append(triggerMap[r.Trigger], &CompiledRule{
			ID:      r.RuleID,
			Program: prog,
		})
	}

	for k, v := range triggerMap {
		rulesByTrigger.Store(k, v)
	}

	return results, nil
}

func (s *Service) evaluateRuleResult(rule *Rule, attrs map[string]any) *rulev1.RuleEvaluationResult {
	result := &rulev1.RuleEvaluationResult{
		RuleId: rule.RuleID,
		Status: rulev1.EvaluationStatus_SUCCESS,
	}

	matched, actionValue, err := s.executeRule(rule, attrs)
	if err != nil {
		s.logger.Error("rule evaluation failed", zap.String("rule_id", rule.RuleID), zap.String("rule_name", rule.Name), zap.Error(err))
		result.Status = rulev1.EvaluationStatus_ERROR
		result.ErrorMessage = "failed to evaluate rule"
		return result
	}

	result.Matched = matched
	if matched {
		result.ActionValue = actionValue
	}

	return result
}

func (s *Service) executeRule(rule *Rule, attrs map[string]any) (bool, *structpb.Struct, error) {

	env, err := celengine.GetOrBuildEnv(attrs)
	if err != nil {
		return false, nil, err
	}

	matched, err := celengine.Evaluate(env, rule.DSLExpression, attrs)
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

	summary, err := s.buildActionSummary(actions)
	if err != nil {
		return false, nil, err
	}

	return true, summary, nil
}

func (s *Service) buildActionSummary(actions []RuleAction) (*structpb.Struct, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	var (
		totalPoints          int64
		totalCashback        float64
		totalVoucherDiscount float64

		vouchers      []any
		notifications []any
		tags          = make(map[string]string)
		metadata      = make(map[string]string)
		actionEntries = make([]any, 0, len(actions))
	)

	for _, action := range actions {
		entry := map[string]any{"type": action.Type.String()}

		switch action.Type {
		case rulev1.RuleActionType_REWARD_POINT:
			if p := action.PointAction; p != nil {
				entry["points"] = p.Points
				entry["reference"] = p.Reference
				entry["metadata"] = stringMapToAny(p.Metadata)
				metadata = mergeStringMaps(metadata, p.Metadata)
				totalPoints += p.Points
			}

		case rulev1.RuleActionType_VOUCHER:
			if v := action.VoucherAction; v != nil {
				detail := map[string]any{
					"voucher_code":   v.VoucherCode,
					"discount_value": v.DiscountValue,
					"discount_type":  v.DiscountType,
				}
				if v.ExpiryDate != nil {
					detail["expiry_date"] = v.ExpiryDate.UTC().Format(time.RFC3339)
				}
				detail["metadata"] = stringMapToAny(v.Metadata)
				metadata = mergeStringMaps(metadata, v.Metadata)
				vouchers = append(vouchers, detail)
				entry["voucher"] = detail
				totalVoucherDiscount += v.DiscountValue
			}

		case rulev1.RuleActionType_CASHBACK:
			if c := action.CashbackAction; c != nil {
				detail := map[string]any{
					"amount":   c.Amount,
					"currency": c.Currency,
				}
				if c.TargetWalletID != "" {
					detail["target_wallet_id"] = c.TargetWalletID
				}
				detail["metadata"] = stringMapToAny(c.Metadata)
				metadata = mergeStringMaps(metadata, c.Metadata)
				totalCashback += c.Amount
				entry["cashback"] = detail
			}

		case rulev1.RuleActionType_NOTIFY:
			if n := action.NotifyAction; n != nil {
				detail := map[string]any{
					"channel":     n.Channel,
					"template_id": n.TemplateID,
					"metadata":    stringMapToAny(n.Metadata),
				}
				metadata = mergeStringMaps(metadata, n.Metadata)
				notifications = append(notifications, detail)
				entry["notification"] = detail
			}

		case rulev1.RuleActionType_TAG:
			if t := action.TagAction; t != nil {
				tag := map[string]any{"tag_key": t.TagKey, "tag_value": t.TagValue}
				entry["tag"] = tag
				if t.TagKey != "" {
					tags[t.TagKey] = t.TagValue
				}
			}
		}

		actionEntries = append(actionEntries, entry)
	}

	summary := map[string]any{
		"actions":       actionEntries,
		"actions_count": len(actionEntries),
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"execution_id":  s.newExecutionID(),
	}

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

	pbSummary, err := structpb.NewStruct(summary)
	if err != nil {
		return nil, err
	}

	return pbSummary, nil
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

	values := md.Get("X-Tenant-ID")
	if len(values) == 0 {
		return "", status.Error(codes.InvalidArgument, "tenant_id is required")
	}

	if candidate := strings.TrimSpace(values[0]); candidate != "" {
		return candidate, nil
	}

	return "", status.Error(codes.InvalidArgument, "tenant_id is required")
}

func CalcRuleSetHash(rules []Rule) string {
	h := sha256.New()
	sort.Slice(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	for _, r := range rules {
		io.WriteString(h, r.RuleID)
		io.WriteString(h, "|")
		io.WriteString(h, strconv.Itoa(int(r.Priority)))
		io.WriteString(h, "|")
		io.WriteString(h, r.DSLExpression)
		io.WriteString(h, "|")
		io.WriteString(h, strings.TrimSpace(string(r.Actions))) // JSON canonicalized kalau bisa
		io.WriteString(h, ";")
	}
	return hex.EncodeToString(h.Sum(nil))
}
