package library

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
)

func TestSeriesStatusTable(t *testing.T) {
	for _, tc := range []struct {
		name        string
		show        bool
		seasons     []int
		wantStatus  string
		wantRegular int
	}{
		{"nothing", false, nil, seriesStatusUnwatched, 0},
		{"show only", true, nil, seriesStatusWatched, 0},
		{"show and seasons", true, []int{1, 2}, seriesStatusWatched, 2},
		{"seasons only", false, []int{1, 2}, seriesStatusInProgress, 2},
		{"specials only", false, []int{0}, seriesStatusInProgress, 0},
		{"specials and a season", false, []int{0, 3}, seriesStatusInProgress, 1},
		{"show and specials", true, []int{0}, seriesStatusWatched, 0},
	} {
		status, regular := seriesStatus(tc.show, tc.seasons)
		if status != tc.wantStatus || regular != tc.wantRegular {
			t.Errorf("%s: seriesStatus = (%s, %d), want (%s, %d)", tc.name, status, regular, tc.wantStatus, tc.wantRegular)
		}
	}
}

// seriesStep is one transition and the state it must leave.
type seriesStep struct {
	name        string
	method      string // PUT marks, DELETE unmarks
	season      int    // -1 is the whole show
	wantStatus  string
	wantShow    bool
	wantRegular int
	wantSeasons []int
}

// The series transition table, through the routes against SQLite: the show
// row and the season rows are independent, unmarking the show reveals the
// seasons that remain, marking every regular season never creates a show
// row, and specials never count towards completion.
func TestSeriesTransitionTable(t *testing.T) {
	kit := newLibraryKit(t)
	const show = 1399
	steps := []seriesStep{
		{"mark show", http.MethodPut, -1, seriesStatusWatched, true, 0, nil},
		{"mark season 1 under the show", http.MethodPut, 1, seriesStatusWatched, true, 1, []int{1}},
		{"unmark show with a season remaining", http.MethodDelete, -1, seriesStatusInProgress, false, 1, []int{1}},
		{"mark season 2", http.MethodPut, 2, seriesStatusInProgress, false, 2, []int{1, 2}},
		{"mark season 3, the last regular one", http.MethodPut, 3, seriesStatusInProgress, false, 3, []int{1, 2, 3}},
		{"mark specials", http.MethodPut, 0, seriesStatusInProgress, false, 3, []int{0, 1, 2, 3}},
		{"unmark season 1", http.MethodDelete, 1, seriesStatusInProgress, false, 2, []int{0, 2, 3}},
		{"unmark season 2", http.MethodDelete, 2, seriesStatusInProgress, false, 1, []int{0, 3}},
		{"unmark season 3, leaving specials", http.MethodDelete, 3, seriesStatusInProgress, false, 0, []int{0}},
		{"unmark specials", http.MethodDelete, 0, seriesStatusUnwatched, false, 0, nil},
		{"mark season 4", http.MethodPut, 4, seriesStatusInProgress, false, 1, []int{4}},
		{"unmark season 4", http.MethodDelete, 4, seriesStatusUnwatched, false, 0, nil},
		{"mark show again", http.MethodPut, -1, seriesStatusWatched, true, 0, nil},
		{"unmark show with nothing remaining", http.MethodDelete, -1, seriesStatusUnwatched, false, 0, nil},
	}
	for _, step := range steps {
		body := fmt.Sprintf(`{"mediaType":"tv","tmdbId":%d,"scope":"show"}`, show)
		if step.season >= 0 {
			body = fmt.Sprintf(`{"mediaType":"tv","tmdbId":%d,"scope":"season","seasonNumber":%d}`, show, step.season)
		}
		answered := decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, step.method, "/api/v1/library/watch", body, http.StatusOK))
		read := decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodGet, fmt.Sprintf("/api/v1/library/title/tv/%d", show), "", http.StatusOK))
		for source, state := range map[string]personTitleState{"mutation answer": answered.Me, "title read": read.Me} {
			var watchedSeasons []int
			for _, season := range state.Seasons {
				if season.Watched {
					watchedSeasons = append(watchedSeasons, season.SeasonNumber)
				}
			}
			if state.Status != step.wantStatus || state.Watched != step.wantShow || state.RegularSeasonsWatched != step.wantRegular || !slices.Equal(watchedSeasons, step.wantSeasons) {
				t.Fatalf("after %q, %s = status %s, show %v, regular %d, seasons %v; want %s, %v, %d, %v",
					step.name, source, state.Status, state.Watched, state.RegularSeasonsWatched, watchedSeasons,
					step.wantStatus, step.wantShow, step.wantRegular, step.wantSeasons)
			}
		}
		if got := kit.countRows(t, `SELECT COUNT(*) FROM watches WHERE scope = 'show'`); got != map[bool]int{true: 1, false: 0}[step.wantShow] {
			t.Fatalf("after %q, show rows = %d, want the show row only while the show is marked", step.name, got)
		}
	}
}

// The partner's state is visible read-only next to the caller's, and the
// caller's list memberships name only the caller's and the couple's lists.
func TestTitleStateShowsBothPeople(t *testing.T) {
	kit := newLibraryKit(t)
	kit.mustServe(t, kit.bob, http.MethodPut, "/api/v1/library/watch", `{"mediaType":"movie","tmdbId":603,"scope":"movie"}`, http.StatusOK)
	kit.mustServe(t, kit.bob, http.MethodPut, "/api/v1/library/rating", `{"mediaType":"movie","tmdbId":603,"scope":"movie","level":"loved","review":"great"}`, http.StatusOK)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":603,"mediaType":"movie","title":"x"}`, http.StatusCreated)
	coupleID := kit.listIDOf(t, listKindCouple, 0)
	kit.mustServe(t, kit.bob, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", coupleID), `{"mediaType":"movie","tmdbId":603}`, http.StatusOK)

	state := decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/library/title/movie/603", "", http.StatusOK))

	if state.Me.Status != seriesStatusUnwatched || state.Me.Rating != nil || state.Me.DisplayName != "Alice" {
		t.Errorf("caller state = %+v, want Alice with nothing", state.Me)
	}
	if state.Partner.Status != seriesStatusWatched || state.Partner.Rating == nil || *state.Partner.Rating.Level != "loved" || state.Partner.DisplayName != "Bob" {
		t.Errorf("partner state = %+v, want Bob's watch and loved rating", state.Partner)
	}
	if len(state.Lists) != 1 || state.Lists[0].ID != coupleID {
		t.Errorf("caller memberships = %+v, want only the couple list (Bob's watchlist is his)", state.Lists)
	}
}

// A rating without a recorded watch is valid; so is a watch without a rating.
func TestWatchAndRatingAreIndependent(t *testing.T) {
	kit := newLibraryKit(t)
	state := decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating",
		`{"mediaType":"tv","tmdbId":1,"scope":"season","seasonNumber":2,"level":"good"}`, http.StatusOK))
	if state.Me.Status != seriesStatusUnwatched || len(state.Me.Seasons) != 1 || state.Me.Seasons[0].Watched || state.Me.Seasons[0].Rating == nil {
		t.Errorf("rating without a watch = %+v", state.Me)
	}
	if state.Me.Rating != nil {
		t.Errorf("a season rating produced a show rating: %+v", state.Me.Rating)
	}
	state = decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/watch",
		`{"mediaType":"movie","tmdbId":2,"scope":"movie"}`, http.StatusOK))
	if !state.Me.Watched || state.Me.Rating != nil {
		t.Errorf("watch without a rating = %+v", state.Me)
	}
}

// Clearing one rating clears only that row: the show's rating and each
// season's are independent, so clearing a season keeps the show and the
// other seasons, and clearing the show keeps every season.
func TestClearingARatingKeepsTheOtherScopes(t *testing.T) {
	kit := newLibraryKit(t)
	for _, body := range []string{
		`{"mediaType":"tv","tmdbId":1,"scope":"show","level":"loved"}`,
		`{"mediaType":"tv","tmdbId":1,"scope":"season","seasonNumber":1,"level":"good"}`,
		`{"mediaType":"tv","tmdbId":1,"scope":"season","seasonNumber":2,"level":"meh"}`,
	} {
		kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating", body, http.StatusOK)
	}
	seasonLevels := func(state titleStateBody) map[int]string {
		levels := map[int]string{}
		for _, season := range state.Me.Seasons {
			if season.Rating != nil && season.Rating.Level != nil {
				levels[season.SeasonNumber] = *season.Rating.Level
			}
		}
		return levels
	}

	state := decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/rating",
		`{"mediaType":"tv","tmdbId":1,"scope":"season","seasonNumber":1}`, http.StatusOK))
	if state.Me.Rating == nil || state.Me.Rating.Level == nil || *state.Me.Rating.Level != "loved" {
		t.Errorf("clearing season 1 changed the show's rating: %+v", state.Me.Rating)
	}
	if got := seasonLevels(state); len(got) != 1 || got[2] != "meh" {
		t.Errorf("after clearing season 1, season ratings = %v, want only season 2 = meh", got)
	}

	state = decodeLibraryJSON[titleStateBody](t, kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/rating",
		`{"mediaType":"tv","tmdbId":1,"scope":"show"}`, http.StatusOK))
	if state.Me.Rating != nil {
		t.Errorf("the show's rating survived its own clearing: %+v", state.Me.Rating)
	}
	if got := seasonLevels(state); len(got) != 1 || got[2] != "meh" {
		t.Errorf("clearing the show changed the season ratings: %v, want season 2 = meh", got)
	}
}
