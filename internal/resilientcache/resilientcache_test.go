package resilientcache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/test/mock/cachemock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flakyCache wraps a cache and can be switched into failing or hanging mode.
type flakyCache struct {
	*cachemock.Cache[[]byte]
	fail  atomic.Bool
	hang  atomic.Bool
	calls atomic.Int32
}

func (f *flakyCache) check(ctx context.Context) error {
	f.calls.Add(1)
	if f.hang.Load() {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.fail.Load() {
		return errors.New("NOREPLICAS Not enough good replicas to write.")
	}
	return nil
}

func (f *flakyCache) Get(ctx context.Context, key string) (*[]byte, error) {
	if err := f.check(ctx); err != nil {
		return nil, err
	}
	return f.Cache.Get(ctx, key)
}

func (f *flakyCache) Set(ctx context.Context, key string, value []byte, retention time.Duration) error {
	if err := f.check(ctx); err != nil {
		return err
	}
	return f.Cache.Set(ctx, key, value, retention)
}

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestFallback(t *testing.T, newPrimary func() (aucache.Cache[[]byte], error)) (*FallbackCache[[]byte], *MemoryCache[[]byte], *testClock) {
	t.Helper()
	clock := &testClock{t: time.Unix(1_700_000_000, 0)}
	fallback := newMemoryCache[[]byte](clock.now)
	c := NewFallbackCache("test", newPrimary, fallback, Options{
		OperationTimeout: 50 * time.Millisecond,
		Cooldown:         30 * time.Second,
	})
	c.now = clock.now
	return c, fallback, clock
}

func TestFallbackCache_UsesPrimaryWhenHealthy(t *testing.T) {
	ctx := context.Background()
	primary := &flakyCache{Cache: cachemock.New[[]byte]()}
	c, fallback, _ := newTestFallback(t, func() (aucache.Cache[[]byte], error) { return primary, nil })

	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Minute))

	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, []byte("v"), *got)

	inFallback, _ := fallback.Get(ctx, "k")
	assert.Nil(t, inFallback)
}

func TestFallbackCache_StartsWhenPrimaryCannotConnect(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32
	primary := &flakyCache{Cache: cachemock.New[[]byte]()}
	c, _, clock := newTestFallback(t, func() (aucache.Cache[[]byte], error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("EOF")
		}
		return primary, nil
	})

	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Minute))
	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(1), attempts.Load(), "must not reconnect during cooldown")

	clock.advance(31 * time.Second)
	require.NoError(t, c.Set(ctx, "k2", []byte("v2"), time.Minute))
	assert.Equal(t, int32(2), attempts.Load())
	inPrimary, _ := primary.Cache.Get(ctx, "k2")
	require.NotNil(t, inPrimary)
}

func TestFallbackCache_FallsBackOnPrimaryErrors(t *testing.T) {
	ctx := context.Background()
	primary := &flakyCache{Cache: cachemock.New[[]byte]()}
	primary.fail.Store(true)
	c, fallback, clock := newTestFallback(t, func() (aucache.Cache[[]byte], error) { return primary, nil })

	require.NoError(t, c.Set(ctx, "k", []byte("v"), time.Minute))
	inFallback, _ := fallback.Get(ctx, "k")
	require.NotNil(t, inFallback)

	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int32(1), primary.calls.Load(), "primary must be bypassed during cooldown")

	primary.fail.Store(false)
	clock.advance(31 * time.Second)
	require.NoError(t, c.Set(ctx, "k3", []byte("v3"), time.Minute))
	inPrimary, _ := primary.Cache.Get(ctx, "k3")
	require.NotNil(t, inPrimary, "primary must be used again after cooldown")
}

func TestFallbackCache_TimesOutHangingPrimary(t *testing.T) {
	ctx := context.Background()
	primary := &flakyCache{Cache: cachemock.New[[]byte]()}
	primary.hang.Store(true)
	c, _, _ := newTestFallback(t, func() (aucache.Cache[[]byte], error) { return primary, nil })

	start := time.Now()
	got, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Nil(t, got)
	assert.Less(t, time.Since(start), time.Second)
}

func TestFallbackCache_PropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	primary := &flakyCache{Cache: cachemock.New[[]byte]()}
	primary.hang.Store(true)
	c, _, _ := newTestFallback(t, func() (aucache.Cache[[]byte], error) { return primary, nil })
	c.opts.OperationTimeout = time.Minute

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	_, err := c.Get(ctx, "k")
	require.ErrorIs(t, err, context.Canceled)

	c.mu.Lock()
	defer c.mu.Unlock()
	assert.False(t, c.degraded, "caller cancellation must not mark the primary unavailable")
}

func TestMemoryCache_HonorsRetention(t *testing.T) {
	ctx := context.Background()
	clock := &testClock{t: time.Unix(1_700_000_000, 0)}
	c := newMemoryCache[[]byte](clock.now)

	require.NoError(t, c.Set(ctx, "short", []byte("a"), time.Minute))
	require.NoError(t, c.Set(ctx, "forever", []byte("b"), 0))

	remaining, err := c.RemainingRetention(ctx, "short")
	require.NoError(t, err)
	assert.Equal(t, time.Minute, remaining)

	clock.advance(2 * time.Minute)
	got, err := c.Get(ctx, "short")
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = c.Get(ctx, "forever")
	require.NoError(t, err)
	require.NotNil(t, got)

	keys, _ := c.Keys(ctx)
	assert.Equal(t, []string{"forever"}, keys)

	// a write after the sweep interval purges expired entries
	require.NoError(t, c.Set(ctx, "other", []byte("c"), time.Minute))
	c.mu.RLock()
	_, stillStored := c.entries["short"]
	c.mu.RUnlock()
	assert.False(t, stillStored)
}
