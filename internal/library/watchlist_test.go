package library

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
)

// POST answers 201 with the created item, and repeating it answers 201 with
// the same item and leaves one row.
func TestSeerrWatchlistAddIsIdempotentAndAnswersTheItem(t *testing.T) {
	kit := newLibraryKit(t)
	const body = `{"tmdbId":603,"mediaType":"movie","title":"The Matrix"}`
	first := kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", body, http.StatusCreated)
	item := decodeLibraryJSON[seerrWatchlistItem](t, first)
	want := seerrWatchlistItem{ID: 603, RatingKey: "movie-603", TMDBID: 603, MediaType: "movie", Title: "The Matrix",
		CreatedAt: "2026-03-01T12:00:00.000Z", UpdatedAt: "2026-03-01T12:00:00.000Z"}
	if item != want {
		t.Errorf("POST answered %+v, want %+v", item, want)
	}
	kit.clock.Advance(time.Hour)
	if again := kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", body, http.StatusCreated); !bytes.Equal(again, first) {
		t.Errorf("repeated POST = %s, want the same item %s", again, first)
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM list_items`); got != 1 {
		t.Errorf("rows after adding twice = %d, want 1", got)
	}
	kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":603,"mediaType":"person"}`, http.StatusBadRequest)
}

func TestSeerrWatchlistRemoveIsIdempotent(t *testing.T) {
	kit := newLibraryKit(t)
	kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":1399,"mediaType":"tv","title":"x"}`, http.StatusCreated)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":1399,"mediaType":"tv","title":"x"}`, http.StatusCreated)
	for range 2 {
		resp := kit.serve(t, kit.alice, http.MethodDelete, "/api/v1/watchlist/1399?mediaType=tv", "")
		if resp.status != http.StatusNoContent || len(resp.body) != 0 {
			t.Errorf("DELETE = %d %q, want 204 with no body", resp.status, resp.body)
		}
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM list_items`); got != 1 {
		t.Errorf("rows = %d, want only the partner's item left", got)
	}
	for _, query := range []string{"", "?mediaType=person"} {
		resp := kit.serve(t, kit.alice, http.MethodDelete, "/api/v1/watchlist/1399"+query, "")
		got := decodeLibraryJSON[struct{ Message string }](t, resp.body)
		if resp.status != http.StatusBadRequest || got.Message != "Invalid mediaType query parameter." {
			t.Errorf("DELETE%s = %d %s, want upstream's 400", query, resp.status, resp.body)
		}
	}
}

// /discover/watchlist pages 20 at a time in list order, every item carries
// ratingKey in the local <mediaType>-<tmdbId> form and id equal to the TMDB
// id, and a title TMDB cannot answer falls back to the placeholder.
func TestSeerrDiscoverWatchlistPages(t *testing.T) {
	kit := newLibraryKit(t)
	kit.tmdb.failing["/tv/7"] = true
	for id := 1; id <= 25; id++ {
		mediaType := []string{"movie", "tv"}[id%2]
		kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", fmt.Sprintf(`{"tmdbId":%d,"mediaType":%q}`, id, mediaType), http.StatusCreated)
		kit.clock.Advance(time.Second)
	}
	type entry = seerrWatchlistEntry
	first := decodeLibraryJSON[catalog.ResultsPage[entry]](t, kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/discover/watchlist?page=1", "", http.StatusOK))
	second := decodeLibraryJSON[catalog.ResultsPage[entry]](t, kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/discover/watchlist?page=2", "", http.StatusOK))
	if first.TotalResults != 25 || first.TotalPages != 2 || len(first.Results) != 20 || len(second.Results) != 5 || second.Page != 2 {
		t.Fatalf("pages = %d/%d results %d+%d", first.TotalPages, first.TotalResults, len(first.Results), len(second.Results))
	}
	all := append(first.Results, second.Results...)
	for i, item := range all {
		wantID := 25 - i
		wantType := []string{"movie", "tv"}[wantID%2]
		if item.ID != wantID || item.TMDBID != wantID || item.MediaType != wantType || item.RatingKey != fmt.Sprintf("%s-%d", wantType, wantID) {
			t.Errorf("item %d = %+v, want %s %d newest first", i, item, wantType, wantID)
		}
		wantTitle := map[string]string{"movie": "Movie %d", "tv": "Show %d"}[wantType]
		if wantID == 7 {
			wantTitle = "tv %d"
		}
		if item.Title != fmt.Sprintf(wantTitle, wantID) {
			t.Errorf("item %d title = %q, want %q", wantID, item.Title, fmt.Sprintf(wantTitle, wantID))
		}
	}
	empty := decodeLibraryJSON[catalog.ResultsPage[entry]](t, kit.mustServe(t, kit.bob, http.MethodGet, "/api/v1/discover/watchlist", "", http.StatusOK))
	if empty.TotalResults != 0 || empty.Results == nil || len(empty.Results) != 0 {
		t.Errorf("the partner's own watchlist page = %+v, want empty", empty)
	}
	kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/discover/watchlist?page=0", "", http.StatusBadRequest)
}

// The catalog's membership question is answered for the caller's watchlist
// only, never the partner's, and without an account nothing is on one.
func TestOnCallerWatchlistAnswersForTheCallerOnly(t *testing.T) {
	kit := newLibraryKit(t)
	kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":1,"mediaType":"movie"}`, http.StatusCreated)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":2,"mediaType":"movie"}`, http.StatusCreated)
	kit.mustServe(t, kit.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", kit.listIDOf(t, listKindCouple, 0)), `{"mediaType":"movie","tmdbId":3}`, http.StatusOK)
	titles := []catalog.WatchlistTitle{{MediaType: "movie", TMDBID: 1}, {MediaType: "movie", TMDBID: 2}, {MediaType: "movie", TMDBID: 3}, {MediaType: "tv", TMDBID: 1}}

	found, err := kit.service.OnCallerWatchlist(auth.ContextWithUser(context.Background(), kit.alice), titles)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || !found[titles[0]] {
		t.Errorf("alice's watchlist = %v, want only movie 1", found)
	}
	if found, err := kit.service.OnCallerWatchlist(context.Background(), titles); err != nil || len(found) != 0 {
		t.Errorf("without an account = %v, %v; want nothing", found, err)
	}
}
