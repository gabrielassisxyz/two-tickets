package httpapi

import (
	"bytes"
	"context"
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

	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/library"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// libraryTestTMDB stands in for TMDB: details for any movie or series, and a
// discover page of movies 601 to 603. It counts details requests and fails
// the paths in failing.
type libraryTestTMDB struct {
	mu      sync.Mutex
	failing map[string]bool
	details atomic.Int32
}

func (f *libraryTestTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/discover/movie" {
		_, _ = w.Write([]byte(`{"page":1,"total_pages":1,"total_results":3,"results":[` +
			`{"id":601,"title":"A"},{"id":602,"title":"B"},{"id":603,"title":"C"}]}`))
		return
	}
	f.details.Add(1)
	f.mu.Lock()
	failing := f.failing[r.URL.Path]
	f.mu.Unlock()
	if failing {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if kind == "tv" {
		_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Show %s"}`, id, id)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Movie %s"}`, id, id)
}

type libraryServer struct {
	*authServer
	tmdb       *libraryTestTMDB
	db         *store.Database
	alice, bob string
}

// startLibraryServer is the router as the binary assembles it: the catalog
// and the library over one TMDB client, the catalog reading watchlist state
// from the library, over a migrated temporary state.db with both accounts
// and their bootstrapped lists.
func startLibraryServer(t *testing.T) *libraryServer {
	t.Helper()
	fake := &libraryTestTMDB{failing: map[string]bool{}}
	tmdbServer := httptest.NewServer(fake)
	t.Cleanup(tmdbServer.Close)
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:     tmdbServer.URL,
		Limiter:     rate.NewLimiter(rate.Inf, 1),
		Sleep:       func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		MaxAttempts: 1,
	})
	var db *store.Database
	s := startAuthServerWith(t, nil, func(deps RouterDeps) http.Handler {
		db = deps.Readiness[0].(*store.Database)
		if err := store.BootstrapLibraryLists(context.Background(), db, routerTestEpoch); err != nil {
			t.Fatal(err)
		}
		deps.Catalog = catalog.NewService(client, nil)
		deps.Library = library.NewService(db, deps.Auth, hydrate.New(client), nil, nil)
		deps.Catalog.UseWatchlist(deps.Library)
		return NewRouter(deps)
	})
	return &libraryServer{
		authServer: s, tmdb: fake, db: db,
		alice: s.mustLogin(t, "alice", routerAlicePassword),
		bob:   s.mustLogin(t, "bob", routerBobPassword),
	}
}

// call sends a request with the session token and, for a mutation, the
// allowed origin.
func (s *libraryServer) call(t *testing.T, token, method, path, body string) recordedResponse {
	t.Helper()
	var headers map[string]string
	if method != http.MethodGet {
		headers = withOrigin(nil)
	}
	return s.do(t, authRequest{method: method, path: path, body: body, token: token, headers: headers})
}

func (s *libraryServer) expect(t *testing.T, token, method, path, body string, want int) []byte {
	t.Helper()
	resp := s.call(t, token, method, path, body)
	if resp.status != want {
		t.Fatalf("%s %s %.120s = %d %.300s, want %d", method, path, body, resp.status, resp.body, want)
	}
	if want >= 400 {
		assertErrorShape(t, resp)
	}
	return resp.body
}

func (s *libraryServer) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.Read.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (s *libraryServer) userID(t *testing.T, username string) int64 {
	t.Helper()
	var id int64
	if err := s.db.Read.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (s *libraryServer) listID(t *testing.T, kind string, owner any) int64 {
	t.Helper()
	var id int64
	if err := s.db.Read.QueryRow(`SELECT id FROM lists WHERE kind = ? AND owner_user_id IS ? ORDER BY id LIMIT 1`, kind, owner).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func decodeLibraryAnswer[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %.300s: %v", raw, err)
	}
	return value
}

// Every route of the library table and the three Seerr watchlist routes,
// over HTTP through the session gate and the origin check, with 200, 201,
// 204, 400 and 403 each asserted.
func TestLibraryRoutesAnswerAsSpecified(t *testing.T) {
	s := startLibraryServer(t)
	const movie = `"mediaType":"movie","tmdbId":603,"scope":"movie"`
	s.expect(t, s.alice, http.MethodGet, "/api/v1/library/title/movie/603", "", http.StatusOK)
	s.expect(t, s.alice, http.MethodPut, "/api/v1/library/watch", "{"+movie+"}", http.StatusOK)
	s.expect(t, s.alice, http.MethodPut, "/api/v1/library/rating", "{"+movie+`,"level":"liked","review":"yes"}`, http.StatusOK)
	s.expect(t, s.alice, http.MethodPut, "/api/v1/library/rating", "{"+movie+"}", http.StatusBadRequest)
	state := decodeLibraryAnswer[struct {
		Me struct {
			Status string
			Rating struct{ Level, Review string }
		}
	}](t, s.expect(t, s.alice, http.MethodGet, "/api/v1/library/title/movie/603", "", http.StatusOK))
	if state.Me.Status != "watched" || state.Me.Rating.Level != "liked" || state.Me.Rating.Review != "yes" {
		t.Errorf("title state = %+v", state.Me)
	}
	s.expect(t, s.alice, http.MethodDelete, "/api/v1/library/rating", "{"+movie+"}", http.StatusOK)
	s.expect(t, s.alice, http.MethodDelete, "/api/v1/library/watch", "{"+movie+"}", http.StatusOK)

	s.expect(t, s.alice, http.MethodGet, "/api/v1/library/lists", "", http.StatusOK)
	created := decodeLibraryAnswer[struct{ ID int64 }](t, s.expect(t, s.alice, http.MethodPost, "/api/v1/library/lists", `{"name":"Sci-fi"}`, http.StatusCreated))
	listPath := fmt.Sprintf("/api/v1/library/lists/%d", created.ID)
	s.expect(t, s.alice, http.MethodPatch, listPath, `{"name":"Science fiction"}`, http.StatusOK)
	s.expect(t, s.bob, http.MethodPatch, listPath, `{"name":"mine"}`, http.StatusForbidden)
	s.expect(t, s.alice, http.MethodPut, listPath+"/items", `{"mediaType":"movie","tmdbId":603,"note":"rewatch"}`, http.StatusOK)
	s.expect(t, s.bob, http.MethodPut, listPath+"/items", `{"mediaType":"movie","tmdbId":604}`, http.StatusForbidden)
	items := decodeLibraryAnswer[struct {
		Results []struct {
			TMDBID    int  `json:"tmdbId"`
			Available bool `json:"available"`
		}
		NextCursor *string
	}](t, s.expect(t, s.bob, http.MethodGet, listPath+"/items", "", http.StatusOK))
	if len(items.Results) != 1 || items.Results[0].TMDBID != 603 || !items.Results[0].Available || items.NextCursor != nil {
		t.Errorf("the partner reading the list = %+v", items)
	}
	s.expect(t, s.alice, http.MethodDelete, listPath+"/items/movie/603", "", http.StatusNoContent)
	s.expect(t, s.alice, http.MethodDelete, listPath, "", http.StatusNoContent)
	s.expect(t, s.alice, http.MethodGet, "/api/v1/library/history?level=loved", "", http.StatusOK)

	s.expect(t, s.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":603,"mediaType":"movie","title":"C"}`, http.StatusCreated)
	s.expect(t, s.alice, http.MethodGet, "/api/v1/discover/watchlist?page=1", "", http.StatusOK)
	s.expect(t, s.alice, http.MethodDelete, "/api/v1/watchlist/603?mediaType=movie", "", http.StatusNoContent)
}

// Every library route needs a session, and every mutation the allowed
// origin, because the family registers inside the gate.
func TestLibraryRoutesAreBehindTheSessionGateAndTheOriginCheck(t *testing.T) {
	s := startLibraryServer(t)
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/library/title/movie/1", ""},
		{http.MethodPut, "/api/v1/library/watch", `{"mediaType":"movie","tmdbId":1,"scope":"movie"}`},
		{http.MethodDelete, "/api/v1/library/watch", `{"mediaType":"movie","tmdbId":1,"scope":"movie"}`},
		{http.MethodPut, "/api/v1/library/rating", `{"mediaType":"movie","tmdbId":1,"scope":"movie","level":"meh"}`},
		{http.MethodDelete, "/api/v1/library/rating", `{"mediaType":"movie","tmdbId":1,"scope":"movie"}`},
		{http.MethodGet, "/api/v1/library/lists", ""},
		{http.MethodPost, "/api/v1/library/lists", `{"name":"x"}`},
		{http.MethodPatch, "/api/v1/library/lists/1", `{"name":"x"}`},
		{http.MethodDelete, "/api/v1/library/lists/1", ""},
		{http.MethodGet, "/api/v1/library/lists/1/items", ""},
		{http.MethodPut, "/api/v1/library/lists/1/items", `{"mediaType":"movie","tmdbId":1}`},
		{http.MethodDelete, "/api/v1/library/lists/1/items/movie/1", ""},
		{http.MethodGet, "/api/v1/library/history", ""},
		{http.MethodPost, "/api/v1/watchlist", `{"tmdbId":1,"mediaType":"movie"}`},
		{http.MethodDelete, "/api/v1/watchlist/1?mediaType=movie", ""},
		{http.MethodGet, "/api/v1/discover/watchlist", ""},
	}
	for _, route := range routes {
		resp := s.do(t, authRequest{method: route.method, path: route.path, body: route.body, headers: withOrigin(nil)})
		if resp.status != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route.method, route.path, resp.status)
		}
		if route.method == http.MethodGet {
			continue
		}
		foreign := map[string]string{"Origin": "https://elsewhere.example", "Content-Type": "application/json"}
		if resp := s.do(t, authRequest{method: route.method, path: route.path, body: route.body, token: s.alice, headers: foreign}); resp.status != http.StatusForbidden {
			t.Errorf("%s %s from a foreign origin = %d, want 403", route.method, route.path, resp.status)
		}
	}
	if got := s.count(t, `SELECT (SELECT COUNT(*) FROM watches) + (SELECT COUNT(*) FROM ratings) + (SELECT COUNT(*) FROM list_items) + (SELECT COUNT(*) FROM lists WHERE kind = 'custom')`); got != 0 {
		t.Errorf("refused requests stored %d rows", got)
	}
}

// libraryPartnerSnapshot is everything the partner owns, as rows.
func (s *libraryServer) partnerSnapshot(t *testing.T, partnerID int64) string {
	t.Helper()
	var out strings.Builder
	for _, query := range []string{
		`SELECT media_type, tmdb_id, scope, season_number, watched_at FROM watches WHERE user_id = ? ORDER BY 1, 2, 3, 4`,
		`SELECT media_type, tmdb_id, scope, season_number, level, review, updated_at FROM ratings WHERE user_id = ? ORDER BY 1, 2, 3, 4`,
		`SELECT l.id, l.name, li.media_type, li.tmdb_id FROM lists AS l LEFT JOIN list_items AS li ON li.list_id = l.id WHERE l.owner_user_id = ? ORDER BY 1, 3, 4`,
	} {
		rows, err := s.db.Read.Query(query, partnerID)
		if err != nil {
			t.Fatal(err)
		}
		columns, _ := rows.Columns()
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintln(&out, values...)
		}
		_ = rows.Close()
	}
	return out.String()
}

// In both directions: a mutation naming the partner in the body is refused,
// one naming the partner in the query acts on the caller, the partner's rows
// never change, the partner's state is readable through GET /library/title,
// and every mutation on the partner's lists answers 403.
func TestLibraryPartnerIsolationBothDirections(t *testing.T) {
	s := startLibraryServer(t)
	for _, direction := range []struct{ caller, partner, partnerName string }{
		{s.alice, s.bob, "bob"},
		{s.bob, s.alice, "alice"},
	} {
		partnerID := s.userID(t, direction.partnerName)
		const season = `"mediaType":"tv","tmdbId":1399,"scope":"season","seasonNumber":2`
		s.expect(t, direction.partner, http.MethodPut, "/api/v1/library/watch", "{"+season+"}", http.StatusOK)
		s.expect(t, direction.partner, http.MethodPut, "/api/v1/library/rating", "{"+season+`,"level":"loved"}`, http.StatusOK)
		custom := decodeLibraryAnswer[struct{ ID int64 }](t, s.expect(t, direction.partner, http.MethodPost, "/api/v1/library/lists", `{"name":"Private"}`, http.StatusCreated))
		s.expect(t, direction.partner, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", custom.ID), `{"mediaType":"tv","tmdbId":1399}`, http.StatusOK)
		before := s.partnerSnapshot(t, partnerID)

		for _, path := range []string{"/api/v1/library/watch", "/api/v1/library/rating"} {
			named := fmt.Sprintf(`{%s,"level":"hated","userId":%d}`, season, partnerID)
			s.expect(t, direction.caller, http.MethodDelete, path, named, http.StatusBadRequest)
			s.expect(t, direction.caller, http.MethodPut, path, named, http.StatusBadRequest)
			queried := fmt.Sprintf("%s?userId=%d&user_id=%d", path, partnerID, partnerID)
			s.expect(t, direction.caller, http.MethodDelete, queried, "{"+season+"}", http.StatusOK)
		}
		s.expect(t, direction.caller, http.MethodPut, fmt.Sprintf("/api/v1/library/watch?userId=%d", partnerID), "{"+season+"}", http.StatusOK)
		s.expect(t, direction.caller, http.MethodPut, fmt.Sprintf("/api/v1/library/users/%d/watch", partnerID), "{"+season+"}", http.StatusNotFound)

		partnerWatchlist := s.listID(t, "watchlist", partnerID)
		for _, listID := range []int64{partnerWatchlist, custom.ID} {
			path := fmt.Sprintf("/api/v1/library/lists/%d", listID)
			s.expect(t, direction.caller, http.MethodPatch, path, `{"name":"taken"}`, http.StatusForbidden)
			s.expect(t, direction.caller, http.MethodDelete, path, "", http.StatusForbidden)
			s.expect(t, direction.caller, http.MethodPut, path+"/items", `{"mediaType":"movie","tmdbId":5}`, http.StatusForbidden)
			s.expect(t, direction.caller, http.MethodDelete, path+"/items/tv/1399", "", http.StatusForbidden)
		}

		if after := s.partnerSnapshot(t, partnerID); after != before {
			t.Errorf("the partner's rows changed:\nbefore %s\nafter  %s", before, after)
		}
		seen := decodeLibraryAnswer[struct {
			Me, Partner struct {
				Seasons []struct {
					SeasonNumber int
					Watched      bool
					Rating       *struct{ Level string }
				}
			}
		}](t, s.expect(t, direction.caller, http.MethodGet, "/api/v1/library/title/tv/1399", "", http.StatusOK))
		partnerSeasons := seen.Partner.Seasons
		if len(partnerSeasons) != 1 || !partnerSeasons[0].Watched || partnerSeasons[0].Rating == nil || partnerSeasons[0].Rating.Level != "loved" {
			t.Errorf("the partner's state read by the caller = %+v", seen.Partner)
		}
		if len(seen.Me.Seasons) != 1 || !seen.Me.Seasons[0].Watched || seen.Me.Seasons[0].Rating != nil {
			t.Errorf("the caller's own state = %+v, want only the watch the caller wrote", seen.Me)
		}
		s.expect(t, direction.caller, http.MethodDelete, "/api/v1/library/watch", "{"+season+"}", http.StatusOK)
	}
}

// Many concurrent writes to one row, through HTTP, leave one consistent row
// and every request answers 200.
func TestLibraryConcurrentMutationsLeaveOneConsistentRow(t *testing.T) {
	s := startLibraryServer(t)
	const writers = 16
	couple := s.listID(t, "couple", nil)
	const season = `"mediaType":"tv","tmdbId":42,"scope":"season","seasonNumber":1`
	const show = `"mediaType":"tv","tmdbId":42,"scope":"show"`
	var wg sync.WaitGroup
	statuses := make(chan string, writers*5)
	for i := range writers {
		token := []string{s.alice, s.bob}[i%2]
		wg.Go(func() {
			for _, req := range []struct{ token, method, path, body string }{
				{s.alice, http.MethodPut, "/api/v1/library/watch", "{" + season + "}"},
				{s.alice, http.MethodPut, "/api/v1/library/watch", "{" + show + "}"},
				{s.alice, http.MethodPut, "/api/v1/library/rating", fmt.Sprintf(`{%s,"level":%q,"review":"review %d"}`, season, []string{"meh", "good", "loved"}[i%3], i%3)},
				{token, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", couple), `{"mediaType":"tv","tmdbId":42}`},
				{s.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":42,"mediaType":"tv"}`},
			} {
				resp := s.call(t, req.token, req.method, req.path, req.body)
				statuses <- fmt.Sprintf("%s %s %d", req.method, req.path, resp.status)
			}
		})
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if !strings.HasSuffix(status, " 200") && !strings.HasSuffix(status, " 201") {
			t.Errorf("concurrent write answered %s", status)
		}
	}
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM watches WHERE tmdb_id = 42 AND scope = 'season'`: 1,
		`SELECT COUNT(*) FROM watches WHERE tmdb_id = 42 AND scope = 'show'`:   1,
		`SELECT COUNT(*) FROM ratings WHERE tmdb_id = 42`:                      1,
		`SELECT COUNT(*) FROM list_items WHERE tmdb_id = 42`:                   2,
	} {
		if got := s.count(t, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	var level, review string
	if err := s.db.Read.QueryRow(`SELECT level, review FROM ratings WHERE tmdb_id = 42`).Scan(&level, &review); err != nil {
		t.Fatal(err)
	}
	pairs := map[string]string{"meh": "review 0", "good": "review 1", "loved": "review 2"}
	if pairs[level] != review {
		t.Errorf("rating row = (%s, %s): a level and a review from different requests", level, review)
	}
}

// One title TMDB cannot answer is one placeholder card; the page answers 200.
func TestLibraryPageWithAFailingTitleAnswersAPlaceholder(t *testing.T) {
	s := startLibraryServer(t)
	s.tmdb.mu.Lock()
	s.tmdb.failing["/movie/2"] = true
	s.tmdb.mu.Unlock()
	watchlist := s.listID(t, "watchlist", s.userID(t, "alice"))
	for id := 1; id <= 3; id++ {
		s.expect(t, s.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", watchlist), fmt.Sprintf(`{"mediaType":"movie","tmdbId":%d}`, id), http.StatusOK)
	}
	page := decodeLibraryAnswer[struct {
		Results []struct {
			TMDBID    int                        `json:"tmdbId"`
			Available bool                       `json:"available"`
			Result    map[string]json.RawMessage `json:"result"`
		}
	}](t, s.expect(t, s.alice, http.MethodGet, fmt.Sprintf("/api/v1/library/lists/%d/items", watchlist), "", http.StatusOK))
	if len(page.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(page.Results))
	}
	for _, item := range page.Results {
		if item.Available != (item.TMDBID != 2) {
			t.Errorf("movie %d available = %v", item.TMDBID, item.Available)
		}
		if len(item.Result) != len(page.Results[0].Result) || item.Result["title"] == nil {
			t.Errorf("movie %d card keys = %d, want the same keys as every card", item.TMDBID, len(item.Result))
		}
	}
	if got := string(page.Results[1].Result["title"]); got != `"movie 2"` {
		t.Errorf("placeholder title = %s", got)
	}
}

// The catalog marks the caller's watchlist through the library: on result
// items as mediaInfo.watchlists, on the title page as onUserWatchlist, and
// never for the partner.
func TestCatalogAnswersCarryTheCallersWatchlist(t *testing.T) {
	s := startLibraryServer(t)
	s.expect(t, s.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":602,"mediaType":"movie","title":"B"}`, http.StatusCreated)
	mark := []byte(`"mediaInfo":{"watchlists":[{"tmdbId":602,"mediaType":"movie"}]}`)
	if page := s.expect(t, s.alice, http.MethodGet, "/api/v1/discover/movies?page=1", "", http.StatusOK); bytes.Count(page, mark) != 1 || bytes.Count(page, []byte(`"mediaInfo"`)) != 1 {
		t.Errorf("alice's discover page = %s, want one mediaInfo, on movie 602", page)
	}
	if page := s.expect(t, s.bob, http.MethodGet, "/api/v1/discover/movies?page=1", "", http.StatusOK); bytes.Contains(page, []byte(`"mediaInfo"`)) {
		t.Errorf("bob's discover page carries alice's watchlist: %s", page)
	}
}
