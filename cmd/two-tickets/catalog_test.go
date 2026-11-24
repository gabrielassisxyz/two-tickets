package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

const pruneTestMaxBytes int64 = 4096

// fakePruneCache records how the loop used it. With blockPrune set, a prune
// waits until the loop is stopped, so the test can stop it mid-prune.
type fakePruneCache struct {
	blockPrune bool
	started    chan struct{}

	mu                sync.Mutex
	prunes            int
	maxBytes          int64
	inFlight          bool
	closed            bool
	closedDuringPrune bool
	prunedAfterClose  bool
}

func newFakePruneCache(blockPrune bool) *fakePruneCache {
	return &fakePruneCache{blockPrune: blockPrune, started: make(chan struct{}, 1)}
}

func (f *fakePruneCache) PruneAndMaybeRotate(ctx context.Context, _ int64, maxBytes int64) (bool, error) {
	f.mu.Lock()
	f.prunes++
	f.maxBytes = maxBytes
	f.inFlight = true
	f.prunedAfterClose = f.prunedAfterClose || f.closed
	f.mu.Unlock()
	select {
	case f.started <- struct{}{}:
	default:
	}
	var err error
	if f.blockPrune {
		<-ctx.Done()
		err = ctx.Err()
	}
	f.mu.Lock()
	f.inFlight = false
	f.mu.Unlock()
	return false, err
}

func (f *fakePruneCache) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.closedDuringPrune = f.closedDuringPrune || f.inFlight
	return nil
}

func (f *fakePruneCache) pruneCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prunes
}

func waitForPrunes(t *testing.T, cache *fakePruneCache, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for cache.pruneCount() < want {
		if time.Now().After(deadline) {
			t.Fatalf("the loop pruned %d times, want at least %d", cache.pruneCount(), want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCachePruningRunsOnceAtStart(t *testing.T) {
	cache := newFakePruneCache(false)
	stop := startCachePruning(context.Background(), cache, time.Hour, pruneTestMaxBytes, slog.New(slog.DiscardHandler))
	waitForPrunes(t, cache, 1)
	time.Sleep(50 * time.Millisecond)
	stop()
	if got := cache.pruneCount(); got != 1 {
		t.Errorf("pruned %d times inside an hour-long interval, want once at start", got)
	}
	if cache.maxBytes != pruneTestMaxBytes {
		t.Errorf("pruned with a cap of %d, want %d", cache.maxBytes, pruneTestMaxBytes)
	}
}

func TestCachePruningRunsOnItsIntervalAndStopsOnCancellation(t *testing.T) {
	cache := newFakePruneCache(false)
	ctx, cancel := context.WithCancel(context.Background())
	stop := startCachePruning(ctx, cache, 5*time.Millisecond, pruneTestMaxBytes, slog.New(slog.DiscardHandler))
	waitForPrunes(t, cache, 3)
	cancel()
	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the loop did not stop once its context was cancelled")
	}
	after := cache.pruneCount()
	time.Sleep(50 * time.Millisecond)
	if got := cache.pruneCount(); got != after {
		t.Errorf("pruned %d more times after stopping", got-after)
	}
	if !cache.closed || cache.prunedAfterClose {
		t.Errorf("closed = %v, pruned after close = %v; want closed, never pruned after", cache.closed, cache.prunedAfterClose)
	}
}

// Stopping while a prune is in flight must wait for it before closing:
// serve's only shutdown of cache.db is this stop function.
func TestCachePruningStopsBeforeTheCacheCloses(t *testing.T) {
	var logs bytes.Buffer
	cache := newFakePruneCache(true)
	stop := startCachePruning(context.Background(), cache, time.Hour, pruneTestMaxBytes, slog.New(slog.NewTextHandler(&logs, nil)))
	<-cache.started
	stop()
	if !cache.closed {
		t.Fatal("stopping the loop did not close the cache")
	}
	if cache.closedDuringPrune {
		t.Fatal("the cache was closed while a prune was still running")
	}
	if logs.Len() != 0 {
		t.Errorf("a prune ended by the stop was logged as a failure:\n%s", logs.String())
	}
}

func TestCacheReadinessFailsOnceTheCacheIsClosed(t *testing.T) {
	cache, err := store.OpenCacheStore(context.Background(), filepath.Join(t.TempDir(), store.CacheFileName))
	if err != nil {
		t.Fatal(err)
	}
	check := cacheReadiness{cache: cache}
	if err := check.Ping(context.Background()); err != nil {
		t.Fatalf("ping an open cache: %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	if err := check.Ping(context.Background()); err == nil {
		t.Fatal("ping a closed cache answered nil")
	}
}

// The e2e stack points the api at a TMDB substitute through TMDB_BASE_URL;
// the one client the process builds must call it, not TMDB.
func TestTMDBClientCallsTheConfiguredBaseURL(t *testing.T) {
	var gotPath string
	substitute := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"genres":[]}`))
	}))
	t.Cleanup(substitute.Close)
	cache, err := store.OpenCacheStore(context.Background(), filepath.Join(t.TempDir(), store.CacheFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	cfg := config.Config{TMDBAPIKey: config.NewSecret("sentinelsentinelsentinel"), TMDBBaseURL: substitute.URL + "/3"}
	client := newTMDBClient(cfg, cache, slog.New(slog.DiscardHandler))
	if _, err := client.GetMovieGenres(context.Background()); err != nil {
		t.Fatalf("GetMovieGenres through the substitute: %v", err)
	}
	if gotPath != "/3/genre/movie/list" {
		t.Fatalf("the substitute saw %q, want /3/genre/movie/list", gotPath)
	}
}
