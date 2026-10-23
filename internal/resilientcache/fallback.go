package resilientcache

import (
	"context"
	"sync"
	"time"

	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	aulogging "github.com/StephanHCB/go-autumn-logging"
)

type Options struct {
	// OperationTimeout bounds every single call to the primary cache.
	OperationTimeout time.Duration
	// Cooldown is how long the primary cache is skipped after a failure.
	Cooldown time.Duration
}

// FallbackCache wraps a primary (remote) cache and transparently falls back to a
// secondary (local) cache whenever the primary is unreachable, slow, or failing.
// The primary is created lazily and recreated on demand, so an unavailable primary
// at startup does not prevent the application from starting.
type FallbackCache[Entity any] struct {
	name       string
	newPrimary func() (aucache.Cache[Entity], error)
	fallback   aucache.Cache[Entity]
	opts       Options
	now        func() time.Time

	mu               sync.Mutex
	primary          aucache.Cache[Entity]
	connecting       bool
	degraded         bool
	unavailableUntil time.Time
}

var _ aucache.Cache[[]byte] = (*FallbackCache[[]byte])(nil)

func NewFallbackCache[Entity any](
	name string,
	newPrimary func() (aucache.Cache[Entity], error),
	fallback aucache.Cache[Entity],
	opts Options,
) *FallbackCache[Entity] {
	return &FallbackCache[Entity]{
		name:       name,
		newPrimary: newPrimary,
		fallback:   fallback,
		opts:       opts,
		now:        time.Now,
	}
}

func (c *FallbackCache[Entity]) Entries(ctx context.Context) (map[string]Entity, error) {
	return run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) (map[string]Entity, error) { return p.Entries(ctx) },
		func() (map[string]Entity, error) { return c.fallback.Entries(ctx) },
	)
}

func (c *FallbackCache[Entity]) Keys(ctx context.Context) ([]string, error) {
	return run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) ([]string, error) { return p.Keys(ctx) },
		func() ([]string, error) { return c.fallback.Keys(ctx) },
	)
}

func (c *FallbackCache[Entity]) Values(ctx context.Context) ([]Entity, error) {
	return run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) ([]Entity, error) { return p.Values(ctx) },
		func() ([]Entity, error) { return c.fallback.Values(ctx) },
	)
}

func (c *FallbackCache[Entity]) Set(ctx context.Context, key string, value Entity, retention time.Duration) error {
	_, err := run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) (struct{}, error) {
			return struct{}{}, p.Set(ctx, key, value, retention)
		},
		func() (struct{}, error) { return struct{}{}, c.fallback.Set(ctx, key, value, retention) },
	)
	return err
}

func (c *FallbackCache[Entity]) Get(ctx context.Context, key string) (*Entity, error) {
	return run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) (*Entity, error) { return p.Get(ctx, key) },
		func() (*Entity, error) { return c.fallback.Get(ctx, key) },
	)
}

func (c *FallbackCache[Entity]) Remove(ctx context.Context, key string) error {
	// entries may live in either cache, so always clear the fallback as well
	if err := c.fallback.Remove(ctx, key); err != nil {
		return err
	}
	_, err := run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) (struct{}, error) { return struct{}{}, p.Remove(ctx, key) },
		func() (struct{}, error) { return struct{}{}, nil },
	)
	return err
}

func (c *FallbackCache[Entity]) RemainingRetention(ctx context.Context, key string) (time.Duration, error) {
	return run(c, ctx,
		func(ctx context.Context, p aucache.Cache[Entity]) (time.Duration, error) {
			return p.RemainingRetention(ctx, key)
		},
		func() (time.Duration, error) { return c.fallback.RemainingRetention(ctx, key) },
	)
}

func run[Entity any, Result any](
	c *FallbackCache[Entity],
	ctx context.Context,
	primaryFn func(context.Context, aucache.Cache[Entity]) (Result, error),
	fallbackFn func() (Result, error),
) (Result, error) {
	primary := c.acquirePrimary(ctx)
	if primary == nil {
		return fallbackFn()
	}

	opCtx, cancel := context.WithTimeout(ctx, c.opts.OperationTimeout)
	result, err := primaryFn(opCtx, primary)
	cancel()
	if err == nil {
		c.markAvailable(ctx)
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		// the caller gave up, which says nothing about the health of the primary
		var zero Result
		return zero, ctxErr
	}
	c.markUnavailable(ctx, err)
	return fallbackFn()
}

// acquirePrimary returns the primary cache, or nil if it should currently be bypassed.
func (c *FallbackCache[Entity]) acquirePrimary(ctx context.Context) aucache.Cache[Entity] {
	c.mu.Lock()
	if c.now().Before(c.unavailableUntil) || c.connecting {
		c.mu.Unlock()
		return nil
	}
	if c.primary != nil {
		primary := c.primary
		c.mu.Unlock()
		return primary
	}
	c.connecting = true
	c.mu.Unlock()

	// connect outside the lock so concurrent requests use the fallback instead of waiting
	primary, err := c.newPrimary()

	c.mu.Lock()
	c.connecting = false
	if err == nil {
		c.primary = primary
	}
	c.mu.Unlock()

	if err != nil {
		c.markUnavailable(ctx, err)
		return nil
	}
	return primary
}

func (c *FallbackCache[Entity]) markUnavailable(ctx context.Context, err error) {
	c.mu.Lock()
	c.unavailableUntil = c.now().Add(c.opts.Cooldown)
	wasDegraded := c.degraded
	c.degraded = true
	c.mu.Unlock()

	if !wasDegraded {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf(
			"primary cache '%s' is unavailable, falling back to in-memory cache (retrying every %s)",
			c.name, c.opts.Cooldown)
	} else {
		aulogging.Logger.Ctx(ctx).Debug().WithErr(err).Printf("primary cache '%s' is still unavailable", c.name)
	}
}

func (c *FallbackCache[Entity]) markAvailable(ctx context.Context) {
	c.mu.Lock()
	wasDegraded := c.degraded
	c.degraded = false
	c.mu.Unlock()

	if wasDegraded {
		aulogging.Logger.Ctx(ctx).Info().Printf("primary cache '%s' is available again", c.name)
	}
}
