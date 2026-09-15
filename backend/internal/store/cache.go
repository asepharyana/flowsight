package store

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache wraps Redis with an in-memory TTL fallback so the demo runs without
// REDIS_URL. Reference data (registry/taxonomy/tags) uses 24h TTL.
type Cache struct {
	rdb *redis.Client
	mu  sync.Mutex
	mem map[string]memEntry
}

type memEntry struct {
	val string
	exp time.Time
}

// NewCache builds a cache; empty redisURL selects the in-memory fallback.
func NewCache(redisURL string) *Cache {
	c := &Cache{mem: map[string]memEntry{}}
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err == nil {
			c.rdb = redis.NewClient(opt)
		}
	}
	return c
}

// HasRedis reports whether a live Redis backs this cache.
func (c *Cache) HasRedis() bool { return c.rdb != nil }

// Get returns the cached value or "" on miss/expiry.
func (c *Cache) Get(ctx context.Context, key string) string {
	if c.rdb != nil {
		if v, err := c.rdb.Get(ctx, key).Result(); err == nil {
			return v
		}
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.mem[key]
	if !ok || time.Now().After(e.exp) {
		return ""
	}
	return e.val
}

// Set stores a value with TTL.
func (c *Cache) Set(ctx context.Context, key, val string, ttl time.Duration) {
	if c.rdb != nil {
		_ = c.rdb.Set(ctx, key, val, ttl).Err()
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem[key] = memEntry{val: val, exp: time.Now().Add(ttl)}
}

// GetJSON unmarshals a cached JSON value; ok=false on miss.
func (c *Cache) GetJSON(ctx context.Context, key string, v any) bool {
	raw := c.Get(ctx, key)
	if raw == "" {
		return false
	}
	return json.Unmarshal([]byte(raw), v) == nil
}

// SetJSON marshals and stores a value with TTL.
func (c *Cache) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.Set(ctx, key, string(b), ttl)
}
