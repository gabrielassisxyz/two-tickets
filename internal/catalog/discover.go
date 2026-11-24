package catalog

import (
	"context"
	"net/http"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The Discover family: the paginated discover routes, trending, the
// language, genre, studio, network and keyword pages, the genre sliders, the
// slider manifest and /backdrops. Shapes are upstream's
// server/routes/discover.ts and the /backdrops route of server/routes/index.ts,
// pinned by testdata/golden/discover. Every TMDB request carries exactly the
// parameters the recorded fixtures show upstream sending, and a failed TMDB
// call answers upstream's 500 with upstream's message.

func (s *Service) registerDiscover(mux watchlistRouteMux) {
	mux.HandleFunc("GET /api/v1/settings/discover", s.handleDiscoverSliderManifest)
	mux.HandleFunc("GET /api/v1/discover/movies", s.handleDiscoverMovies)
	mux.HandleFunc("GET /api/v1/discover/tv", s.handleDiscoverTV)
	mux.HandleFunc("GET /api/v1/discover/movies/upcoming", s.handleDiscoverMoviesUpcoming)
	mux.HandleFunc("GET /api/v1/discover/tv/upcoming", s.handleDiscoverTVUpcoming)
	mux.HandleFunc("GET /api/v1/discover/movies/language/{language}", s.handleDiscoverMoviesByLanguage)
	mux.HandleFunc("GET /api/v1/discover/tv/language/{language}", s.handleDiscoverTVByLanguage)
	mux.HandleFunc("GET /api/v1/discover/movies/genre/{id}", s.handleDiscoverMoviesByGenre)
	mux.HandleFunc("GET /api/v1/discover/tv/genre/{id}", s.handleDiscoverTVByGenre)
	mux.HandleFunc("GET /api/v1/discover/movies/studio/{id}", s.handleDiscoverMoviesByStudio)
	mux.HandleFunc("GET /api/v1/discover/tv/network/{id}", s.handleDiscoverTVByNetwork)
	mux.HandleFunc("GET /api/v1/discover/keyword/{id}/movies", s.handleDiscoverMoviesByKeyword)
	mux.HandleFunc("GET /api/v1/discover/trending", s.handleDiscoverTrending)
	mux.HandleFunc("GET /api/v1/discover/genreslider/movie", s.handleDiscoverMovieGenreSlider)
	mux.HandleFunc("GET /api/v1/discover/genreslider/tv", s.handleDiscoverTVGenreSlider)
	mux.HandleFunc("GET /api/v1/backdrops", s.handleDiscoverBackdrops)
}

// discoverMoviesPage and discoverTVPage are /discover/movies and /discover/tv:
// the page plus the details of every keyword the query filtered by.
type discoverMoviesPage struct {
	ResultsPage[MovieResult]
	Keywords []tmdb.KeywordDetails `json:"keywords"`
}

type discoverTVPage struct {
	ResultsPage[TvResult]
	Keywords []tmdb.KeywordDetails `json:"keywords"`
}

type discoverLanguagePage[T any] struct {
	ResultsPage[T]
	Language tmdb.Language `json:"language"`
}

type discoverGenrePage[T any] struct {
	ResultsPage[T]
	Genre tmdb.Genre `json:"genre"`
}

type discoverStudioPage struct {
	ResultsPage[MovieResult]
	Studio discoverStudio `json:"studio"`
}

type discoverNetworkPage struct {
	ResultsPage[TvResult]
	Network discoverNetwork `json:"network"`
}

func discoverBadRequest(w http.ResponseWriter, problem string) {
	respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), problem)
}

func discoverNotFound(w http.ResponseWriter, message string) {
	respond.Error(w, http.StatusNotFound, message)
}

func discoverFailed(w http.ResponseWriter, message string) {
	respond.Error(w, http.StatusInternalServerError, message)
}

// discoverMoviePage maps a TMDB movie list and drops adult titles, keeping
// TMDB's totals as upstream does.
func discoverMoviePage(data *tmdb.DiscoverMoviesResponse) ResultsPage[MovieResult] {
	results := make([]MovieResult, 0, len(data.Results))
	for _, movie := range data.Results {
		results = append(results, MapMovieResult(movie))
	}
	return NewResultsPage(data.Page, data.TotalPages, data.TotalResults, WithoutAdult(results))
}

func discoverTVResultsPage(data *tmdb.DiscoverTVResponse) ResultsPage[TvResult] {
	results := make([]TvResult, 0, len(data.Results))
	for _, show := range data.Results {
		results = append(results, MapTvResult(show))
	}
	return NewResultsPage(data.Page, data.TotalPages, data.TotalResults, WithoutAdult(results))
}

func (s *Service) handleDiscoverMovies(w http.ResponseWriter, r *http.Request) {
	params, problem := discoverParseQuery(discoverMovieMedia, r.URL.Query(), s.now())
	if problem != "" {
		discoverBadRequest(w, problem)
		return
	}
	const failure = "Unable to retrieve popular movies."
	data, err := s.tmdb.GetDiscoverMovies(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	keywords, err := s.discoverKeywordDetails(r.Context(), params["with_keywords"])
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverMoviesPage{ResultsPage: discoverMoviePage(data), Keywords: keywords})
}

func (s *Service) handleDiscoverTV(w http.ResponseWriter, r *http.Request) {
	params, problem := discoverParseQuery(discoverTVMedia, r.URL.Query(), s.now())
	if problem != "" {
		discoverBadRequest(w, problem)
		return
	}
	const failure = "Unable to retrieve popular series."
	data, err := s.tmdb.GetDiscoverTV(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	keywords, err := s.discoverKeywordDetails(r.Context(), params["with_keywords"])
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverTVPage{ResultsPage: discoverTVResultsPage(data), Keywords: keywords})
}

// discoverKeywordDetails looks up every keyword of a filter. A keyword TMDB
// does not know is dropped, as upstream drops its null.
func (s *Service) discoverKeywordDetails(ctx context.Context, filter string) ([]tmdb.KeywordDetails, error) {
	keywords := []tmdb.KeywordDetails{}
	if filter == "" {
		return keywords, nil
	}
	ids := strings.FieldsFunc(filter, func(r rune) bool { return r == ',' || r == '|' })
	for _, id := range ids {
		detail, err := s.tmdb.GetKeywordDetails(ctx, id)
		if err != nil {
			return nil, err
		}
		if detail != nil {
			keywords = append(keywords, *detail)
		}
	}
	return keywords, nil
}

func (s *Service) handleDiscoverMoviesUpcoming(w http.ResponseWriter, r *http.Request) {
	page, ok := discoverParsePage(r.URL.Query())
	if !ok {
		discoverBadRequest(w, "page must be a positive integer")
		return
	}
	data, err := s.tmdb.GetDiscoverMovies(r.Context(), discoverUpcomingParams(discoverMovieMedia, page, s.now()))
	if err != nil {
		discoverFailed(w, "Unable to retrieve upcoming movies.")
		return
	}
	respond.JSON(w, discoverMoviePage(data))
}

func (s *Service) handleDiscoverTVUpcoming(w http.ResponseWriter, r *http.Request) {
	page, ok := discoverParsePage(r.URL.Query())
	if !ok {
		discoverBadRequest(w, "page must be a positive integer")
		return
	}
	data, err := s.tmdb.GetDiscoverTV(r.Context(), discoverUpcomingParams(discoverTVMedia, page, s.now()))
	if err != nil {
		discoverFailed(w, "Unable to retrieve upcoming series.")
		return
	}
	respond.JSON(w, discoverTVResultsPage(data))
}

func (s *Service) handleDiscoverMoviesByKeyword(w http.ResponseWriter, r *http.Request) {
	id, page, problem := discoverPathIDAndPage(r)
	if problem != "" {
		discoverBadRequest(w, problem)
		return
	}
	// The keyword route keeps upstream's own parameters: no region, no
	// filters, only the page and the display language.
	params := map[string]string{"page": page, "language": tmdb.DefaultLanguage}
	var data tmdb.DiscoverMoviesResponse
	if err := s.tmdb.Get(r.Context(), "/keyword/"+id+"/movies", params, &data); err != nil {
		discoverFailed(w, "Unable to retrieve movies by keyword.")
		return
	}
	respond.JSON(w, discoverMoviePage(&data))
}

// discoverPathIDAndPage reads the {id} of a path and the page of its query.
func discoverPathIDAndPage(r *http.Request) (id, page, problem string) {
	id, ok := discoverCheckID(r.PathValue("id"))
	if !ok {
		return "", "", "id must be a positive integer"
	}
	page, ok = discoverParsePage(r.URL.Query())
	if !ok {
		return "", "", "page must be a positive integer"
	}
	return id, page, ""
}
