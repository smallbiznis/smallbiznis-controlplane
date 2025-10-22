package sequence

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

var Module = fx.Module("sequence",
	fx.Provide(NewRedisGenerator),
)

type Generator interface {
	NextTenantCode(ctx context.Context) (string, error)
	NextRuleCode(ctx context.Context, tenantID string) (string, error)
	NextTransactionCode(ctx context.Context, tenantID string) (string, error)
	NextCampaignCode(ctx context.Context, tenantID string) (string, error)
	NextVoucherCode(ctx context.Context, tenantID, campaignCode string) (string, error)
}

type RedisGenerator struct {
	rdb *redis.Client
}

type Params struct {
	fx.In

	Redis *redis.Client
}

func NewRedisGenerator(p Params) Generator {
	return &RedisGenerator{
		rdb: p.Redis,
	}
}

func (g *RedisGenerator) NextTenantCode(ctx context.Context) (string, error) {
	key := "seq:tenant"
	seq, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("T%03d", seq), nil
}

func (g *RedisGenerator) NextRuleCode(ctx context.Context, tenantID string) (string, error) {
	return g.nextDailyCode(ctx, "RL", tenantID, false)
}

func (g *RedisGenerator) NextTransactionCode(ctx context.Context, tenantID string) (string, error) {
	return g.nextDailyCode(ctx, "TXN", tenantID, false)
}

func (g *RedisGenerator) NextCampaignCode(ctx context.Context, tenantID string) (string, error) {
	return g.nextDailyCode(ctx, "CMP", tenantID, true)
}

func (g *RedisGenerator) NextVoucherCode(ctx context.Context, tenantID, campaignCode string) (string, error) {
	today := time.Now().UTC().Format("20060102")
	key := fmt.Sprintf("seq:VCHR:%s:%s:%s", tenantID, campaignCode, today)

	seq, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return "", err
	}

	if seq == 1 {
		expire := time.Until(time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Second))
		_ = g.rdb.Expire(ctx, key, expire).Err()
	}

	return fmt.Sprintf("%s%s%05d", campaignCode, today, seq), nil
}

func (g *RedisGenerator) nextDailyCode(ctx context.Context, prefix, tenantCode string, includeTenantInCode bool) (string, error) {
	today := time.Now().UTC().Format("20060102")
	key := fmt.Sprintf("seq:%s:%s:%s", prefix, tenantCode, today)

	seq, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return "", err
	}

	if seq == 1 {
		expire := time.Until(time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Second))
		_ = g.rdb.Expire(ctx, key, expire).Err()
	}

	if includeTenantInCode {
		return fmt.Sprintf("%s-%s%05d", prefix, today, seq), nil
	}
	return fmt.Sprintf("%s-%s-%05d", prefix, today, seq), nil
}
