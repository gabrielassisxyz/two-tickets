package library

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The bead's seven steps and their 1-10 values, as literals.
func TestLevelValueMapsTheSevenSteps(t *testing.T) {
	want := map[string]int{"hated": 1, "disliked": 3, "meh": 5, "fine": 6, "good": 7, "liked": 8, "loved": 10}
	for level, value := range want {
		if got, ok := LevelValue(level); !ok || got != value {
			t.Errorf("LevelValue(%q) = %d, %v; want %d", level, got, ok, value)
		}
	}
	if len(ratingLevelValues) != len(want) {
		t.Errorf("the scale has %d levels, want %d", len(ratingLevelValues), len(want))
	}
	for _, unknown := range []string{"", "Loved", "ok", "great"} {
		if _, ok := LevelValue(unknown); ok {
			t.Errorf("LevelValue(%q) is known", unknown)
		}
	}
	if RatingScale != "sentiment-7-v1" {
		t.Errorf("RatingScale = %q", RatingScale)
	}
}

// libraryTargets covers both forms of every partial unique index: a movie and
// a whole show have no season number, a season has one.
var libraryTargets = map[string]string{
	"movie":    `"mediaType":"movie","tmdbId":603,"scope":"movie"`,
	"show":     `"mediaType":"tv","tmdbId":1399,"scope":"show"`,
	"season":   `"mediaType":"tv","tmdbId":1399,"scope":"season","seasonNumber":1`,
	"specials": `"mediaType":"tv","tmdbId":1399,"scope":"season","seasonNumber":0`,
}

// Marking twice and rating twice leave one row each and answer 200, on the
// path without a season number and on the path with one.
func TestWatchAndRatingAreIdempotentOnBothIndexPaths(t *testing.T) {
	kit := newLibraryKit(t)
	for _, target := range libraryTargets {
		for range 2 {
			kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/watch", "{"+target+"}", http.StatusOK)
			kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating", "{"+target+`,"level":"liked"}`, http.StatusOK)
			kit.clock.Advance(time.Minute)
		}
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM watches`); got != len(libraryTargets) {
		t.Errorf("watch rows = %d, want one per target (%d)", got, len(libraryTargets))
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM ratings`); got != len(libraryTargets) {
		t.Errorf("rating rows = %d, want one per target (%d)", got, len(libraryTargets))
	}
	// A repeated identical rating is a no-op: its timestamp stays the first.
	if got := kit.countRows(t, `SELECT COUNT(*) FROM ratings WHERE updated_at != created_at`); got != 0 {
		t.Errorf("%d ratings moved their timestamp on an identical repeat", got)
	}
	for _, target := range libraryTargets {
		for range 2 {
			kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/watch", "{"+target+"}", http.StatusOK)
			kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/rating", "{"+target+"}", http.StatusOK)
		}
	}
	if got := kit.countRows(t, `SELECT (SELECT COUNT(*) FROM watches) + (SELECT COUNT(*) FROM ratings)`); got != 0 {
		t.Errorf("rows left after removing everything twice = %d", got)
	}
}

func TestRatingPutMergesTheParts(t *testing.T) {
	kit := newLibraryKit(t)
	put := func(parts string) *rating {
		t.Helper()
		body := kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating", `{"mediaType":"movie","tmdbId":5,"scope":"movie",`+parts+`}`, http.StatusOK)
		return decodeLibraryJSON[titleStateBody](t, body).Me.Rating
	}
	got := put(`"level":"meh"`)
	if got == nil || *got.Level != "meh" || got.Review != nil {
		t.Fatalf("level only = %+v", got)
	}
	got = put(`"review":"slow start"`)
	if *got.Level != "meh" || *got.Review != "slow start" {
		t.Fatalf("review kept the level? %+v", got)
	}
	got = put(`"level":"loved"`)
	if *got.Level != "loved" || got.Review == nil || *got.Review != "slow start" {
		t.Fatalf("the picker cleared the review: %+v", got)
	}
	got = put(`"level":""`)
	if got.Level != nil || *got.Review != "slow start" {
		t.Fatalf("clearing the level = %+v", got)
	}
	if got := put(`"review":""`); got != nil {
		t.Fatalf("clearing the last part left %+v, want no rating", got)
	}
	if n := kit.countRows(t, `SELECT COUNT(*) FROM ratings`); n != 0 {
		t.Fatalf("rating rows = %d after clearing both parts", n)
	}
	var scale string
	put(`"level":"fine"`)
	if err := kit.db.Read.QueryRow(`SELECT scale FROM ratings`).Scan(&scale); err != nil || scale != "sentiment-7-v1" {
		t.Errorf("stored scale = %q (%v)", scale, err)
	}
}

func TestLibraryBodiesAreValidated(t *testing.T) {
	kit := newLibraryKit(t)
	for _, tc := range []struct{ path, body string }{
		{"/api/v1/library/rating", `{"mediaType":"movie","tmdbId":5,"scope":"movie"}`},
		{"/api/v1/library/rating", `{"mediaType":"movie","tmdbId":5,"scope":"movie","level":"great"}`},
		{"/api/v1/library/rating", `{"mediaType":"movie","tmdbId":5,"scope":"movie","review":"` + strings.Repeat("x", 10001) + `"}`},
		{"/api/v1/library/watch", `{"mediaType":"movie","tmdbId":5,"scope":"show"}`},
		{"/api/v1/library/watch", `{"mediaType":"tv","tmdbId":5,"scope":"season"}`},
		{"/api/v1/library/watch", `{"mediaType":"tv","tmdbId":5,"scope":"show","seasonNumber":1}`},
		{"/api/v1/library/watch", `{"mediaType":"tv","tmdbId":5,"scope":"season","seasonNumber":-1}`},
		{"/api/v1/library/watch", `{"mediaType":"tv","tmdbId":0,"scope":"show"}`},
		{"/api/v1/library/watch", `{"mediaType":"person","tmdbId":5,"scope":"movie"}`},
		{"/api/v1/library/watch", `{"mediaType":"movie","tmdbId":5,"scope":"movie"} {}`},
		{"/api/v1/library/watch", `not json`},
	} {
		if resp := kit.serve(t, kit.alice, http.MethodPut, tc.path, tc.body); resp.status != http.StatusBadRequest {
			t.Errorf("PUT %s %.80s = %d %s, want 400", tc.path, tc.body, resp.status, resp.body)
		}
	}
	if got := kit.countRows(t, `SELECT (SELECT COUNT(*) FROM watches) + (SELECT COUNT(*) FROM ratings)`); got != 0 {
		t.Errorf("a rejected body stored %d rows", got)
	}
}

// A body naming another user is rejected, never acted on, and the query
// string is never read for a user: the caller's rows change and the
// partner's never do.
func TestMutationsActOnlyOnTheCallersRows(t *testing.T) {
	kit := newLibraryKit(t)
	for _, path := range []string{"/api/v1/library/watch", "/api/v1/library/rating"} {
		body := `{"mediaType":"movie","tmdbId":603,"scope":"movie","level":"hated","userId":` + fmt.Sprint(kit.bob.ID) + `}`
		if resp := kit.serve(t, kit.alice, http.MethodPut, path, body); resp.status != http.StatusBadRequest {
			t.Errorf("PUT %s naming the partner = %d, want 400", path, resp.status)
		}
	}
	kit.mustServe(t, kit.bob, http.MethodPut, "/api/v1/library/watch", `{"mediaType":"movie","tmdbId":603,"scope":"movie"}`, http.StatusOK)
	query := fmt.Sprintf("?userId=%d&user=%d", kit.bob.ID, kit.bob.ID)
	kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/watch"+query, `{"mediaType":"movie","tmdbId":603,"scope":"movie"}`, http.StatusOK)
	kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating"+query, `{"mediaType":"movie","tmdbId":603,"scope":"movie","level":"hated"}`, http.StatusOK)
	if got := kit.countRows(t, `SELECT COUNT(*) FROM watches WHERE user_id = ?`, kit.bob.ID); got != 1 {
		t.Errorf("the partner's watch rows = %d after the caller's delete, want 1", got)
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM ratings WHERE user_id = ?`, kit.bob.ID); got != 0 {
		t.Errorf("the partner gained %d ratings from the caller's PUT", got)
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM ratings WHERE user_id = ?`, kit.alice.ID); got != 1 {
		t.Errorf("the caller's ratings = %d, want the one just written", got)
	}
}
