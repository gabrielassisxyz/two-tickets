package tmdb

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
)

func TestRetryAfterOneSecondIsHonoured(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if upstream.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Central do Brasil"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	cache := newCacheStoreForTest(t, ctx)
	client, _, sleeper := newTestClient(t, server.URL, testClientArgs{cache: cache})

	var movie MovieDetails
	if err := client.Get(ctx, "/movie/550", nil, &movie); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := upstream.Load(); got != 2 {
		t.Fatalf("upstream requests = %d, want 2 (one 429, one success)", got)
	}
	recorded := sleeper.recorded()
	if len(recorded) != 1 {
		t.Fatalf("waits = %d, want 1", len(recorded))
	}
	if recorded[0] < time.Second {
		t.Fatalf("wait = %v, want at least 1s from Retry-After", recorded[0])
	}
}

func TestFiveHundredThreeTimesReturnsExactlyThreeAttempts(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	client, _, _ := newTestClient(t, server.URL, testClientArgs{budget: 30 * time.Second})

	start := time.Now()
	var movie MovieDetails
	err := client.Get(ctx, "/movie/550", nil, &movie)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Get succeeded against three 500s")
	}
	if got := upstream.Load(); got != 3 {
		t.Fatalf("upstream attempts = %d, want exactly 3", got)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("elapsed = %v, want within the 30s budget", elapsed)
	}
	// The catalog maps TMDB failures by status, so exhaustion must keep it.
	var tmdbErr *TMDBError
	if !errors.As(err, &tmdbErr) || tmdbErr.Status != http.StatusInternalServerError {
		t.Fatalf("error = %v, want a *TMDBError carrying status 500", err)
	}
}

func TestRateLimitedOnEveryAttemptKeepsTheStatus(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, _, _ := newTestClient(t, server.URL, testClientArgs{budget: 30 * time.Second})
	var movie MovieDetails
	err := client.Get(context.Background(), "/movie/550", nil, &movie)
	if got := upstream.Load(); got != 3 {
		t.Fatalf("upstream attempts = %d, want exactly 3", got)
	}
	var tmdbErr *TMDBError
	if !errors.As(err, &tmdbErr) || tmdbErr.Status != http.StatusTooManyRequests {
		t.Fatalf("error = %v, want a *TMDBError carrying status 429", err)
	}
}

func TestNeverAnsweringServerIsAbandonedPerAttempt(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()

	ctx := context.Background()
	// Reduced limits: 200 ms per attempt instead of the production 20 s, so
	// the test proves abandonment without waiting a minute.
	client, _, _ := newTestClient(t, server.URL, testClientArgs{attemptTO: 200 * time.Millisecond, maxTries: 1})

	start := time.Now()
	var movie MovieDetails
	err := client.Get(ctx, "/movie/550", nil, &movie)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Get succeeded against a server that never answers")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("elapsed = %v, want abandonment near the 200ms attempt limit", elapsed)
	}
	if strings.Contains(err.Error(), dummyAPIKey) {
		t.Fatalf("error leaks the key: %q", err)
	}
}

func TestCallerCancellationStopsRetriesImmediately(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel shortly after the request starts: the client must stop instead
	// of running the retry budget out.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	client, _, _ := newTestClient(t, server.URL, testClientArgs{attemptTO: 5 * time.Second})

	var movie MovieDetails
	err := client.Get(ctx, "/movie/550", nil, &movie)
	if err == nil {
		t.Fatal("Get succeeded after cancellation")
	}
	if got := upstream.Load(); got != 1 {
		t.Fatalf("upstream attempts = %d, want 1: cancellation must stop retries", got)
	}
}

// The caller returning is not enough: with nobody left waiting, the shared
// fetch itself must stop, and the upstream request with it.
func TestLastCallerLeavingCancelsTheUpstreamRequest(t *testing.T) {
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client, _, _ := newTestClient(t, server.URL, testClientArgs{attemptTO: 5 * time.Second})
	done := make(chan error, 1)
	go func() {
		var movie MovieDetails
		done <- client.Get(ctx, "/movie/550", nil, &movie)
	}()
	waitUntilShared(t, client, 1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Get after cancellation = %v, want context.Canceled", err)
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("the upstream request kept running after its only caller left")
	}
}

func TestCached200ServedWithoutUpstreamUntilTTLPasses(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Central do Brasil"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	cache := newCacheStoreForTest(t, ctx)
	client, clock, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})

	var first MovieDetails
	if err := client.Get(ctx, "/movie/550", nil, &first); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	var second MovieDetails
	if err := client.Get(ctx, "/movie/550", nil, &second); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got := upstream.Load(); got != 1 {
		t.Fatalf("upstream = %d after a cached hit, want 1", got)
	}
	// Details live 7 days: advance past the TTL and the next call must fetch.
	clock.Advance(8 * 24 * time.Hour)
	var third MovieDetails
	if err := client.Get(ctx, "/movie/550", nil, &third); err != nil {
		t.Fatalf("third Get: %v", err)
	}
	if got := upstream.Load(); got != 2 {
		t.Fatalf("upstream = %d after TTL expiry, want 2", got)
	}
}

// Only the keyword 404 is an intentional negative. Any other 404 is a missing
// record: cached as null it would decode into an empty title, with no error,
// for as long as the row lived.
func TestNotFoundOutsideKeywordsIsAnErrorAndLeavesNoRow(t *testing.T) {
	ctx := context.Background()
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	cache := newCacheStoreForTest(t, ctx)
	client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})

	var movie MovieDetails
	err := client.Get(ctx, "/movie/999999", nil, &movie)
	var tmdbErr *TMDBError
	if !errors.As(err, &tmdbErr) || tmdbErr.Status != http.StatusNotFound {
		t.Fatalf("error = %v, want a *TMDBError carrying status 404", err)
	}
	key := BuildCacheKey(CacheFormatVersion, "GET", "/movie/999999", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "include_adult": "false"})
	if _, ok, _ := cache.Get(ctx, key); ok {
		t.Fatal("a 404 outside the keyword family left a cache row")
	}
	_ = client.Get(ctx, "/movie/999999", nil, &movie)
	if got := upstream.Load(); got != 2 {
		t.Fatalf("upstream requests = %d, want 2: a missing record is asked again, never served from cache", got)
	}
}

func TestFailuresLeaveNoCacheRowAndKeyword404IsNegative(t *testing.T) {
	ctx := context.Background()

	t.Run("429 leaves no row", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()
		cache := newCacheStoreForTest(t, ctx)
		client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache, maxTries: 1})
		var out MovieDetails
		_ = client.Get(ctx, "/movie/1", nil, &out)
		key := BuildCacheKey(CacheFormatVersion, "GET", "/movie/1", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "include_adult": "false"})
		if _, ok, _ := cache.Get(ctx, key); ok {
			t.Fatal("a 429 left a cache row")
		}
	})

	t.Run("5xx leaves no row", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		cache := newCacheStoreForTest(t, ctx)
		client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache, maxTries: 1})
		var out MovieDetails
		_ = client.Get(ctx, "/movie/2", nil, &out)
		key := BuildCacheKey(CacheFormatVersion, "GET", "/movie/2", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "include_adult": "false"})
		if _, ok, _ := cache.Get(ctx, key); ok {
			t.Fatal("a 5xx left a cache row")
		}
	})

	t.Run("timeout leaves no row", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		}))
		defer server.Close()
		cache := newCacheStoreForTest(t, ctx)
		client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache, maxTries: 1, attemptTO: 100 * time.Millisecond})
		var out MovieDetails
		_ = client.Get(ctx, "/movie/3", nil, &out)
		key := BuildCacheKey(CacheFormatVersion, "GET", "/movie/3", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "include_adult": "false"})
		if _, ok, _ := cache.Get(ctx, key); ok {
			t.Fatal("a timeout left a cache row")
		}
	})

	t.Run("cancellation leaves no row", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()
		cache := newCacheStoreForTest(t, ctx)
		client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		var out MovieDetails
		_ = client.Get(cancelled, "/movie/4", nil, &out)
		key := BuildCacheKey(CacheFormatVersion, "GET", "/movie/4", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "include_adult": "false"})
		if _, ok, _ := cache.Get(ctx, key); ok {
			t.Fatal("a cancellation left a cache row")
		}
	})

	t.Run("keyword 404 is a cached negative", func(t *testing.T) {
		var upstream atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			upstream.Add(1)
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		cache := newCacheStoreForTest(t, ctx)
		client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})
		first, err := client.GetKeywordDetails(ctx, "999999")
		if err != nil {
			t.Fatalf("first keyword Get: %v", err)
		}
		if first != nil {
			t.Fatalf("missing keyword = %+v, want nil (JSON null)", first)
		}
		second, err := client.GetKeywordDetails(ctx, "999999")
		if err != nil {
			t.Fatalf("second keyword Get: %v", err)
		}
		if second != nil {
			t.Fatalf("cached negative = %+v, want nil", second)
		}
		if got := upstream.Load(); got != 1 {
			t.Fatalf("upstream = %d, want 1: the 404 must be served from cache", got)
		}
	})
}

func TestBumpingFormatVersionInvalidatesRows(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Central do Brasil"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	cache := newCacheStoreForTest(t, ctx)
	first, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache, formatVer: 1})
	var movie MovieDetails
	if err := first.Get(ctx, "/movie/550", nil, &movie); err != nil {
		t.Fatal(err)
	}
	second, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache, formatVer: 2})
	if err := second.Get(ctx, "/movie/550", nil, &movie); err != nil {
		t.Fatal(err)
	}
	if got := upstream.Load(); got != 2 {
		t.Fatalf("upstream = %d, want 2: the v1 row must be unreachable at v2", got)
	}
}

func TestTMDBKeyNeverReachesLogs(t *testing.T) {
	// A transport failure, not an HTTP status: the server is gone, so the
	// HTTP client builds a *url.Error carrying the full URL, key included.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	baseURL := server.URL
	server.Close()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	client := NewClient(Options{
		BaseURL:        baseURL,
		APIKey:         config.NewSecret(dummyAPIKey),
		Logger:         logger,
		MaxAttempts:    1,
		RetryBudget:    time.Second,
		AttemptTimeout: time.Second,
	})
	var movie MovieDetails
	err := client.Get(context.Background(), "/movie/550", nil, &movie)
	if err != nil && strings.Contains(err.Error(), dummyAPIKey) {
		t.Fatalf("error leaks the key: %q", err)
	}
	if strings.Contains(logs.String(), dummyAPIKey) {
		t.Fatalf("log output leaks the key: %q", logs.String())
	}
}
