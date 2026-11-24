package catalog

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// Search routes answer the search box: /search with its tmdb:, imdb:, tvdb:
// and year: prefixes, plus /search/keyword and /search/company. Shapes match
// Seerr's own responses and are pinned by goldens under testdata/golden/search.
// Keyword and company answers stay raw snake_case, as their consumers expect.

func (s *Service) registerSearch(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/search", http.HandlerFunc(s.handleSearch))
	mux.Handle("GET /api/v1/search/keyword", http.HandlerFunc(s.handleSearchKeyword))
	mux.Handle("GET /api/v1/search/company", http.HandlerFunc(s.handleSearchCompany))
}

// searchYearPattern finds the first year:NNNN in a query, case-insensitively.
// Go's RE2 has no lookbehind, so the prefixes are extracted by hand; this one
// only locates the year field for stripping.
var searchYearPattern = regexp.MustCompile(`(?i)year:\d{4}`)

// searchParseQuery returns the raw query param, or false when it is missing or
// empty. Empty is a 400: upstream would search literally, which never helps.
func searchParseQuery(r *http.Request) (string, bool) {
	query := r.URL.Query().Get("query")
	if strings.TrimSpace(query) == "" {
		return "", false
	}
	return query, true
}

// searchParsePage returns the page param, defaulting to 1 when absent. A
// present page must be a positive integer, anything else is a 400.
func searchParsePage(r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page <= 0 {
		return 0, false
	}
	return page, true
}

// searchExtractTmdbID returns the digits after the first tmdb:, or false when
// there are none. The query must already be lowercased.
func searchExtractTmdbID(lowered string) (string, bool) {
	index := strings.Index(lowered, "tmdb:")
	if index < 0 {
		return "", false
	}
	rest := lowered[index+len("tmdb:"):]
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return "", false
	}
	return rest[:digits], true
}

// searchExtractImdbID returns the ttNNNN or nmNNNN after the first imdb:, or
// false. The query must already be lowercased.
func searchExtractImdbID(lowered string) (string, bool) {
	index := strings.Index(lowered, "imdb:")
	if index < 0 {
		return "", false
	}
	rest := lowered[index+len("imdb:"):]
	if len(rest) < 3 {
		return "", false
	}
	if rest[0] != 't' && rest[0] != 'n' || rest[1] != 't' && rest[1] != 'm' {
		return "", false
	}
	if (rest[0] == 't' && rest[1] != 't') || (rest[0] == 'n' && rest[1] != 'm') {
		return "", false
	}
	digits := 2
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 2 {
		return "", false
	}
	return rest[:digits], true
}

// searchExtractTvdbID returns the digits after the first tvdb:, or false. The
// query must already be lowercased.
func searchExtractTvdbID(lowered string) (string, bool) {
	index := strings.Index(lowered, "tvdb:")
	if index < 0 {
		return "", false
	}
	rest := lowered[index+len("tvdb:"):]
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return "", false
	}
	return rest[:digits], true
}

// searchExtractYear returns the first four digits after the first year:, or
// false. The query must already be lowercased.
func searchExtractYear(lowered string) (string, bool) {
	index := strings.Index(lowered, "year:")
	if index < 0 {
		return "", false
	}
	rest := lowered[index+len("year:"):]
	if len(rest) < 4 {
		return "", false
	}
	for i := 0; i < 4; i++ {
		if rest[i] < '0' || rest[i] > '9' {
			return "", false
		}
	}
	return rest[:4], true
}

// searchStripYear removes the first year:NNNN from the original query, keeping
// the rest verbatim. Upstream removes only the first occurrence, so this does
// the same; the trailing space in "friends year:1994" survives as "friends ".
func searchStripYear(query string) string {
	loc := searchYearPattern.FindStringIndex(query)
	if loc == nil {
		return query
	}
	return query[:loc[0]] + query[loc[1]:]
}

// searchMovieDetails is a movie details answer plus its videos, so the tmdb:
// probe can decide the English trailer fallback upstream always performs. The
// shared tmdb.MovieDetails drops videos, which this route never maps.
type searchMovieDetails struct {
	tmdb.MovieDetails
	Videos struct {
		Results []tmdb.Video `json:"results"`
	} `json:"videos"`
}

// searchTvDetails is the series counterpart of searchMovieDetails.
type searchTvDetails struct {
	tmdb.TVDetails
	Videos struct {
		Results []tmdb.Video `json:"results"`
	} `json:"videos"`
}

func searchHasTrailer(videos []tmdb.Video) bool {
	for _, video := range videos {
		if video.Type == "Trailer" {
			return true
		}
	}
	return false
}

const (
	searchMovieAppend = "credits,external_ids,videos,keywords,release_dates,watch/providers"
	searchTvAppend    = "aggregate_credits,credits,external_ids,keywords,videos,content_ratings,watch/providers"
)

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request) {
	query, ok := searchParseQuery(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "query must be non-empty")
		return
	}
	page, ok := searchParsePage(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "page must be a positive integer")
		return
	}
	lowered := strings.ToLower(query)

	if id, ok := searchExtractTmdbID(lowered); ok {
		s.handleSearchTmdb(w, r, id)
		return
	}
	if strings.Contains(lowered, "tmdb:") {
		respond.Error(w, http.StatusBadRequest, "tmdb: id must be a positive integer")
		return
	}
	if id, ok := searchExtractImdbID(lowered); ok {
		s.handleSearchExternalID(w, r, id, "imdb_id")
		return
	}
	if id, ok := searchExtractTvdbID(lowered); ok {
		s.handleSearchExternalID(w, r, id, "tvdb_id")
		return
	}
	if year, ok := searchExtractYear(lowered); ok {
		s.handleSearchYear(w, r, query, year)
		return
	}
	s.handleSearchMulti(w, r, query, page)
}

func (s *Service) handleSearchMulti(w http.ResponseWriter, r *http.Request, query string, page int) {
	ctx := r.Context()
	resp, err := s.tmdb.SearchMulti(ctx, query, map[string]string{
		"page":          strconv.Itoa(page),
		"language":      tmdb.DefaultLanguage,
		"include_adult": "false",
	})
	if err != nil {
		empty := NewResultsPage(1, 1, 0, []SearchResult{})
		respond.JSON(w, empty)
		return
	}
	mapped := MapSearchResults(resp.Results)
	kept := WithoutAdult(mapped)
	respond.JSON(w, NewResultsPage(resp.Page, resp.TotalPages, resp.TotalResults, kept))
}

func (s *Service) handleSearchTmdb(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	items := make([]tmdb.MediaItem, 0, 3)

	var movie searchMovieDetails
	if err := s.tmdb.Get(ctx, "/movie/"+id, map[string]string{
		"language":               tmdb.DefaultLanguage,
		"append_to_response":     searchMovieAppend,
		"include_video_language": tmdb.DefaultLanguage,
	}, &movie); err == nil {
		if !strings.HasPrefix(tmdb.DefaultLanguage, "en") && !searchHasTrailer(movie.Videos.Results) {
			var fallback searchMovieDetails
			_ = s.tmdb.Get(ctx, "/movie/"+id, map[string]string{
				"language":               tmdb.DefaultLanguage,
				"append_to_response":     "videos",
				"include_video_language": "en",
			}, &fallback)
		}
		items = append(items, MapMovieDetailsToResult(movie.MovieDetails))
	}

	var show searchTvDetails
	if err := s.tmdb.Get(ctx, "/tv/"+id, map[string]string{
		"language":               tmdb.DefaultLanguage,
		"append_to_response":     searchTvAppend,
		"include_video_language": tmdb.DefaultLanguage,
	}, &show); err == nil {
		if !strings.HasPrefix(tmdb.DefaultLanguage, "en") && !searchHasTrailer(show.Videos.Results) {
			var fallback searchTvDetails
			_ = s.tmdb.Get(ctx, "/tv/"+id, map[string]string{
				"language":               tmdb.DefaultLanguage,
				"append_to_response":     "videos",
				"include_video_language": "en",
			}, &fallback)
		}
		items = append(items, MapTvDetailsToResult(show.TVDetails))
	}

	var person tmdb.PersonDetails
	if err := s.tmdb.Get(ctx, "/person/"+id, map[string]string{
		"language": tmdb.DefaultLanguage,
	}, &person); err == nil {
		items = append(items, MapPersonDetailsToResult(person))
	}

	mapped := MapSearchResults(items)
	kept := WithoutAdult(mapped)
	respond.JSON(w, NewResultsPage(1, 1, len(kept), kept))
}

func (s *Service) handleSearchExternalID(w http.ResponseWriter, r *http.Request, id, source string) {
	ctx := r.Context()
	resp, err := s.tmdb.GetByExternalID(ctx, id, source, map[string]string{
		"language": tmdb.DefaultLanguage,
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve search results.")
		return
	}
	items := make([]tmdb.MediaItem, 0, len(resp.MovieResults)+len(resp.TVResults)+len(resp.PersonResults))
	for i := range resp.MovieResults {
		movie := resp.MovieResults[i]
		items = append(items, tmdb.MediaItem{MediaType: "movie", Movie: &movie})
	}
	for i := range resp.TVResults {
		show := resp.TVResults[i]
		items = append(items, tmdb.MediaItem{MediaType: "tv", TV: &show})
	}
	for i := range resp.PersonResults {
		person := resp.PersonResults[i]
		items = append(items, tmdb.MediaItem{MediaType: "person", Person: &person})
	}
	mapped := MapSearchResults(items)
	kept := WithoutAdult(mapped)
	respond.JSON(w, NewResultsPage(1, 1, len(kept), kept))
}

func (s *Service) handleSearchYear(w http.ResponseWriter, r *http.Request, query, year string) {
	ctx := r.Context()
	stripped := searchStripYear(query)

	var moviePage *tmdb.SearchMoviesResponse
	movies, err := s.tmdb.SearchMovies(ctx, stripped, map[string]string{
		"page":                 "1",
		"include_adult":        "false",
		"language":             "en",
		"primary_release_year": year,
	})
	if err == nil {
		moviePage = movies
	}

	var tvPage *tmdb.SearchTVResponse
	tv, err := s.tmdb.SearchTVShows(ctx, stripped, map[string]string{
		"page":                "1",
		"include_adult":       "false",
		"language":            "en",
		"first_air_date_year": year,
	})
	if err == nil {
		tvPage = tv
	}

	items := []tmdb.MediaItem{}
	if moviePage != nil {
		for i := range moviePage.Results {
			movie := moviePage.Results[i]
			items = append(items, tmdb.MediaItem{MediaType: "movie", Movie: &movie})
		}
	}
	if tvPage != nil {
		for i := range tvPage.Results {
			show := tvPage.Results[i]
			items = append(items, tmdb.MediaItem{MediaType: "tv", TV: &show})
		}
	}
	mapped := MapSearchResults(items)
	kept := WithoutAdult(mapped)
	respond.JSON(w, NewResultsPage(1, 1, len(kept), kept))
}

func (s *Service) handleSearchKeyword(w http.ResponseWriter, r *http.Request) {
	query, ok := searchParseQuery(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "query must be non-empty")
		return
	}
	page, ok := searchParsePage(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "page must be a positive integer")
		return
	}
	resp, err := s.tmdb.SearchKeyword(r.Context(), query, map[string]string{
		"page": strconv.Itoa(page),
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve keyword search results.")
		return
	}
	if resp.Results == nil {
		resp.Results = []tmdb.KeywordSearchResult{}
	}
	respond.JSON(w, resp)
}

func (s *Service) handleSearchCompany(w http.ResponseWriter, r *http.Request) {
	query, ok := searchParseQuery(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "query must be non-empty")
		return
	}
	page, ok := searchParsePage(r)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "page must be a positive integer")
		return
	}
	resp, err := s.tmdb.SearchCompany(r.Context(), query, map[string]string{
		"page": strconv.Itoa(page),
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve company search results.")
		return
	}
	if resp.Results == nil {
		resp.Results = []tmdb.CompanySearchResult{}
	}
	respond.JSON(w, resp)
}
