package recs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// forYouFakeTMDB stands in for TMDB: recommendations for any movie or
// series seed, the weekly trending list, and details for any id. failing
// paths answer 500. inFlight tracks concurrent recommendation fetches.
type forYouFakeTMDB struct {
	mu       sync.Mutex
	failing  map[string]bool
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	delay    time.Duration
}

func (f *forYouFakeTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	f.mu.Lock()
	failing := f.failing[path]
	f.mu.Unlock()
	if failing {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.HasSuffix(path, "/recommendations") {
		current := f.inFlight.Add(1)
		for {
			seen := f.maxSeen.Load()
			if current <= seen || f.maxSeen.CompareAndSwap(seen, current) {
				break
			}
		}
		defer f.inFlight.Add(-1)
		if f.delay > 0 {
			time.Sleep(f.delay)
		}
		trimmed := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/recommendations")
		kind, id, _ := strings.Cut(trimmed, "/")
		var seed int
		_, _ = fmt.Sscanf(id, "%d", &seed)
		base, title := 1000, "title"
		if kind == "tv" {
			base, title = 2000, "name"
		}
		items := make([]string, 0, 3)
		for k := range 3 {
			items = append(items, fmt.Sprintf(`{"id":%d,"%s":"Candidate %d","vote_average":6.5,"vote_count":0}`, base+seed*10+k, title, base+seed*10+k))
		}
		_, _ = fmt.Fprintf(w, `{"page":1,"total_pages":1,"total_results":3,"results":[%s]}`, strings.Join(items, ","))
		return
	}
	if path == "/trending/all/week" {
		_, _ = w.Write([]byte(`{"page":1,"total_pages":1,"total_results":4,"results":[` +
			`{"id":9001,"media_type":"movie","title":"Trend One"},` +
			`{"id":9002,"media_type":"tv","name":"Trend Two"},` +
			`{"id":9003,"media_type":"movie","title":"Trend Three"},` +
			`{"id":9004,"media_type":"person","name":"Trend Person"}]}`))
		return
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if kind == "tv" {
		_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Show %s"}`, id, id)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Movie %s"}`, id, id)
}

type forYouServer struct {
	fake *forYouFakeTMDB
	db   *store.Database
	svc  *Service
}

func startForYouServer(t *testing.T, fake *forYouFakeTMDB) *forYouServer {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:     server.URL,
		Limiter:     rate.NewLimiter(rate.Inf, 1),
		Sleep:       func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		MaxAttempts: 1,
	})
	db := newRecsTestDatabase(t)
	svc := NewService(db, client, hydrate.New(client), nil, nil, nil)
	return &forYouServer{fake: fake, db: db, svc: svc}
}

func forYouUser(id int64) auth.User {
	return auth.User{ID: id, DisplayName: fmt.Sprintf("user %d", id)}
}

func (s *forYouServer) get(t *testing.T, user auth.User, target string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()
	s.svc.handleRecommendations(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func (s *forYouServer) postExposures(t *testing.T, user auth.User, body string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/exposures", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()
	s.svc.handleExposures(rec, req)
	return rec.Code
}

func forYouInsertRating(t *testing.T, db *sql.DB, userID int64, mediaType string, tmdbID int64, scope, level string, updatedAt int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO ratings (user_id, media_type, tmdb_id, scope, level, scale, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'sentiment-7-v1', ?, ?)`, userID, mediaType, tmdbID, scope, level, updatedAt, updatedAt); err != nil {
		t.Fatalf("insert rating: %v", err)
	}
}

func forYouInsertWatch(t *testing.T, db *sql.DB, userID int64, mediaType string, tmdbID int64, scope string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO watches (user_id, media_type, tmdb_id, scope, watched_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, mediaType, tmdbID, scope, recsFixtureTime, recsFixtureTime); err != nil {
		t.Fatalf("insert watch: %v", err)
	}
}

type forYouAnswer struct {
	Page         int   `json:"page"`
	TotalPages   int   `json:"totalPages"`
	TotalResults int   `json:"totalResults"`
	Personal     bool  `json:"personal"`
	RunID        int64 `json:"runId"`
	Results      []struct {
		MediaType   string         `json:"mediaType"`
		TmdbID      int64          `json:"tmdbId"`
		Available   bool           `json:"available"`
		Reasons     []string       `json:"reasons"`
		Rank        int64          `json:"rank"`
		RunID       int64          `json:"runId"`
		OnWatchlist bool           `json:"onWatchlist"`
		Result      map[string]any `json:"result"`
	} `json:"results"`
}

func decodeForYouAnswer(t *testing.T, raw []byte) forYouAnswer {
	t.Helper()
	var answer forYouAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("decode %.300s: %v", raw, err)
	}
	return answer
}

// With two positive seeds the response is the cold-start list flagged
// popular; with three it is personal.
func TestForYouColdStartBelowThreeSeeds(t *testing.T) {
	s := startForYouServer(t, &forYouFakeTMDB{failing: map[string]bool{}})
	user := forYouUser(recsUserA)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 1, "movie", "loved", 100)
	forYouInsertRating(t, s.db.Write, recsUserA, "tv", 2, "show", "liked", 200)

	code, raw := s.get(t, user, "/api/v1/recommendations?mediaType=all&page=1")
	if code != http.StatusOK {
		t.Fatalf("GET recommendations = %d %.300s", code, raw)
	}
	answer := decodeForYouAnswer(t, raw)
	if answer.Personal {
		t.Errorf("two seeds answer personal, want the popular cold start")
	}
	if answer.RunID == 0 {
		t.Fatal("cold start stores no run")
	}
	// Trending carries four rows: three titles and a person, who never
	// recommends.
	if answer.TotalResults != 3 {
		t.Fatalf("cold-start total = %d, want the 3 trending titles: %+v", answer.TotalResults, answer)
	}
	for _, result := range answer.Results {
		if len(result.Reasons) != 0 {
			t.Errorf("cold-start reason = %q, want none", result.Reasons)
		}
		if result.RunID != answer.RunID || result.Rank <= 0 {
			t.Errorf("cold-start identity = run %d rank %d", result.RunID, result.Rank)
		}
	}

	// A second read without changes serves the same run.
	_, raw = s.get(t, user, "/api/v1/recommendations")
	if again := decodeForYouAnswer(t, raw); again.RunID != answer.RunID {
		t.Errorf("second read = run %d, want the stored run %d", again.RunID, answer.RunID)
	}

	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 3, "movie", "loved", 300)
	_, raw = s.get(t, user, "/api/v1/recommendations")
	personal := decodeForYouAnswer(t, raw)
	if !personal.Personal {
		t.Fatal("three seeds still answer popular")
	}
	if personal.RunID == answer.RunID {
		t.Error("the third seed recomputed nothing")
	}
	seeds := map[string]bool{"Movie 1": true, "Show 2": true, "Movie 3": true}
	for _, result := range personal.Results {
		if len(result.Reasons) == 0 || len(result.Reasons) > 3 {
			t.Errorf("result %d reasons = %q, want 1-3 real seeds", result.TmdbID, result.Reasons)
		}
		for _, reason := range result.Reasons {
			if !seeds[reason] {
				t.Errorf("result %d reason %q is not a contributing seed", result.TmdbID, reason)
			}
		}
		if !result.Available {
			t.Errorf("result %d is a placeholder", result.TmdbID)
		}
	}
}

// Watched or rated titles never appear, in cold start too, and a title
// watched after a run is gone at the next read.
func TestForYouExcludesWatchedAndRated(t *testing.T) {
	s := startForYouServer(t, &forYouFakeTMDB{failing: map[string]bool{}})
	user := forYouUser(recsUserA)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 1, "movie", "loved", 100)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 2, "movie", "loved", 200)
	// 9001 trends but is already watched; 9002 trends but is already rated.
	forYouInsertWatch(t, s.db.Write, recsUserA, "movie", 9001, "movie")
	forYouInsertRating(t, s.db.Write, recsUserA, "tv", 9002, "show", "meh", 300)
	_, raw := s.get(t, user, "/api/v1/recommendations")
	answer := decodeForYouAnswer(t, raw)
	if answer.Personal {
		t.Fatal("two positive seeds answer personal")
	}
	for _, result := range answer.Results {
		if (result.MediaType == "movie" && result.TmdbID == 9001) || (result.MediaType == "tv" && result.TmdbID == 9002) {
			t.Errorf("watched or rated %s/%d is recommended", result.MediaType, result.TmdbID)
		}
	}
	if answer.TotalResults != 1 {
		t.Fatalf("cold-start total = %d, want only 9003: %+v", answer.TotalResults, answer)
	}

	// Personal too: seed movie 3, whose candidates include rated movie 2's
	// shadow... instead watch the top candidate and see it vanish.
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 3, "movie", "loved", 400)
	_, raw = s.get(t, user, "/api/v1/recommendations")
	before := decodeForYouAnswer(t, raw)
	if !before.Personal || len(before.Results) == 0 {
		t.Fatalf("three seeds answer = %+v, want a personal list", before)
	}
	top := before.Results[0]
	topScope := top.MediaType
	if topScope == "tv" {
		topScope = "show"
	}
	forYouInsertWatch(t, s.db.Write, recsUserA, top.MediaType, top.TmdbID, topScope)
	_, raw = s.get(t, user, "/api/v1/recommendations")
	after := decodeForYouAnswer(t, raw)
	if after.RunID == before.RunID {
		t.Fatal("watching a recommendation recomputed nothing")
	}
	for _, result := range after.Results {
		if result.MediaType == top.MediaType && result.TmdbID == top.TmdbID {
			t.Errorf("watched %s/%d is still recommended", top.MediaType, top.TmdbID)
		}
	}
}

// A failed run leaves the previously published run served, and the media
// type filter keeps totals and pages right.
func TestForYouFailureKeepsThePublishedRun(t *testing.T) {
	fake := &forYouFakeTMDB{failing: map[string]bool{}}
	s := startForYouServer(t, fake)
	user := forYouUser(recsUserA)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 1, "movie", "loved", 100)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 2, "movie", "loved", 200)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 3, "movie", "loved", 300)
	_, raw := s.get(t, user, "/api/v1/recommendations")
	published := decodeForYouAnswer(t, raw)
	if !published.Personal {
		t.Fatalf("first compute = %+v, want personal", published)
	}

	fake.mu.Lock()
	fake.failing["/movie/3/recommendations"] = true
	fake.mu.Unlock()
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 4, "movie", "loved", 400)
	code, raw := s.get(t, user, "/api/v1/recommendations")
	if code != http.StatusOK {
		t.Fatalf("GET during a TMDB outage = %d %.200s", code, raw)
	}
	fallback := decodeForYouAnswer(t, raw)
	if fallback.RunID != published.RunID {
		t.Errorf("outage served run %d, want the published run %d", fallback.RunID, published.RunID)
	}
	if len(fallback.Results) != len(published.Results) {
		t.Errorf("outage serves %d results, want the published %d", len(fallback.Results), len(published.Results))
	}

	// The filter counts a type before paginating it.
	fake.mu.Lock()
	delete(fake.failing, "/movie/3/recommendations")
	fake.mu.Unlock()
	code, raw = s.get(t, user, "/api/v1/recommendations?mediaType=movie&page=1")
	if code != http.StatusOK {
		t.Fatalf("movie page = %d %.200s", code, raw)
	}
	movies := decodeForYouAnswer(t, raw)
	movieCount := 0
	_, raw = s.get(t, user, "/api/v1/recommendations?mediaType=all&page=1")
	all := decodeForYouAnswer(t, raw)
	for _, result := range all.Results {
		if result.MediaType == "movie" {
			movieCount++
		}
	}
	if movies.TotalResults != movieCount {
		t.Errorf("movie total = %d, want the %d movies of the mixed list", movies.TotalResults, movieCount)
	}
	for _, result := range movies.Results {
		if result.MediaType != "movie" {
			t.Errorf("movie filter returned %s/%d", result.MediaType, result.TmdbID)
		}
	}
	code, _ = s.get(t, user, "/api/v1/recommendations?mediaType=book&page=1")
	if code != http.StatusBadRequest {
		t.Errorf("mediaType=book = %d, want 400", code)
	}
}

// Twelve seeds through eight slots never exceed the bound.
func TestForYouSeedFetchesStayWithinEight(t *testing.T) {
	fake := &forYouFakeTMDB{failing: map[string]bool{}, delay: 20 * time.Millisecond}
	s := startForYouServer(t, fake)
	for i := int64(1); i <= 12; i++ {
		forYouInsertRating(t, s.db.Write, recsUserA, "movie", i, "movie", "loved", i)
	}
	if code, raw := s.get(t, forYouUser(recsUserA), "/api/v1/recommendations"); code != http.StatusOK {
		t.Fatalf("GET = %d %.200s", code, raw)
	}
	if max := fake.maxSeen.Load(); max > 8 {
		t.Errorf("max concurrent seed fetches = %d, want at most 8", max)
	}
}

// Exposures are idempotent per rank, and a foreign or missing run records
// nothing.
func TestForYouExposuresBelongToTheSessionUser(t *testing.T) {
	s := startForYouServer(t, &forYouFakeTMDB{failing: map[string]bool{}})
	alice, bob := forYouUser(recsUserA), forYouUser(recsUserB)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 1, "movie", "loved", 100)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 2, "movie", "loved", 200)
	forYouInsertRating(t, s.db.Write, recsUserA, "movie", 3, "movie", "loved", 300)
	_, raw := s.get(t, alice, "/api/v1/recommendations")
	answer := decodeForYouAnswer(t, raw)
	if len(answer.Results) == 0 {
		t.Fatal("no results to expose")
	}
	rank := answer.Results[0].Rank
	body := fmt.Sprintf(`{"runId":%d,"ranks":[%d]}`, answer.RunID, rank)
	if code := s.postExposures(t, alice, body); code != http.StatusNoContent {
		t.Fatalf("first exposure = %d, want 204", code)
	}
	if code := s.postExposures(t, alice, body); code != http.StatusNoContent {
		t.Fatalf("repeat exposure = %d, want 204", code)
	}
	var n int
	if err := s.db.Read.QueryRow(`SELECT count(*) FROM rec_exposures WHERE run_id = ? AND rank = ?`, answer.RunID, rank).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d rows for one rank, want 1", n)
	}
	if code := s.postExposures(t, bob, body); code != http.StatusNotFound {
		t.Errorf("another user's run = %d, want 404", code)
	}
	if code := s.postExposures(t, alice, `{"runId":9999,"ranks":[1]}`); code != http.StatusNotFound {
		t.Errorf("unknown run = %d, want 404", code)
	}
	if code := s.postExposures(t, alice, fmt.Sprintf(`{"runId":%d,"ranks":[9999]}`, answer.RunID)); code != http.StatusNotFound {
		t.Errorf("unknown rank = %d, want 404", code)
	}
	if code := s.postExposures(t, alice, fmt.Sprintf(`{"runId":%d,"ranks":[1],"userId":2}`, answer.RunID)); code != http.StatusBadRequest {
		t.Errorf("a userId in the body = %d, want 400", code)
	}
	var total int
	if err := s.db.Read.QueryRow(`SELECT count(*) FROM rec_exposures`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("%d exposures stored, want only the one legitimate row", total)
	}
}
