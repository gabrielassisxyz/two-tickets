package hydrate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// countingTMDB stands in for TMDB's movie and series details. It counts every
// request, records the most it ever had in flight, holds each request for
// hold so that an unbounded fan-out would put the whole page in flight at
// once, and answers 500 for the ids in failing.
type countingTMDB struct {
	hold    time.Duration
	failing map[string]bool
	// stall, when set, holds every request until the client gives up.
	stall bool

	calls, inFlight, peak atomic.Int32
	mu                    sync.Mutex
	paths                 []string
}

func (f *countingTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	now := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for seen := f.peak.Load(); now > seen && !f.peak.CompareAndSwap(seen, now); seen = f.peak.Load() {
	}
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()
	if f.stall {
		<-r.Context().Done()
		return
	}
	time.Sleep(f.hold)
	if f.failing[r.URL.Path] {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status_message":"boom"}`))
		return
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	w.Header().Set("Content-Type", "application/json")
	if kind == "tv" {
		_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Show %s","original_name":"Show %s","poster_path":"/tv%s.jpg","genres":[{"id":18,"name":"Drama"}],"origin_country":["BR"]}`, id, id, id, id)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Movie %s","original_title":"Movie %s","poster_path":"/m%s.jpg","genres":[{"id":28,"name":"Action"}]}`, id, id, id, id)
}

// newCountingHydrator has no response cache, so every title is cache-cold,
// and tries each request once.
func newCountingHydrator(t *testing.T, fake *countingTMDB) *Hydrator {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:        server.URL,
		Limiter:        rate.NewLimiter(rate.Inf, 1),
		Sleep:          func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		MaxAttempts:    1,
		AttemptTimeout: 5 * time.Second,
		MaxConcurrency: 10,
	})
	return New(client)
}

func pageOfTitles(count int) []Title {
	titles := make([]Title, 0, count)
	for i := 1; i <= count; i++ {
		mediaType := "movie"
		if i%2 == 0 {
			mediaType = "tv"
		}
		titles = append(titles, Title{MediaType: mediaType, TMDBID: 1000 + i})
	}
	return titles
}

// A full page of 20 distinct cache-cold titles makes at most 20 detail calls
// and never more than 4 at once. Both bounds are the bead's literals, not the
// constants that implement them, so changing a constant fails here.
func TestCardsFanOutOfAFullPageIsBounded(t *testing.T) {
	const pageItems, maxDetailCalls, maxInFlight = 20, 20, 4
	fake := &countingTMDB{hold: 40 * time.Millisecond}
	hydrator := newCountingHydrator(t, fake)
	titles := pageOfTitles(pageItems)

	cards := hydrator.Cards(context.Background(), titles)

	if len(cards) != pageItems {
		t.Fatalf("cards = %d, want %d", len(cards), pageItems)
	}
	if got := fake.calls.Load(); got > maxDetailCalls {
		t.Errorf("detail calls = %d, want at most %d", got, maxDetailCalls)
	}
	if got := fake.peak.Load(); got > maxInFlight {
		t.Errorf("detail calls in flight peaked at %d, want at most %d", got, maxInFlight)
	}
	for i, card := range cards {
		if !card.Available || card.TMDBID != titles[i].TMDBID || card.MediaType != titles[i].MediaType {
			t.Errorf("card %d = %+v, want the available card of %s", i, card, titles[i])
		}
	}
}

// However many titles a caller hands over, one call never costs more than a
// page's worth of detail calls: the worst case is the bound, not the input.
func TestCardsNeverFetchesMoreThanAPage(t *testing.T) {
	const maxDetailCalls = 20
	fake := &countingTMDB{}
	hydrator := newCountingHydrator(t, fake)
	titles := pageOfTitles(25)
	titles = append(titles, titles[0], titles[1])

	cards := hydrator.Cards(context.Background(), titles)

	if got := fake.calls.Load(); got > maxDetailCalls {
		t.Fatalf("detail calls = %d for %d titles, want at most %d", got, len(titles), maxDetailCalls)
	}
	if len(cards) != len(titles) {
		t.Fatalf("cards = %d, want one per title (%d)", len(cards), len(titles))
	}
	if !cards[len(cards)-1].Available {
		t.Errorf("a repeated title in the page was not hydrated: %+v", cards[len(cards)-1])
	}
}

// A history page holds one row per season, so one series can fill most of
// it. Each distinct title costs one detail call, however many rows name it.
func TestCardsFetchesEachDistinctTitleOnce(t *testing.T) {
	fake := &countingTMDB{hold: 40 * time.Millisecond}
	hydrator := newCountingHydrator(t, fake)
	titles := make([]Title, 0, 20)
	for range 11 {
		titles = append(titles, Title{MediaType: "tv", TMDBID: 500})
	}
	titles = append(titles, pageOfTitles(9)...)

	cards := hydrator.Cards(context.Background(), titles)

	if got := fake.calls.Load(); got != 10 {
		t.Errorf("detail calls = %d for 10 distinct titles, want 10", got)
	}
	for i, card := range cards {
		if !card.Available || card.TMDBID != titles[i].TMDBID {
			t.Errorf("card %d = %+v, want %s", i, card, titles[i])
		}
	}
}

// One failing title is one placeholder card with the same keys as a real
// card; the rest of the page is unaffected.
func TestCardsFailingTitleIsAPlaceholderWithTheSameKeys(t *testing.T) {
	fake := &countingTMDB{failing: map[string]bool{"/movie/1003": true, "/tv/1004": true}}
	hydrator := newCountingHydrator(t, fake)
	titles := pageOfTitles(6)

	cards := hydrator.Cards(context.Background(), titles)

	for i, card := range cards {
		failing := titles[i].TMDBID == 1003 || titles[i].TMDBID == 1004
		if card.Available == failing {
			t.Errorf("card %s available = %v, want %v", titles[i], card.Available, !failing)
		}
	}
	assertSameKeys(t, cards[0], cards[2])
	assertSameKeys(t, cards[1], cards[3])
	if got := cards[2].DisplayTitle(); got != "movie 1003" {
		t.Errorf("placeholder title = %q, want %q", got, "movie 1003")
	}
	if got := cards[0].DisplayTitle(); got != "Movie 1001" {
		t.Errorf("hydrated title = %q, want TMDB's", got)
	}
}

// A page whose titles TMDB never answers returns placeholders once the page
// deadline passes, instead of waiting out the client's own retry budget.
func TestCardsPageDeadlineEndsAStalledPage(t *testing.T) {
	fake := &countingTMDB{stall: true}
	hydrator := newCountingHydrator(t, fake)
	hydrator.deadline = 150 * time.Millisecond
	titles := pageOfTitles(10)

	started := time.Now()
	cards := hydrator.Cards(context.Background(), titles)
	elapsed := time.Since(started)

	if elapsed > 3*time.Second {
		t.Fatalf("a stalled page took %v, want it to end at the deadline", elapsed)
	}
	for _, card := range cards {
		if card.Available {
			t.Fatalf("a stalled title is available: %+v", card)
		}
	}
	if got := fake.calls.Load(); got > 4 {
		t.Errorf("a stalled page started %d calls, want only the 4 that held a slot", got)
	}
}

func TestPageDeadlineIsShorterThanTheClientBudget(t *testing.T) {
	if hydratePageDeadline <= 0 || hydratePageDeadline > 10*time.Second {
		t.Fatalf("page deadline = %v, want a positive bound well under the TMDB client's 30 s budget", hydratePageDeadline)
	}
}

func TestCardSerialisesTheSharedResultShape(t *testing.T) {
	fake := &countingTMDB{}
	hydrator := newCountingHydrator(t, fake)
	cards := hydrator.Cards(context.Background(), []Title{{MediaType: "movie", TMDBID: 7}})
	raw, err := json.Marshal(cards[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"mediaType", "tmdbId", "available", "result"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("card %s has no %q", raw, key)
		}
	}
	want, _ := json.Marshal(catalog.MapSearchResults([]tmdb.MediaItem{catalog.MapMovieDetailsToResult(tmdb.MovieDetails{
		MovieBrief: tmdb.MovieBrief{ID: 7, Title: "Movie 7", OriginalTitle: "Movie 7", PosterPath: ptr("/m7.jpg")},
		Genres:     []tmdb.Genre{{ID: 28, Name: "Action"}},
	})})[0])
	if string(decoded["result"]) != string(want) {
		t.Errorf("result = %s, want the shared search result %s", decoded["result"], want)
	}
}

func ptr(s string) *string { return &s }

func assertSameKeys(t *testing.T, real, placeholder Card) {
	t.Helper()
	keysOf := func(card Card) []string {
		raw, err := json.Marshal(card.Result)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(decoded))
		for key := range decoded {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		return keys
	}
	if got, want := keysOf(placeholder), keysOf(real); !slices.Equal(got, want) {
		t.Errorf("placeholder keys %v, want the real card's %v", got, want)
	}
}
