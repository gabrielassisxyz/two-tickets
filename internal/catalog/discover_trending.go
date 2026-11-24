package catalog

import (
	"net/http"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// discoverAnonymousLanguage is the language upstream sends where it builds
// its TMDB client without a user: the server locale of upstream's default
// settings. The recorded fixtures for /backdrops and the genre sliders carry
// it, and both routes read only artwork paths, so it never reaches the screen.
const discoverAnonymousLanguage = "en"

// handleDiscoverTrending keeps upstream's defaults: every media type, the
// daily window, region BR on the mixed list only.
func (s *Service) handleDiscoverTrending(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, ok := discoverParsePage(query)
	if !ok {
		discoverBadRequest(w, "page must be a positive integer")
		return
	}
	window, ok := discoverTrendingWindow(query.Get("timeWindow"))
	if !ok {
		discoverBadRequest(w, "timeWindow must be day or week")
		return
	}
	params := map[string]string{"page": page, "language": tmdb.DefaultLanguage}
	const failure = "Unable to retrieve trending items."
	switch mediaType := query.Get("mediaType"); mediaType {
	case "", "all":
		params["region"] = tmdb.DefaultRegion
		data, err := s.tmdb.GetAllTrending(r.Context(), window, params)
		if err != nil {
			discoverFailed(w, failure)
			return
		}
		// MapSearchResults goes by media_type, so a collection is typed as a
		// collection; upstream's trending mapper types it as a movie.
		results := WithoutAdult(MapSearchResults(data.Results))
		respond.JSON(w, NewResultsPage(data.Page, data.TotalPages, data.TotalResults, results))
	case "movie":
		var data tmdb.DiscoverMoviesResponse
		if err := s.tmdb.Get(r.Context(), "/trending/movie/"+window, params, &data); err != nil {
			discoverFailed(w, failure)
			return
		}
		respond.JSON(w, discoverMoviePage(&data))
	case "tv":
		var data tmdb.DiscoverTVResponse
		if err := s.tmdb.Get(r.Context(), "/trending/tv/"+window, params, &data); err != nil {
			discoverFailed(w, failure)
			return
		}
		respond.JSON(w, discoverTVResultsPage(&data))
	default:
		discoverBadRequest(w, "mediaType must be all, movie or tv")
	}
}

func discoverTrendingWindow(raw string) (string, bool) {
	switch raw {
	case "", "day":
		return "day", true
	case "week":
		return "week", true
	}
	return "", false
}

// handleDiscoverBackdrops serves the login page's background: the artwork of
// the weekly all-media trending list, collections included, people and adult
// titles left out. It is one of the routes that answers without a session.
func (s *Service) handleDiscoverBackdrops(w http.ResponseWriter, r *http.Request) {
	params := map[string]string{"page": "1", "language": discoverAnonymousLanguage, "region": tmdb.DefaultRegion}
	data, err := s.tmdb.GetAllTrending(r.Context(), "week", params)
	if err != nil {
		discoverFailed(w, "Unable to retrieve backdrops.")
		return
	}
	respond.JSON(w, discoverBackdropsOf(WithoutAdult(MapSearchResults(data.Results))))
}

func discoverBackdropOf(result SearchResult) string {
	var path *string
	switch result := result.(type) {
	case MovieResult:
		path = result.BackdropPath
	case TvResult:
		path = result.BackdropPath
	case CollectionResult:
		path = result.BackdropPath
	}
	if path == nil {
		return ""
	}
	return *path
}
