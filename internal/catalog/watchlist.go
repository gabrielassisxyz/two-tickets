package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// The watchlist state the kept frontend reads from catalog answers: title
// pages read onUserWatchlist, cards read mediaInfo.watchlists. The library
// owns the watchlist; the catalog only asks it, through WatchlistMembership,
// and imports neither the library nor auth.
//
// mediaInfo goes on result items only, never on a detail answer: the kept
// detail pages treat a present mediaInfo as "a media row exists" and then
// read fields this app never has. So a result carries
// "mediaInfo": {"watchlists": [{"tmdbId": ..., "mediaType": ...}]} when it is
// on the caller's watchlist, and no mediaInfo at all otherwise; the cards
// only read the array's length.

// WatchlistTitle names one title for a membership question.
type WatchlistTitle struct {
	MediaType string
	TMDBID    int
}

// WatchlistMembership answers which of titles are on the watchlist of the
// account ctx carries. The implementation resolves that account itself.
type WatchlistMembership interface {
	OnCallerWatchlist(ctx context.Context, titles []WatchlistTitle) (map[WatchlistTitle]bool, error)
}

// UseWatchlist sets where watchlist state comes from. Without it, or with
// nil, nothing is on a watchlist, which is what every golden records.
func (s *Service) UseWatchlist(membership WatchlistMembership) {
	s.watchlist = membership
}

// watchlistHas answers for one title; a failed lookup reads as "not on the
// watchlist", so a library fault never fails a title page.
func (s *Service) watchlistHas(ctx context.Context, mediaType string, id int) bool {
	if s.watchlist == nil {
		return false
	}
	title := WatchlistTitle{MediaType: mediaType, TMDBID: id}
	found, err := s.watchlist.OnCallerWatchlist(ctx, []WatchlistTitle{title})
	return err == nil && found[title]
}

// watchlistRouteMux is the part of *http.ServeMux the route families use.
// Register hands them watchlistMarkingMux instead, so every family's pages
// pass through watchlistMarkResults without any handler knowing.
type watchlistRouteMux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

type watchlistMarkingMux struct {
	mux     *http.ServeMux
	service *Service
}

func (m watchlistMarkingMux) Handle(pattern string, handler http.Handler) {
	m.mux.Handle(pattern, m.service.watchlistMarkResults(handler))
}

func (m watchlistMarkingMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(handler))
}

// watchlistMarkResults is the one place result pages gain mediaInfo. It
// buffers a successful JSON answer, collects the movie and series items of
// its top-level results or parts array, asks the library about all of them in
// one query, and splices mediaInfo into the items that are on the watchlist.
// Every other byte of the answer is left as the handler wrote it.
func (s *Service) watchlistMarkResults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.watchlist == nil || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		buffered := &watchlistBufferedWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(buffered, r)
		body := buffered.body.Bytes()
		if buffered.status == http.StatusOK && strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			body = s.watchlistMarkBody(r.Context(), body)
		}
		w.WriteHeader(buffered.status)
		_, _ = w.Write(body)
	})
}

type watchlistBufferedWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (b *watchlistBufferedWriter) WriteHeader(status int) {
	if !b.wroteHeader {
		b.status, b.wroteHeader = status, true
	}
}

func (b *watchlistBufferedWriter) Write(p []byte) (int, error) {
	b.wroteHeader = true
	return b.body.Write(p)
}

// watchlistItem is a result item's position in the body: end is the offset
// just past its closing brace.
type watchlistItem struct {
	title WatchlistTitle
	end   int
}

func (s *Service) watchlistMarkBody(ctx context.Context, body []byte) []byte {
	items := watchlistResultItems(body)
	if len(items) == 0 {
		return body
	}
	titles := make([]WatchlistTitle, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.title)
	}
	found, err := s.watchlist.OnCallerWatchlist(ctx, titles)
	if err != nil || len(found) == 0 {
		return body
	}
	var out bytes.Buffer
	out.Grow(len(body) + 64*len(found))
	from := 0
	for _, item := range items {
		if !found[item.title] {
			continue
		}
		closing := item.end - 1
		out.Write(body[from:closing])
		out.WriteString(`,"mediaInfo":{"watchlists":[{"tmdbId":` + strconv.Itoa(item.title.TMDBID) +
			`,"mediaType":"` + item.title.MediaType + `"}]}`)
		from = closing
	}
	out.Write(body[from:])
	return out.Bytes()
}

// watchlistResultItems finds the movie and series objects directly inside the
// answer's top-level results or parts array, in order. Anything it cannot
// read is skipped, which leaves that part of the answer unmarked.
func watchlistResultItems(body []byte) []watchlistItem {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	var items []watchlistItem
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return items
		}
		if key == "results" || key == "parts" {
			if items, err = watchlistArrayItems(decoder, items); err != nil {
				return items
			}
			continue
		}
		var skipped json.RawMessage
		if err := decoder.Decode(&skipped); err != nil {
			return items
		}
	}
	return items
}

func watchlistArrayItems(decoder *json.Decoder, items []watchlistItem) ([]watchlistItem, error) {
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return items, err
	}
	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return items, err
		}
		end := int(decoder.InputOffset())
		var head struct {
			ID        int    `json:"id"`
			MediaType string `json:"mediaType"`
		}
		if len(raw) < 2 || raw[0] != '{' || json.Unmarshal(raw, &head) != nil {
			continue
		}
		if (head.MediaType == "movie" || head.MediaType == "tv") && head.ID > 0 {
			items = append(items, watchlistItem{title: WatchlistTitle{MediaType: head.MediaType, TMDBID: head.ID}, end: end})
		}
	}
	_, err := decoder.Token()
	return items, err
}
