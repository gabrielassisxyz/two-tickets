package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleflightCoalesces100IdenticalColdRequests(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		// Hold the handler briefly so the 100 callers overlap inside one
		// singleflight call rather than arriving after it completes.
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Central do Brasil"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	cache := newCacheStoreForTest(t, ctx)
	client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})

	const callers = 100
	var group sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			var movie MovieDetails
			errs[slot] = client.Get(ctx, "/movie/550", nil, &movie)
		}(i)
	}
	group.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	if got := upstream.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want exactly 1", got)
	}
}

// A shared fetch outlives any one of its callers: the caller that gives up
// gets its own cancellation at once, and the caller still waiting gets the
// response from the same single upstream request.
func TestOneCallerCancellingDoesNotFailTheOthers(t *testing.T) {
	var upstream atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":550,"title":"Central do Brasil"}`))
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	client, _, _ := newTestClient(t, server.URL, testClientArgs{})

	leaving, cancel := context.WithCancel(context.Background())
	left := make(chan error, 1)
	go func() {
		var movie MovieDetails
		left <- client.Get(leaving, "/movie/550", nil, &movie)
	}()
	waitUntilShared(t, client, 1)
	stayed := make(chan error, 1)
	var movie MovieDetails
	go func() { stayed <- client.Get(context.Background(), "/movie/550", nil, &movie) }()
	waitUntilShared(t, client, 2)

	cancel()
	if err := <-left; !errors.Is(err, context.Canceled) {
		t.Fatalf("the caller that cancelled got %v, want context.Canceled", err)
	}
	close(release)
	if err := <-stayed; err != nil {
		t.Fatalf("the caller that stayed got %v, want the response", err)
	}
	if movie.Title != "Central do Brasil" {
		t.Fatalf("title = %q, want the shared response", movie.Title)
	}
	if got := upstream.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want exactly 1", got)
	}
}

// waitUntilShared waits until exactly one fetch is in flight with the given
// number of callers waiting on it.
func waitUntilShared(t *testing.T, client *Client, waiters int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client.flightsMu.Lock()
		count, total := len(client.flights), 0
		for _, flight := range client.flights {
			total = flight.waiters
		}
		client.flightsMu.Unlock()
		if count == 1 && total == waiters {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("no single shared fetch with %d waiters appeared", waiters)
}

type startRecorder struct {
	mu     sync.Mutex
	starts []time.Time
	base   http.RoundTripper
}

func (r *startRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.starts = append(r.starts, time.Now())
	r.mu.Unlock()
	return r.base.RoundTrip(request)
}

func (r *startRecorder) snapshot() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.starts...)
}

func TestLimiterNeverExceeds20StartsInAnySecond(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
	}))
	defer server.Close()

	ctx := context.Background()
	recorder := &startRecorder{base: http.DefaultTransport}
	httpClient := &http.Client{Transport: recorder}
	// No Limiter option: the test measures the production default, which is
	// what a constructed limiter here would never have proved.
	client := NewClient(Options{
		BaseURL:        server.URL,
		APIKey:         testAPIKey(),
		HTTPClient:     httpClient,
		Clock:          newTestClock(),
		Sleep:          (&recordingSleeper{}).Sleep,
		Jitter:         func() float64 { return 0 },
		MaxAttempts:    1,
		RetryBudget:    30 * time.Second,
		AttemptTimeout: 5 * time.Second,
		MaxConcurrency: 10,
		FormatVersion:  CacheFormatVersion,
		Language:       DefaultLanguage,
		Region:         DefaultRegion,
	})

	// Sequential sustained load: 100 requests at 20/sec take 5 seconds, the
	// bead's 5-second burst. The limiter itself spaces grants; the transport
	// records each start just before the request leaves.
	limiter := client.limiter
	if got := limiter.Limit(); got != 20 {
		t.Fatalf("limiter rate = %v/sec, want 20/sec", got)
	}
	if got := limiter.Burst(); got != 1 {
		t.Fatalf("limiter burst = %d, want 1: a burst of 20 would let 40 starts land inside one second", got)
	}
	const total = 100
	burstStart := time.Now()
	for i := 0; i < total; i++ {
		var out DiscoverMoviesResponse
		_ = client.Get(ctx, "/discover/movie", map[string]string{"page": fmt.Sprintf("%d", i)}, &out)
	}
	elapsed := time.Since(burstStart)
	// 99 gaps of 50 ms take 4.95 s; anything well under proves the limiter
	// is not gating.
	if elapsed < 4*time.Second {
		t.Fatalf("100 gated requests took %v, want at least 4s at 20/sec", elapsed)
	}

	starts := recorder.snapshot()
	if len(starts) != total {
		t.Fatalf("upstream starts = %d, want %d", len(starts), total)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	for i, start := range starts {
		// Half-open windows [start, start+1s). A token bucket at 20/sec
		// with burst 1 holds at most 20 grants per window in theory; the
		// test allows 21 because grant spacing measured with time.Now()
		// varies by fractions of a millisecond (observed gaps 49.3-50.7 ms),
		// which can squeeze one extra start across a window edge. A burst
		// of 20 would hold 40, and no limiter would hold 100, so 21 still
		// distinguishes the settled design from its mutations.
		windowEnd := start.Add(time.Second)
		count := 0
		for j := i; j < len(starts) && starts[j].Before(windowEnd); j++ {
			count++
		}
		if count > 21 {
			t.Fatalf("sliding window at %v holds %d starts, want at most 21 (20 plus edge jitter)", start, count)
		}
	}
}

func TestDiscoverLoadUsesOneUpstreamPerDistinctKey(t *testing.T) {
	var upstream atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstream.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
	}))
	defer server.Close()

	ctx := context.Background()
	cache := newCacheStoreForTest(t, ctx)
	client, _, _ := newTestClient(t, server.URL, testClientArgs{cache: cache})

	// One browse page asks for three pages at once; each of four sliders for
	// two. Eleven distinct keys over a cold cache.
	type job struct {
		path   string
		params map[string]string
	}
	var jobs []job
	for _, page := range []string{"1", "2", "3"} {
		jobs = append(jobs, job{"/discover/movie", map[string]string{"page": page}})
	}
	for slider := 0; slider < 4; slider++ {
		for _, page := range []string{"1", "2"} {
			jobs = append(jobs, job{"/discover/tv", map[string]string{"page": page, "with_genres": fmt.Sprintf("%d", 10+slider)}})
		}
	}
	var group sync.WaitGroup
	errs := make([]error, len(jobs))
	for i, j := range jobs {
		group.Add(1)
		go func(slot int, path string, params map[string]string) {
			defer group.Done()
			var out DiscoverMoviesResponse
			if path == "/discover/tv" {
				var tv DiscoverTVResponse
				errs[slot] = client.Get(ctx, path, params, &tv)
				return
			}
			errs[slot] = client.Get(ctx, path, params, &out)
		}(i, j.path, j.params)
	}
	group.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("job %d: %v", i, err)
		}
	}
	if got, want := upstream.Load(), int64(len(jobs)); got != want {
		t.Fatalf("upstream requests = %d, want %d distinct keys", got, want)
	}
}
