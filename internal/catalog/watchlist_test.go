package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
)

// fakeWatchlist answers from a fixed set and records every question.
type fakeWatchlist struct {
	on   map[WatchlistTitle]bool
	fail bool

	mu    sync.Mutex
	asked [][]WatchlistTitle
}

func (f *fakeWatchlist) OnCallerWatchlist(_ context.Context, titles []WatchlistTitle) (map[WatchlistTitle]bool, error) {
	f.mu.Lock()
	f.asked = append(f.asked, append([]WatchlistTitle(nil), titles...))
	f.mu.Unlock()
	if f.fail {
		return nil, errors.New("library unavailable")
	}
	found := map[WatchlistTitle]bool{}
	for _, title := range titles {
		if f.on[title] {
			found[title] = true
		}
	}
	return found, nil
}

// watchlistServe answers the golden's request through a service whose
// watchlist is membership (nil for none).
func watchlistServe(t *testing.T, family, name string, membership WatchlistMembership) []byte {
	t.Helper()
	golden := catalogtest.LoadGolden(t, family, name)
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	service := NewService(titleTestClient(server.URL, 5*time.Second), func() time.Time { return discoverFrozenNow })
	if membership != nil {
		service.UseWatchlist(membership)
	}
	mux := http.NewServeMux()
	service.Register(mux)
	recorder := httptest.NewRecorder()
	method, target, _ := bytes.Cut([]byte(golden.Request), []byte(" "))
	mux.ServeHTTP(recorder, httptest.NewRequest(string(method), string(target), nil))
	if recorder.Code != golden.Status {
		t.Fatalf("%s = %d, want %d: %s", golden.Request, recorder.Code, golden.Status, recorder.Body)
	}
	return recorder.Body.Bytes()
}

func watchlistSplice(title WatchlistTitle) string {
	return `,"mediaInfo":{"watchlists":[{"tmdbId":` + strconv.Itoa(title.TMDBID) + `,"mediaType":"` + title.MediaType + `"}]}`
}

type watchlistResultHead struct {
	ID        int             `json:"id"`
	MediaType string          `json:"mediaType"`
	MediaInfo json.RawMessage `json:"mediaInfo"`
}

func watchlistHeads(t *testing.T, body []byte, list string) []watchlistResultHead {
	t.Helper()
	var page map[string]json.RawMessage
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	var heads []watchlistResultHead
	if err := json.Unmarshal(page[list], &heads); err != nil {
		t.Fatalf("decode %s: %v", list, err)
	}
	return heads
}

// A result on the caller's watchlist gains mediaInfo.watchlists, every other
// result has no mediaInfo, the whole page costs one membership question, and
// removing the one splice gives back the unmarked answer byte for byte.
func TestWatchlistMarksOnlyTheResultsOnTheWatchlist(t *testing.T) {
	for _, tc := range []struct{ family, name, list string }{
		{"title", "movie-recommendations", "results"},
		{"title", "tv-similar", "results"},
		{"title", "collection-full", "parts"},
		{"discover", "discover-movies", "results"},
		{"discover", "discover-trending-four-kinds", "results"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unmarked := watchlistServe(t, tc.family, tc.name, nil)
			heads := watchlistHeads(t, unmarked, tc.list)
			var target WatchlistTitle
			var candidates int
			for _, head := range heads {
				if head.MediaType != "movie" && head.MediaType != "tv" {
					continue
				}
				candidates++
				if target.TMDBID == 0 && candidates == 2 {
					target = WatchlistTitle{MediaType: head.MediaType, TMDBID: head.ID}
				}
			}
			if target.TMDBID == 0 {
				t.Fatalf("golden %s has fewer than two titles in %s", tc.name, tc.list)
			}
			membership := &fakeWatchlist{on: map[WatchlistTitle]bool{target: true}}

			marked := watchlistServe(t, tc.family, tc.name, membership)

			if len(membership.asked) != 1 {
				t.Fatalf("membership questions = %d, want one per page", len(membership.asked))
			}
			if got := len(membership.asked[0]); got != candidates {
				t.Errorf("titles asked about = %d, want every movie and series in the page (%d)", got, candidates)
			}
			for _, head := range watchlistHeads(t, marked, tc.list) {
				onList := head.ID == target.TMDBID && head.MediaType == target.MediaType
				if onList != (head.MediaInfo != nil) {
					t.Errorf("%s %d: mediaInfo = %s, want present only on the watchlisted title", head.MediaType, head.ID, head.MediaInfo)
				}
			}
			if restored := bytes.Replace(marked, []byte(watchlistSplice(target)), nil, 1); !bytes.Equal(restored, unmarked) {
				t.Errorf("the marked answer differs from the unmarked one by more than the splice")
			}
		})
	}
}

// With nothing on the watchlist, or a library that fails, a page is the
// unmarked answer exactly.
func TestWatchlistLeavesAnUnmarkedPageByteIdentical(t *testing.T) {
	unmarked := watchlistServe(t, "discover", "discover-movies", nil)
	for name, membership := range map[string]*fakeWatchlist{
		"empty watchlist": {on: map[WatchlistTitle]bool{}},
		"failing library": {fail: true},
	} {
		if got := watchlistServe(t, "discover", "discover-movies", membership); !bytes.Equal(got, unmarked) {
			t.Errorf("%s: the page changed", name)
		}
	}
}

// A detail answer carries onUserWatchlist and never mediaInfo, even when
// every title it mentions is on the watchlist.
func TestWatchlistDetailAnswersCarryOnlyOnUserWatchlist(t *testing.T) {
	for _, tc := range []struct{ name, mediaType string }{
		{"movie-details-full", "movie"},
		{"tv-details-full", "tv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unmarked := watchlistServe(t, "title", tc.name, nil)
			var details struct {
				ID              int  `json:"id"`
				OnUserWatchlist bool `json:"onUserWatchlist"`
			}
			if err := json.Unmarshal(unmarked, &details); err != nil || details.OnUserWatchlist {
				t.Fatalf("unmarked details = %+v (%v), want onUserWatchlist false", details, err)
			}
			title := WatchlistTitle{MediaType: tc.mediaType, TMDBID: details.ID}
			membership := &fakeWatchlist{on: map[WatchlistTitle]bool{title: true}}

			marked := watchlistServe(t, "title", tc.name, membership)

			if err := json.Unmarshal(marked, &details); err != nil || !details.OnUserWatchlist {
				t.Errorf("details on the watchlist = %+v (%v), want onUserWatchlist true", details, err)
			}
			if bytes.Contains(marked, []byte(`"mediaInfo"`)) {
				t.Errorf("a detail answer carries mediaInfo")
			}
			want := bytes.Replace(unmarked, []byte(`"onUserWatchlist":false`), []byte(`"onUserWatchlist":true`), 1)
			if !bytes.Equal(marked, want) {
				t.Errorf("the detail answer changed beyond onUserWatchlist")
			}
		})
	}
}
