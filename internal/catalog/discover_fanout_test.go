package catalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A genre slider makes one discover call per genre and never has more than
// four of them in flight, so one Discover page cannot take every slot of the
// TMDB client. Each call is held long enough for an unbounded fan-out to put
// every genre in flight at once. The bound is the bead's literal four, not the
// constant, so that changing the constant fails here.
func TestDiscoverGenreSliderBoundsTheCallsInFlight(t *testing.T) {
	const genreCount, maxInFlight = 12, 4
	genres := make([]string, 0, genreCount)
	for id := 1; id <= genreCount; id++ {
		genres = append(genres, fmt.Sprintf(`{"id":%d,"name":"Genre %02d"}`, id, id))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"genres":[` + strings.Join(genres, ",") + `]}`))
	}))
	defer server.Close()
	service := NewService(discoverTestClient(server.URL), nil)

	var calls, inFlight, peak atomic.Int32
	backdropsOf := func(ctx context.Context, genreID int) ([]string, error) {
		calls.Add(1)
		now := inFlight.Add(1)
		for seen := peak.Load(); now > seen && !peak.CompareAndSwap(seen, now); seen = peak.Load() {
		}
		time.Sleep(50 * time.Millisecond)
		inFlight.Add(-1)
		return nil, nil
	}
	if _, err := service.discoverGenreSlider(context.Background(), "/genre/movie/list", backdropsOf); err != nil {
		t.Fatalf("genre slider: %v", err)
	}
	if got := calls.Load(); got != genreCount {
		t.Fatalf("discover calls = %d, want one per genre (%d)", got, genreCount)
	}
	if got := peak.Load(); got > maxInFlight {
		t.Errorf("calls in flight peaked at %d, want at most %d", got, maxInFlight)
	}
}
