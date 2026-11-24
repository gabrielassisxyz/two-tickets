package library

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

// walkLibraryPages follows nextCursor from the first page to the last and
// returns each page's results.
func walkLibraryPages[T any](t *testing.T, kit *libraryKit, path string) [][]T {
	t.Helper()
	var pages [][]T
	target := path
	for range 10 {
		page := decodeLibraryJSON[cursorPage[T]](t, kit.mustServe(t, kit.alice, http.MethodGet, target, "", http.StatusOK))
		pages = append(pages, page.Results)
		if page.NextCursor == nil {
			return pages
		}
		separator := "?"
		if u, _ := url.Parse(path); u.RawQuery != "" {
			separator = "&"
		}
		target = path + separator + "cursor=" + url.QueryEscape(*page.NextCursor)
	}
	t.Fatalf("%s never reached a last page", path)
	return nil
}

func pageSizes[T any](pages [][]T) []int {
	sizes := make([]int, 0, len(pages))
	for _, page := range pages {
		sizes = append(sizes, len(page))
	}
	return sizes
}

// 45 items, added three at a time so that every third shares added_at with
// two others, page as 20, 20 and 5 in the full ordering key, with no item
// repeated or skipped at a page boundary.
func TestListPagesFollowTheFullOrderingKey(t *testing.T) {
	kit := newLibraryKit(t)
	listID := decodeLibraryJSON[listBody](t, kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/library/lists", `{"name":"Big"}`, http.StatusCreated)).ID
	type key struct {
		addedAt   int64
		mediaType string
		tmdbID    int
	}
	var want []key
	for i := 1; i <= 45; i++ {
		mediaType := []string{"tv", "movie"}[i%2]
		kit.mustServe(t, kit.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", listID),
			fmt.Sprintf(`{"mediaType":%q,"tmdbId":%d}`, mediaType, 100-i), http.StatusOK)
		want = append(want, key{kit.clock.Now().Unix(), mediaType, 100 - i})
		if i%3 == 0 {
			kit.clock.Advance(time.Second)
		}
	}
	slices.SortFunc(want, func(a, b key) int {
		return cmp.Or(cmp.Compare(b.addedAt, a.addedAt), cmp.Compare(a.mediaType, b.mediaType), cmp.Compare(a.tmdbID, b.tmdbID))
	})

	callsBefore := kit.tmdb.calls.Load()
	pages := walkLibraryPages[testListItem](t, kit, fmt.Sprintf("/api/v1/library/lists/%d/items", listID))

	if sizes := pageSizes(pages); !slices.Equal(sizes, []int{20, 20, 5}) {
		t.Fatalf("page sizes = %v, want [20 20 5]", sizes)
	}
	var got []key
	for _, page := range pages {
		for _, item := range page {
			at, err := time.Parse(libraryTimestampLayout, item.AddedAt)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, key{at.Unix(), item.MediaType, item.TMDBID})
			if !item.Available {
				t.Errorf("item %s/%d was not hydrated", item.MediaType, item.TMDBID)
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("items in page order =\n%v\nwant\n%v", got, want)
	}
	if calls := kit.tmdb.calls.Load() - callsBefore; calls > 45 {
		t.Errorf("three pages made %d detail calls, want at most one per item", calls)
	}
}

// 45 history rows across movies, a whole show and its seasons, rated-only
// seasons and watched-and-rated movies, written five at a time under one
// clock tick so that rows share activity_at, page as 20, 20 and 5 in the full
// ordering key; ?level=loved returns exactly the loved rows.
func TestHistoryPagesFollowTheFullOrderingKeyAndFilterByLevel(t *testing.T) {
	kit := newLibraryKit(t)
	writes := 0
	write := func(path, body string) {
		t.Helper()
		kit.mustServe(t, kit.alice, http.MethodPut, path, body, http.StatusOK)
		if writes++; writes%5 == 0 {
			kit.clock.Advance(time.Second)
		}
	}
	for id := 1; id <= 20; id++ {
		write("/api/v1/library/watch", fmt.Sprintf(`{"mediaType":"movie","tmdbId":%d,"scope":"movie"}`, id))
	}
	write("/api/v1/library/watch", `{"mediaType":"tv","tmdbId":500,"scope":"show"}`)
	for season := 0; season <= 9; season++ {
		write("/api/v1/library/watch", fmt.Sprintf(`{"mediaType":"tv","tmdbId":500,"scope":"season","seasonNumber":%d}`, season))
	}
	wantLoved := 0
	for season := 1; season <= 7; season++ {
		level := "meh"
		if season%2 == 1 {
			level, wantLoved = "loved", wantLoved+1
		}
		write("/api/v1/library/rating", fmt.Sprintf(`{"mediaType":"tv","tmdbId":600,"scope":"season","seasonNumber":%d,"level":%q}`, season, level))
	}
	for id := 21; id <= 27; id++ {
		level := "good"
		if id <= 22 {
			level, wantLoved = "loved", wantLoved+1
		}
		write("/api/v1/library/watch", fmt.Sprintf(`{"mediaType":"movie","tmdbId":%d,"scope":"movie"}`, id))
		write("/api/v1/library/rating", fmt.Sprintf(`{"mediaType":"movie","tmdbId":%d,"scope":"movie","level":%q,"review":"r"}`, id, level))
	}

	pages := walkLibraryPages[testHistoryRow](t, kit, "/api/v1/library/history")

	if sizes := pageSizes(pages); !slices.Equal(sizes, []int{20, 20, 5}) {
		t.Fatalf("page sizes = %v, want [20 20 5]", sizes)
	}
	type key struct {
		activity  string
		mediaType string
		tmdbID    int
		scope     string
		season    int
	}
	var got []key
	seen := map[key]bool{}
	for _, page := range pages {
		for _, row := range page {
			season := -1
			if row.SeasonNumber != nil {
				season = *row.SeasonNumber
			}
			k := key{row.ActivityAt, row.MediaType, row.TMDBID, row.Scope, season}
			if seen[k] {
				t.Errorf("row %+v appears twice", k)
			}
			seen[k] = true
			got = append(got, k)
			if row.TMDBID >= 21 && row.TMDBID <= 27 && (!row.Watched || row.Rating == nil) {
				t.Errorf("movie %d shows %+v, want both its watch and its rating", row.TMDBID, row)
			}
		}
	}
	sorted := slices.IsSortedFunc(got, func(a, b key) int {
		return cmp.Or(cmp.Compare(b.activity, a.activity), cmp.Compare(a.mediaType, b.mediaType),
			cmp.Compare(a.tmdbID, b.tmdbID), cmp.Compare(a.scope, b.scope), cmp.Compare(a.season, b.season))
	})
	if !sorted {
		t.Errorf("history is not in its ordering key: %v", got)
	}

	loved := walkLibraryPages[testHistoryRow](t, kit, "/api/v1/library/history?level=loved")
	count := 0
	for _, page := range loved {
		for _, row := range page {
			count++
			if row.Rating == nil || row.Rating.Level == nil || *row.Rating.Level != "loved" {
				t.Errorf("?level=loved returned %+v", row)
			}
		}
	}
	if count != wantLoved {
		t.Errorf("?level=loved returned %d rows, want %d", count, wantLoved)
	}
	kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/library/history?level=great", "", http.StatusBadRequest)
	kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/library/history?cursor=bm9wZQ", "", http.StatusBadRequest)
}

// History is current state: removing a row's last state removes the row.
func TestHistoryDropsARowWhoseStateIsRemoved(t *testing.T) {
	kit := newLibraryKit(t)
	target := `"mediaType":"movie","tmdbId":3,"scope":"movie"`
	kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/watch", "{"+target+"}", http.StatusOK)
	kit.mustServe(t, kit.alice, http.MethodPut, "/api/v1/library/rating", "{"+target+`,"level":"fine"}`, http.StatusOK)
	kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/watch", "{"+target+"}", http.StatusOK)
	rows := walkLibraryPages[testHistoryRow](t, kit, "/api/v1/library/history")[0]
	if len(rows) != 1 || rows[0].Watched || rows[0].Rating == nil {
		t.Fatalf("after unmarking = %+v, want the rating alone", rows)
	}
	kit.mustServe(t, kit.alice, http.MethodDelete, "/api/v1/library/rating", "{"+target+"}", http.StatusOK)
	if rows := walkLibraryPages[testHistoryRow](t, kit, "/api/v1/library/history")[0]; len(rows) != 0 {
		t.Fatalf("after removing everything = %+v, want no rows", rows)
	}
	kit.mustServe(t, kit.bob, http.MethodPut, "/api/v1/library/watch", "{"+target+"}", http.StatusOK)
	if rows := walkLibraryPages[testHistoryRow](t, kit, "/api/v1/library/history")[0]; len(rows) != 0 {
		t.Fatalf("the caller's history shows the partner's rows: %+v", rows)
	}
}
