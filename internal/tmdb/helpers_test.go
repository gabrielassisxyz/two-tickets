package tmdb

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// Low entropy on purpose: a random-looking value trips the secret scanner.
const dummyAPIKey = "sentinelsentinelsentinel"

func testAPIKey() config.Secret { return config.NewSecret(dummyAPIKey) }

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Unix(1700000000, 0)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type recordingSleeper struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (s *recordingSleeper) Sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.delays = append(s.delays, d)
	s.mu.Unlock()
	// No real waiting: the test asserts on the requested delay, and the
	// caller's cancellation still ends the wait at once.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *recordingSleeper) recorded() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.delays...)
}

func newCacheStoreForTest(t *testing.T, ctx context.Context) *store.CacheStore {
	t.Helper()
	cache, err := store.OpenCacheStore(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("OpenCacheStore: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	return cache
}

type testClientArgs struct {
	clock     *testClock
	sleeper   *recordingSleeper
	cache     responseCache
	limiter   *rate.Limiter
	maxTries  int
	budget    time.Duration
	attemptTO time.Duration
	formatVer int
}

func newTestClient(t *testing.T, baseURL string, args testClientArgs) (*Client, *testClock, *recordingSleeper) {
	t.Helper()
	clock := args.clock
	if clock == nil {
		clock = newTestClock()
	}
	sleeper := args.sleeper
	if sleeper == nil {
		sleeper = &recordingSleeper{}
	}
	limiter := args.limiter
	if limiter == nil {
		// Generous for most tests so the limiter never shapes them; the burst
		// test passes the production limiter explicitly.
		limiter = rate.NewLimiter(rate.Every(time.Millisecond), 50)
	}
	maxTries := args.maxTries
	if maxTries <= 0 {
		maxTries = 3
	}
	budget := args.budget
	if budget <= 0 {
		budget = 30 * time.Second
	}
	attemptTO := args.attemptTO
	if attemptTO <= 0 {
		attemptTO = 5 * time.Second
	}
	formatVer := args.formatVer
	if formatVer <= 0 {
		formatVer = CacheFormatVersion
	}
	client := NewClient(Options{
		BaseURL:        baseURL,
		APIKey:         config.NewSecret(dummyAPIKey),
		Limiter:        limiter,
		Clock:          clock,
		Sleep:          sleeper.Sleep,
		Jitter:         func() float64 { return 0 },
		MaxAttempts:    maxTries,
		RetryBudget:    budget,
		AttemptTimeout: attemptTO,
		MaxConcurrency: 10,
		Cache:          args.cache,
		FormatVersion:  formatVer,
		Language:       DefaultLanguage,
		Region:         DefaultRegion,
	})
	return client, clock, sleeper
}
