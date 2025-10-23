package rule

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"
)

var (
	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{Name: "rule_cache_hits_total"})
	cacheMiss = prometheus.NewCounter(prometheus.CounterOpts{Name: "rule_cache_miss_total"})
)

type RuleSetKey struct {
	TenantID string
	Trigger  string
}

type CompiledRuleSet struct {
	Version   int64
	Hash      string
	Rules     []*CompiledRule
	UpdatedAt time.Time
}

// thread-safe + singleflight + optional LRU
type RuleCache struct {
	mu    sync.RWMutex
	items map[RuleSetKey]*CompiledRuleSet
	ttl   time.Duration
	group singleflight.Group
	// optional: lru *lru.Cache[RuleSetKey, *CompiledRuleSet]
}

func NewRuleCache(ttl time.Duration) *RuleCache {
	return &RuleCache{
		items: make(map[RuleSetKey]*CompiledRuleSet),
		ttl:   ttl,
	}
}

func (c *RuleCache) Get(key RuleSetKey) (*CompiledRuleSet, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	if !ok || (c.ttl > 0 && time.Since(v.UpdatedAt) > c.ttl) {
		return nil, false
	}
	return v, true
}

func (c *RuleCache) Set(key RuleSetKey, v *CompiledRuleSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = v
}

func (c *RuleCache) Invalidate(key RuleSetKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
