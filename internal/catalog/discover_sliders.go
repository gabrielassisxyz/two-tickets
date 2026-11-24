package catalog

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
)

// The Discover page's slider manifest and its two genre sliders. Seerr keeps
// sliders in a table admins edit; this app serves a fixed manifest instead
// and has no editor.

// DiscoverSliderType values the manifest uses, exactly upstream's
// server/constants/discover.ts enum (mirrored in web/src/compat). The kept
// renderer switches on these numbers.
const (
	discoverSliderTrending       = 4
	discoverSliderPopularMovies  = 5
	discoverSliderMovieGenres    = 6
	discoverSliderUpcomingMovies = 7
	discoverSliderStudios        = 8
	discoverSliderPopularTV      = 9
	discoverSliderTVGenres       = 10
	discoverSliderUpcomingTV     = 11
	discoverSliderNetworks       = 12
)

// This app's own slider types, from 100 so they never meet an upstream value
// (web/src/compat/sliderIds.ts holds the same numbers and fails the build on
// a collision). "Para você" (100) is the personal list the recommender
// serves; the library lists follow it.
const (
	discoverSliderForYou     = 100
	discoverSliderMyList     = 101
	discoverSliderCoupleList = 102
)

// discoverSlider is one entry of GET /settings/discover. Built-ins carry no
// title or data; the renderer names them itself.
type discoverSlider struct {
	ID        int     `json:"id"`
	Type      int     `json:"type"`
	Order     int     `json:"order"`
	IsBuiltIn bool    `json:"isBuiltIn"`
	Enabled   bool    `json:"enabled"`
	Title     *string `json:"title"`
	Data      *string `json:"data"`
}

// discoverSliderManifest is upstream's built-in slider list in upstream's
// order, without recently added, recent requests and the Plex watchlist,
// which have no counterpart here, preceded by this app's own "Para você",
// "Minha lista" and "Lista do casal". Each id is the type: the row id a
// fresh upstream install gives each built-in, and stable across releases.
func discoverSliderManifest() []discoverSlider {
	types := []int{
		discoverSliderForYou,
		discoverSliderMyList,
		discoverSliderCoupleList,
		discoverSliderTrending,
		discoverSliderPopularMovies,
		discoverSliderMovieGenres,
		discoverSliderUpcomingMovies,
		discoverSliderStudios,
		discoverSliderPopularTV,
		discoverSliderTVGenres,
		discoverSliderUpcomingTV,
		discoverSliderNetworks,
	}
	sliders := make([]discoverSlider, 0, len(types))
	for order, sliderType := range types {
		sliders = append(sliders, discoverSlider{ID: sliderType, Type: sliderType, Order: order, IsBuiltIn: true, Enabled: true})
	}
	return sliders
}

func (s *Service) handleDiscoverSliderManifest(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, discoverSliderManifest())
}

// discoverGenreSliderItem is upstream's GenreSliderItem.
type discoverGenreSliderItem struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Backdrops []string `json:"backdrops"`
}

// discoverGenreSliderMinBackdrops is how many artwork paths a genre needs to
// be shown: the kept genre card reads backdrops[4].
const discoverGenreSliderMinBackdrops = 5

// discoverGenreSliderFanOut bounds the discover calls one slider has in
// flight, so one Discover page cannot take every slot of the TMDB client.
const discoverGenreSliderFanOut = 4

// discoverGenreBackdrops returns the non-adult artwork of one genre's first
// discover page.
type discoverGenreBackdrops func(ctx context.Context, genreID int) ([]string, error)

func (s *Service) handleDiscoverMovieGenreSlider(w http.ResponseWriter, r *http.Request) {
	items, err := s.discoverGenreSlider(r.Context(), "/genre/movie/list", s.discoverMovieGenreBackdrops)
	if err != nil {
		discoverFailed(w, "Unable to retrieve movie genre slider.")
		return
	}
	respond.JSON(w, items)
}

func (s *Service) handleDiscoverTVGenreSlider(w http.ResponseWriter, r *http.Request) {
	items, err := s.discoverGenreSlider(r.Context(), "/genre/tv/list", s.discoverTVGenreBackdrops)
	if err != nil {
		discoverFailed(w, "Unable to retrieve series genre slider.")
		return
	}
	respond.JSON(w, items)
}

// discoverGenreSlider makes one discover call per genre, at most
// discoverGenreSliderFanOut at a time, and keeps the genres that have enough
// artwork to draw a card.
func (s *Service) discoverGenreSlider(ctx context.Context, listPath string, backdropsOf discoverGenreBackdrops) ([]discoverGenreSliderItem, error) {
	genres, err := discoverFetchGenres(ctx, s.tmdb, listPath)
	if err != nil {
		return nil, err
	}
	items := make([]discoverGenreSliderItem, len(genres))
	failures := make([]error, len(genres))
	slots := make(chan struct{}, discoverGenreSliderFanOut)
	var wg sync.WaitGroup
	for i, genre := range genres {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			backdrops, err := backdropsOf(ctx, genre.ID)
			items[i] = discoverGenreSliderItem{ID: genre.ID, Name: genre.Name, Backdrops: backdrops}
			failures[i] = err
		})
	}
	wg.Wait()
	for _, err := range failures {
		if err != nil {
			return nil, err
		}
	}
	return discoverKeepDrawableGenres(items), nil
}

// discoverKeepDrawableGenres drops every genre with fewer than
// discoverGenreSliderMinBackdrops artwork paths and sorts the rest by their
// localised name, as upstream's sortBy(mappedGenres, 'name') does. Upstream
// keeps a genre with too few and the card then draws a broken image.
func discoverKeepDrawableGenres(items []discoverGenreSliderItem) []discoverGenreSliderItem {
	kept := make([]discoverGenreSliderItem, 0, len(items))
	for _, item := range items {
		if len(item.Backdrops) >= discoverGenreSliderMinBackdrops {
			kept = append(kept, item)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })
	return kept
}

// discoverSliderParams is what upstream's genre sliders send: their TMDB
// client has no user, so no region and no original language, and the
// language is the server locale.
func discoverSliderParams(media discoverMedia, genreID int) map[string]string {
	params := map[string]string{
		"sort_by":     "popularity.desc",
		"page":        "1",
		"language":    discoverAnonymousLanguage,
		"region":      "",
		"with_genres": strconv.Itoa(genreID),
	}
	for key, value := range media.fixed {
		params[key] = value
	}
	return params
}

func (s *Service) discoverMovieGenreBackdrops(ctx context.Context, genreID int) ([]string, error) {
	data, err := s.tmdb.GetDiscoverMovies(ctx, discoverSliderParams(discoverMovieMedia, genreID))
	if err != nil {
		return nil, err
	}
	return discoverBackdropsOf(discoverMoviePage(data).Results), nil
}

func (s *Service) discoverTVGenreBackdrops(ctx context.Context, genreID int) ([]string, error) {
	data, err := s.tmdb.GetDiscoverTV(ctx, discoverSliderParams(discoverTVMedia, genreID))
	if err != nil {
		return nil, err
	}
	return discoverBackdropsOf(discoverTVResultsPage(data).Results), nil
}

// discoverBackdropsOf lists the artwork paths of results, skipping a missing
// or empty one.
func discoverBackdropsOf[T SearchResult](results []T) []string {
	backdrops := []string{}
	for _, result := range results {
		if path := discoverBackdropOf(result); path != "" {
			backdrops = append(backdrops, path)
		}
	}
	return backdrops
}
