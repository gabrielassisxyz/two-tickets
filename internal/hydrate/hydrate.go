// Package hydrate turns stored title ids into the cards a page renders.
//
// Lists, history, "For you" and /discover/watchlist hold (mediaType, tmdbId)
// pairs, not titles. Cards resolves each pair to its TMDB details through the
// process-wide TMDB client, whose cache, singleflight and limiter it relies on
// rather than adding its own, with at most four detail calls in flight and a
// deadline over the whole page. A title TMDB cannot answer in time, answers
// with an error, or flags as adult becomes a placeholder card, so one
// unreachable title degrades one card and never the page.
//
// # The hydrated item shape
//
// Every consumer (the library routes, the library UI, "For you") embeds Card,
// which serialises as:
//
//	{
//	  "mediaType": "movie" | "tv",
//	  "tmdbId":    603,
//	  "available": true,
//	  "result":    { ...the shared search result shape... }
//	}
//
// result is exactly what the catalog's search and discover routes answer for
// the same title: a details answer mapped by catalog.MapMovieDetailsToResult or
// catalog.MapTvDetailsToResult and then catalog.MapSearchResults, so a card
// renders from it with no second request. When available is false, result is
// a placeholder with the same keys: the id and mediaType are the stored ones,
// the title (movies) or name (series) is PlaceholderTitle, artwork paths are
// null, numbers are zero and arrays are empty. The page that holds it still
// answers 200.
//
// Library fields sit next to these keys, at the same level, in the type that
// embeds Card. Cursor pages that hold cards are {results, nextCursor}, where
// nextCursor is an opaque string, or null on the last page.
package hydrate

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

const (
	// PageLimit is the most items one page holds, and so the most distinct
	// titles one call to Cards fetches.
	PageLimit = 20
	// hydrateMaxInFlight is the bound the genre slider uses: one page never
	// takes more than four of the TMDB client's slots.
	hydrateMaxInFlight = 4
	// hydratePageDeadline bounds a whole page. The TMDB client's own budget
	// is 30 s per call, far longer than a page should wait for one card.
	hydratePageDeadline = 8 * time.Second
)

// Title names one stored title. TMDB's movie and series ids are separate
// sequences, so an id never travels without its media type.
type Title struct {
	MediaType string
	TMDBID    int
}

// Card is one hydrated title; see the package documentation for its shape.
type Card struct {
	MediaType string               `json:"mediaType"`
	TMDBID    int                  `json:"tmdbId"`
	Available bool                 `json:"available"`
	Result    catalog.SearchResult `json:"result"`
}

// DisplayTitle is the card's title for a movie and its name for a series.
func (c Card) DisplayTitle() string {
	switch result := c.Result.(type) {
	case catalog.MovieResult:
		return result.Title
	case catalog.TvResult:
		return result.Name
	}
	return PlaceholderTitle(Title{MediaType: c.MediaType, TMDBID: c.TMDBID})
}

// PlaceholderTitle is what a card shows for a title TMDB could not answer. It
// is language neutral on purpose: the UI reads available and renders its own
// translated label, and this string only keeps a title field non-empty.
func PlaceholderTitle(title Title) string {
	return title.MediaType + " " + strconv.Itoa(title.TMDBID)
}

// Hydrator resolves titles through one TMDB client.
type Hydrator struct {
	client   *tmdb.Client
	deadline time.Duration
}

// New returns a hydrator over the process-wide client.
func New(client *tmdb.Client) *Hydrator {
	return &Hydrator{client: client, deadline: hydratePageDeadline}
}

// Cards returns one card per title, in the order given. A title repeated in
// titles is fetched once. Only the first PageLimit distinct titles are
// fetched; any beyond them are placeholders, so no caller can make one page
// cost more than PageLimit detail calls. Cards never fails: every failure is
// a placeholder.
func (h *Hydrator) Cards(ctx context.Context, titles []Title) []Card {
	distinct := hydrateDistinct(titles)
	fetched := h.fetchAll(ctx, distinct)
	cards := make([]Card, len(titles))
	for i, title := range titles {
		card, ok := fetched[title]
		if !ok {
			card = placeholderCard(title)
		}
		cards[i] = card
	}
	return cards
}

func hydrateDistinct(titles []Title) []Title {
	seen := make(map[Title]bool, len(titles))
	distinct := make([]Title, 0, min(len(titles), PageLimit))
	for _, title := range titles {
		if seen[title] || len(distinct) == PageLimit {
			continue
		}
		seen[title] = true
		distinct = append(distinct, title)
	}
	return distinct
}

// fetchAll runs at most hydrateMaxInFlight fetches at a time under the page
// deadline. A fetch still waiting for a slot when the deadline passes is
// never started.
func (h *Hydrator) fetchAll(ctx context.Context, titles []Title) map[Title]Card {
	ctx, cancel := context.WithTimeout(ctx, h.deadline)
	defer cancel()
	cards := make([]Card, len(titles))
	slots := make(chan struct{}, hydrateMaxInFlight)
	var wg sync.WaitGroup
	for i, title := range titles {
		wg.Go(func() {
			cards[i] = placeholderCard(title)
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			if card, ok := h.fetchOne(ctx, title); ok {
				cards[i] = card
			}
		})
	}
	wg.Wait()
	byTitle := make(map[Title]Card, len(titles))
	for i, title := range titles {
		byTitle[title] = cards[i]
	}
	return byTitle
}

func (h *Hydrator) fetchOne(ctx context.Context, title Title) (Card, bool) {
	if ctx.Err() != nil || title.TMDBID <= 0 {
		return Card{}, false
	}
	id := strconv.Itoa(title.TMDBID)
	params := map[string]string{"language": tmdb.DefaultLanguage}
	var item tmdb.MediaItem
	switch title.MediaType {
	case "movie":
		details, err := h.client.GetMovie(ctx, id, params)
		if err != nil {
			return Card{}, false
		}
		item = catalog.MapMovieDetailsToResult(*details)
	case "tv":
		details, err := h.client.GetTVShow(ctx, id, params)
		if err != nil {
			return Card{}, false
		}
		item = catalog.MapTvDetailsToResult(*details)
	default:
		return Card{}, false
	}
	// The adult policy holds here too: a title the catalog would never show
	// is not revealed through a list either.
	results := catalog.WithoutAdult(catalog.MapSearchResults([]tmdb.MediaItem{item}))
	if len(results) != 1 {
		return Card{}, false
	}
	return Card{MediaType: title.MediaType, TMDBID: title.TMDBID, Available: true, Result: results[0]}, true
}

func placeholderCard(title Title) Card {
	name := PlaceholderTitle(title)
	var result catalog.SearchResult
	switch title.MediaType {
	case "tv":
		result = catalog.MapTvResult(tmdb.TVBrief{ID: title.TMDBID, MediaType: "tv", Name: name, OriginalName: name})
	default:
		result = catalog.MapMovieResult(tmdb.MovieBrief{ID: title.TMDBID, Title: name, OriginalTitle: name})
	}
	return Card{MediaType: title.MediaType, TMDBID: title.TMDBID, Available: false, Result: result}
}

// String is for test failures and logs.
func (t Title) String() string {
	return fmt.Sprintf("%s/%d", t.MediaType, t.TMDBID)
}
