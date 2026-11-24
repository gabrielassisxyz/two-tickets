package catalog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// discoverPageCounter stands between the client and FakeTMDB. It records
// when each upstream request arrived and under which query, then asks
// FakeTMDB for the same request on page 1: only first pages are recorded, and
// FakeTMDB still checks every other parameter against the fixtures.
type discoverPageCounter struct {
	fake *httptest.Server

	mu       sync.Mutex
	arrivals []time.Time
	perQuery map[string]int
}

func (c *discoverPageCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.arrivals = append(c.arrivals, time.Now())
	query := r.URL.Query()
	query.Del("api_key")
	c.perQuery[r.URL.Path+"?"+query.Encode()]++
	c.mu.Unlock()

	if query.Has("page") {
		query.Set("page", "1")
	}
	response, err := http.Get(c.fake.URL + r.URL.Path + "?" + query.Encode())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = response.Body.Close() }()
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

// discoverPageLoad is one Discover visit: every slider that calls the api,
// two pages each, the two genre sliders, and one browse page three pages
// deep. Studios and networks are hard-coded in the frontend and call nothing.
var discoverPageLoad = []string{
	"/api/v1/discover/trending?page=1", "/api/v1/discover/trending?page=2",
	"/api/v1/discover/movies?page=1", "/api/v1/discover/movies?page=2",
	"/api/v1/discover/movies/upcoming?page=1", "/api/v1/discover/movies/upcoming?page=2",
	"/api/v1/discover/tv?page=1", "/api/v1/discover/tv?page=2",
	"/api/v1/discover/tv/upcoming?page=1", "/api/v1/discover/tv/upcoming?page=2",
	"/api/v1/discover/genreslider/movie",
	"/api/v1/discover/genreslider/tv",
	"/api/v1/discover/movies/studio/2?page=1", "/api/v1/discover/movies/studio/2?page=2", "/api/v1/discover/movies/studio/2?page=3",
}

// discoverPageLoadKeys is how many distinct cache keys that visit needs:
// ten slider pages, a genre list and four genres per genre slider, the
// studio and its three pages.
const discoverPageLoadKeys = 10 + (1 + 4) + (1 + 4) + (1 + 3)

func TestDiscoverConcurrentLoadMakesOneCallPerCacheKeyWithinTheLimiter(t *testing.T) {
	fake := catalogtest.FakeTMDB(t,
		"trending/all-day", "discover/movies-popular", "discover/movies-upcoming",
		"discover/tv-popular", "discover/tv-upcoming",
		"genre/movie-list-slider", "discover/movies-slider-backdrops-0", "discover/movies-slider-backdrops-1",
		"discover/movies-slider-backdrops-4", "discover/movies-slider-backdrops-5",
		"genre/tv-list-slider", "discover/tv-slider-backdrops-0", "discover/tv-slider-backdrops-1",
		"discover/tv-slider-backdrops-4", "discover/tv-slider-backdrops-5",
		"company/walt-disney-pictures", "discover/movies-studio-disney",
	)
	counter := &discoverPageCounter{fake: fake, perQuery: map[string]int{}}
	upstream := httptest.NewServer(counter)
	t.Cleanup(upstream.Close)

	cache, err := store.OpenCacheStore(t.Context(), filepath.Join(t.TempDir(), store.CacheFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	// The production limiter, 20 requests per second with a burst of one.
	client := tmdb.NewClient(tmdb.Options{
		BaseURL: upstream.URL,
		Cache:   cache,
		Limiter: rate.NewLimiter(rate.Every(50*time.Millisecond), 1),
	})
	service := NewService(client, func() time.Time { return discoverFrozenNow })

	// Both accounts open Discover at the same moment.
	var wg sync.WaitGroup
	for range 2 {
		for _, target := range discoverPageLoad {
			wg.Go(func() {
				if response := discoverServe(t, service, http.MethodGet, target); response.Code != http.StatusOK {
					t.Errorf("%s = %d %s", target, response.Code, response.Body)
				}
			})
		}
	}
	wg.Wait()

	counter.mu.Lock()
	defer counter.mu.Unlock()
	if len(counter.perQuery) != discoverPageLoadKeys {
		t.Errorf("the load asked TMDB %d distinct requests, want %d: %v", len(counter.perQuery), discoverPageLoadKeys, counter.perQuery)
	}
	if len(counter.arrivals) > discoverPageLoadKeys {
		t.Errorf("the load made %d upstream calls for %d cache keys", len(counter.arrivals), discoverPageLoadKeys)
	}
	for query, calls := range counter.perQuery {
		if calls != 1 {
			t.Errorf("TMDB was asked %s %d times", query, calls)
		}
	}
	discoverRequireWithinLimiter(t, counter.arrivals)
}

// discoverRequireWithinLimiter fails when more than 21 calls fall inside one
// second: at 20 per second with a burst of one, the 1st and the 21st call
// are at least a second apart. The slack covers the time between the
// limiter letting a call go and the call arriving.
func discoverRequireWithinLimiter(t *testing.T, arrivals []time.Time) {
	t.Helper()
	const perSecond, slack = 20, 100 * time.Millisecond
	sorted := slices.Clone(arrivals)
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })
	if len(sorted) <= perSecond {
		t.Fatalf("only %d upstream calls; the load is too small to test the limiter", len(sorted))
	}
	for i := 0; i+perSecond < len(sorted); i++ {
		if gap := sorted[i+perSecond].Sub(sorted[i]); gap < time.Second-slack {
			t.Errorf("calls %d to %d arrived within %v, faster than %d per second", i+1, i+perSecond+1, gap, perSecond)
			return
		}
	}
}
