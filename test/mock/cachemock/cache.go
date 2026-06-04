package cachemock

import (
	"context"
	"sync"
	"time"
)

// Cache is a simple in-memory mock implementation of cache.Cache[T].
type Cache[T any] struct {
	mu      sync.RWMutex
	entries map[string]T
}

func New[T any]() *Cache[T] {
	return &Cache[T]{
		entries: make(map[string]T),
	}
}

func (c *Cache[T]) Entries(_ context.Context) (map[string]T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]T, len(c.entries))
	for k, v := range c.entries {
		out[k] = v
	}
	return out, nil
}

func (c *Cache[T]) Keys(_ context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		keys = append(keys, k)
	}
	return keys, nil
}

func (c *Cache[T]) Values(_ context.Context) ([]T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	values := make([]T, 0, len(c.entries))
	for _, v := range c.entries {
		values = append(values, v)
	}
	return values, nil
}

func (c *Cache[T]) Set(_ context.Context, key string, value T, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
	return nil
}

func (c *Cache[T]) Get(_ context.Context, key string) (*T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[key]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

func (c *Cache[T]) Remove(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

func (c *Cache[T]) RemainingRetention(_ context.Context, _ string) (time.Duration, error) {
	return 0, nil
}

