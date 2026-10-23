package resilientcache

import (
	"context"
	"math"
	"sync"
	"time"

	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
)

// sweepInterval bounds how often expired entries are purged during writes.
const sweepInterval = time.Minute

type memoryEntry[Entity any] struct {
	value     Entity
	expiresAt time.Time
}

// MemoryCache is an in-memory cache.Cache that, unlike the go-autumn-synchronisation
// memory cache, honors the retention passed to Set.
type MemoryCache[Entity any] struct {
	mu        sync.RWMutex
	entries   map[string]memoryEntry[Entity]
	now       func() time.Time
	lastSweep time.Time
}

var _ aucache.Cache[[]byte] = (*MemoryCache[[]byte])(nil)

func NewMemoryCache[Entity any]() *MemoryCache[Entity] {
	return newMemoryCache[Entity](time.Now)
}

func newMemoryCache[Entity any](now func() time.Time) *MemoryCache[Entity] {
	return &MemoryCache[Entity]{
		entries:   make(map[string]memoryEntry[Entity]),
		now:       now,
		lastSweep: now(),
	}
}

func (c *MemoryCache[Entity]) Entries(_ context.Context) (map[string]Entity, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.now()
	out := make(map[string]Entity, len(c.entries))
	for k, e := range c.entries {
		if e.alive(now) {
			out[k] = e.value
		}
	}
	return out, nil
}

func (c *MemoryCache[Entity]) Keys(ctx context.Context) ([]string, error) {
	entries, _ := c.Entries(ctx)
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	return keys, nil
}

func (c *MemoryCache[Entity]) Values(ctx context.Context) ([]Entity, error) {
	entries, _ := c.Entries(ctx)
	values := make([]Entity, 0, len(entries))
	for _, v := range entries {
		values = append(values, v)
	}
	return values, nil
}

func (c *MemoryCache[Entity]) Set(_ context.Context, key string, value Entity, retention time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	entry := memoryEntry[Entity]{value: value}
	if retention > 0 {
		entry.expiresAt = now.Add(retention)
	}
	c.entries[key] = entry
	if now.Sub(c.lastSweep) >= sweepInterval {
		for k, e := range c.entries {
			if !e.alive(now) {
				delete(c.entries, k)
			}
		}
		c.lastSweep = now
	}
	return nil
}

func (c *MemoryCache[Entity]) Get(_ context.Context, key string) (*Entity, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || !e.alive(c.now()) {
		return nil, nil
	}
	value := e.value
	return &value, nil
}

func (c *MemoryCache[Entity]) Remove(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

func (c *MemoryCache[Entity]) RemainingRetention(_ context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	now := c.now()
	if !ok || !e.alive(now) {
		return 0, nil
	}
	if e.expiresAt.IsZero() {
		return math.MaxInt64, nil
	}
	return e.expiresAt.Sub(now), nil
}

func (e memoryEntry[Entity]) alive(now time.Time) bool {
	return e.expiresAt.IsZero() || now.Before(e.expiresAt)
}
