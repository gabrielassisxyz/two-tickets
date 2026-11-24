package catalog

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The title page routes: movie, series, season, collection and person
// details, with recommendations, similar titles and combined credits. They
// port server/routes/{movie,tv,collection,person}.ts at the pinned commit and
// are pinned by the goldens under testdata/golden/title.
//
// Every TMDB request a route sends is the one the recorded fixtures specify.
// Upstream's fallback locale is plain "en", so the English requests below
// carry "en", not "en-US".

const (
	titleEnglishLanguage = "en"
	titleMovieAppend     = "credits,external_ids,videos,keywords,release_dates,watch/providers"
	titleTvAppend        = "aggregate_credits,credits,external_ids,keywords,videos,content_ratings,watch/providers"
)

func (s *Service) registerTitles(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/movie/{movieId}", s.handleTitleMovie)
	mux.HandleFunc("GET /api/v1/movie/{movieId}/recommendations", s.handleTitleMovieRecommendations)
	mux.HandleFunc("GET /api/v1/movie/{movieId}/similar", s.handleTitleMovieSimilar)
	mux.HandleFunc("GET /api/v1/tv/{tvId}", s.handleTitleTv)
	mux.HandleFunc("GET /api/v1/tv/{tvId}/season/{seasonNumber}", s.handleTitleTvSeason)
	mux.HandleFunc("GET /api/v1/tv/{tvId}/recommendations", s.handleTitleTvRecommendations)
	mux.HandleFunc("GET /api/v1/tv/{tvId}/similar", s.handleTitleTvSimilar)
	mux.HandleFunc("GET /api/v1/collection/{collectionId}", s.handleTitleCollection)
	mux.HandleFunc("GET /api/v1/person/{personId}", s.handleTitlePerson)
	mux.HandleFunc("GET /api/v1/person/{personId}/combined_credits", s.handleTitlePersonCredits)
}

// titlePathNumber reads a numeric path value. Ids must be positive; a season
// number may be 0, TMDB's specials season. Anything else answers 400 here,
// where upstream's OpenAPI validator answers 400 for a non-number only.
func titlePathNumber(w http.ResponseWriter, r *http.Request, name string, allowZero bool) (int, bool) {
	value, err := strconv.ParseUint(r.PathValue(name), 10, 31)
	if err != nil || (value == 0 && !allowZero) {
		titleRejectInput(w, "params/"+name)
		return 0, false
	}
	return int(value), true
}

// titleQueryPage reads ?page. The frontend always sends it; a request without
// one asks for the first page, as upstream's default does.
func titleQueryPage(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}
	value, err := strconv.ParseUint(raw, 10, 31)
	if err != nil || value == 0 {
		titleRejectInput(w, "query/page")
		return 0, false
	}
	return int(value), true
}

func titleRejectInput(w http.ResponseWriter, field string) {
	message := "request/" + field + " must be a positive integer"
	if field == "params/seasonNumber" {
		message = "request/" + field + " must be a non-negative integer"
	}
	respond.Error(w, http.StatusBadRequest, message, message)
}

func titleLocalisedParams() map[string]string {
	return map[string]string{"language": tmdb.DefaultLanguage}
}

func titlePageParams(page int) map[string]string {
	return map[string]string{"language": tmdb.DefaultLanguage, "page": strconv.Itoa(page)}
}

// titleDetailsParams is the parameter set of upstream's getMovie and
// getTvShow: the display language, the appended blocks, and the language the
// videos are asked in.
func titleDetailsParams(language, appendTo, videoLanguage string) map[string]string {
	return map[string]string{
		"language":               language,
		"append_to_response":     appendTo,
		"include_video_language": videoLanguage,
	}
}

func (s *Service) handleTitleMovie(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "movieId", false)
	if !ok {
		return
	}
	details, found, err := s.fetchTitleMovie(r.Context(), id)
	switch {
	case err != nil:
		// Upstream answers 500 for every failure, a TMDB 404 included, and
		// the kept movie page relies on it.
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve movie.")
	case !found:
		respond.Error(w, http.StatusNotFound, "Movie not found.")
	default:
		respond.JSON(w, details)
	}
}

// fetchTitleMovie reports found false for an adult title, which the adult
// policy refuses before any fallback request is spent on it.
func (s *Service) fetchTitleMovie(ctx context.Context, id int) (titleMovieDetails, bool, error) {
	path := "/movie/" + strconv.Itoa(id)
	var movie titleMovieTMDB
	if err := s.tmdb.Get(ctx, path, titleDetailsParams(tmdb.DefaultLanguage, titleMovieAppend, tmdb.DefaultLanguage), &movie); err != nil {
		return titleMovieDetails{}, false, err
	}
	if movie.Adult {
		return titleMovieDetails{}, false, nil
	}
	if !titleHasTrailer(movie.Videos) {
		var english titleVideosTMDB
		// A failed English video request leaves the localised videos, as
		// upstream swallows the same failure.
		if err := s.tmdb.Get(ctx, path, titleDetailsParams(tmdb.DefaultLanguage, "videos", titleEnglishLanguage), &english); err == nil {
			movie.Videos = titleMergeEnglishTrailers(movie.Videos, english.Videos)
		}
	}
	details := mapTitleMovieDetails(movie)
	if details.Overview == "" {
		var english titleMovieTMDB
		// Upstream fails the whole page when this retry fails; an empty
		// overview is the intentional normalisation.
		if err := s.tmdb.Get(ctx, path, titleDetailsParams(titleEnglishLanguage, titleMovieAppend, titleEnglishLanguage), &english); err == nil {
			details.Overview = english.Overview
		}
	}
	return details, true, nil
}

func (s *Service) handleTitleTv(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "tvId", false)
	if !ok {
		return
	}
	details, found, err := s.fetchTitleTv(r.Context(), id)
	switch {
	case err != nil:
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve series.")
	case !found:
		respond.Error(w, http.StatusNotFound, "Series not found.")
	default:
		respond.JSON(w, details)
	}
}

// fetchTitleTv asks for the English details first, as upstream's probe for
// the anime keyword does. Series always come from TMDB, anime included, so
// that answer only serves as the overview fallback, which upstream fetches
// with the very same request.
func (s *Service) fetchTitleTv(ctx context.Context, id int) (titleTvDetails, bool, error) {
	path := "/tv/" + strconv.Itoa(id)
	var english titleTvTMDB
	englishErr := s.tmdb.Get(ctx, path, titleDetailsParams(titleEnglishLanguage, titleTvAppend, titleEnglishLanguage), &english)
	var show titleTvTMDB
	if err := s.tmdb.Get(ctx, path, titleDetailsParams(tmdb.DefaultLanguage, titleTvAppend, tmdb.DefaultLanguage), &show); err != nil {
		return titleTvDetails{}, false, err
	}
	if show.Adult {
		return titleTvDetails{}, false, nil
	}
	if !titleHasTrailer(show.Videos) {
		var videos titleVideosTMDB
		if err := s.tmdb.Get(ctx, path, titleDetailsParams(tmdb.DefaultLanguage, "videos", titleEnglishLanguage), &videos); err == nil {
			show.Videos = titleMergeEnglishTrailers(show.Videos, videos.Videos)
		}
	}
	details := mapTitleTvDetails(show)
	if details.Overview == "" && englishErr == nil {
		details.Overview = english.Overview
	}
	return details, true, nil
}

func (s *Service) handleTitleTvSeason(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "tvId", false)
	if !ok {
		return
	}
	number, ok := titlePathNumber(w, r, "seasonNumber", true)
	if !ok {
		return
	}
	season, found, err := s.fetchTitleTvSeason(r.Context(), id, number)
	switch {
	case err != nil:
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve season.")
	case !found:
		respond.Error(w, http.StatusNotFound, "Series not found.")
	default:
		respond.JSON(w, season)
	}
}

// fetchTitleTvSeason asks for the show before the season, as upstream does.
// The show answer carries the adult flag the season itself lacks, and a
// show TMDB cannot answer fails its seasons as it does upstream.
func (s *Service) fetchTitleTvSeason(ctx context.Context, id, number int) (titleSeasonWithEpisodes, bool, error) {
	path := "/tv/" + strconv.Itoa(id)
	var show titleTvTMDB
	if err := s.tmdb.Get(ctx, path, titleDetailsParams(titleEnglishLanguage, titleTvAppend, titleEnglishLanguage), &show); err != nil {
		return titleSeasonWithEpisodes{}, false, err
	}
	if show.Adult {
		return titleSeasonWithEpisodes{}, false, nil
	}
	params := map[string]string{"language": tmdb.DefaultLanguage, "append_to_response": "external_ids"}
	var season titleSeasonTMDB
	if err := s.tmdb.Get(ctx, path+"/season/"+strconv.Itoa(number), params, &season); err != nil {
		return titleSeasonWithEpisodes{}, false, err
	}
	return mapTitleSeasonWithEpisodes(season), true, nil
}

func (s *Service) handleTitleMovieRecommendations(w http.ResponseWriter, r *http.Request) {
	s.serveTitleMovieList(w, r, (*tmdb.Client).GetMovieRecommendations, "Unable to retrieve movie recommendations.")
}

func (s *Service) handleTitleMovieSimilar(w http.ResponseWriter, r *http.Request) {
	s.serveTitleMovieList(w, r, (*tmdb.Client).GetMovieSimilar, "Unable to retrieve similar movies.")
}

func (s *Service) handleTitleTvRecommendations(w http.ResponseWriter, r *http.Request) {
	s.serveTitleTvList(w, r, (*tmdb.Client).GetTVRecommendations, "Unable to retrieve series recommendations.")
}

func (s *Service) handleTitleTvSimilar(w http.ResponseWriter, r *http.Request) {
	s.serveTitleTvList(w, r, (*tmdb.Client).GetTVSimilar, "Unable to retrieve similar series.")
}

type titleMovieListFetch func(*tmdb.Client, context.Context, string, map[string]string) (*tmdb.DiscoverMoviesResponse, error)

type titleTvListFetch func(*tmdb.Client, context.Context, string, map[string]string) (*tmdb.DiscoverTVResponse, error)

// The page counts stay TMDB's after the adult filter, as upstream's do.
func (s *Service) serveTitleMovieList(w http.ResponseWriter, r *http.Request, fetch titleMovieListFetch, failure string) {
	id, ok := titlePathNumber(w, r, "movieId", false)
	if !ok {
		return
	}
	page, ok := titleQueryPage(w, r)
	if !ok {
		return
	}
	list, err := fetch(s.tmdb, r.Context(), strconv.Itoa(id), titlePageParams(page))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, failure)
		return
	}
	results := make([]MovieResult, 0, len(list.Results))
	for _, movie := range list.Results {
		results = append(results, MapMovieResult(movie))
	}
	respond.JSON(w, NewResultsPage(list.Page, list.TotalPages, list.TotalResults, WithoutAdult(results)))
}

func (s *Service) serveTitleTvList(w http.ResponseWriter, r *http.Request, fetch titleTvListFetch, failure string) {
	id, ok := titlePathNumber(w, r, "tvId", false)
	if !ok {
		return
	}
	page, ok := titleQueryPage(w, r)
	if !ok {
		return
	}
	list, err := fetch(s.tmdb, r.Context(), strconv.Itoa(id), titlePageParams(page))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, failure)
		return
	}
	results := make([]TvResult, 0, len(list.Results))
	for _, show := range list.Results {
		results = append(results, MapTvResult(show))
	}
	respond.JSON(w, NewResultsPage(list.Page, list.TotalPages, list.TotalResults, WithoutAdult(results)))
}

func (s *Service) handleTitleCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "collectionId", false)
	if !ok {
		return
	}
	var collection titleCollectionTMDB
	if err := s.tmdb.Get(r.Context(), "/collection/"+strconv.Itoa(id), titleLocalisedParams(), &collection); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve collection.")
		return
	}
	respond.JSON(w, mapTitleCollection(collection))
}

// A person flagged adult is still served: the policy names titles and
// credits, and the person's adult credits are what the credits route drops.
func (s *Service) handleTitlePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "personId", false)
	if !ok {
		return
	}
	var person titlePersonTMDB
	if err := s.tmdb.Get(r.Context(), "/person/"+strconv.Itoa(id), titleLocalisedParams(), &person); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve person.")
		return
	}
	respond.JSON(w, mapTitlePersonDetails(person))
}

func (s *Service) handleTitlePersonCredits(w http.ResponseWriter, r *http.Request) {
	id, ok := titlePathNumber(w, r, "personId", false)
	if !ok {
		return
	}
	var credits titleCombinedCreditsTMDB
	if err := s.tmdb.Get(r.Context(), "/person/"+strconv.Itoa(id)+"/combined_credits", titleLocalisedParams(), &credits); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve combined credits.")
		return
	}
	respond.JSON(w, mapTitleCombinedCredits(credits))
}
