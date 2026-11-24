package catalog

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The browse pages: movies or series by original language, genre, studio or
// network. Each answers an enriched envelope, the page plus the object the
// page is about. Upstream takes the display language from ?language here;
// in this app it is always pt-BR, so the query's language is ignored.

// discoverStudio is upstream's mapProductionCompany (server/models/Movie.ts).
// Upstream drops a field TMDB left out and keeps one TMDB sent as null, so
// every optional field keeps all three states.
type discoverStudio struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	OriginCountry tmdb.Optional[string] `json:"originCountry,omitzero"`
	Description   tmdb.Optional[string] `json:"description,omitzero"`
	Headquarters  tmdb.Optional[string] `json:"headquarters,omitzero"`
	Homepage      tmdb.Optional[string] `json:"homepage,omitzero"`
	LogoPath      tmdb.Optional[string] `json:"logoPath,omitzero"`
}

type discoverStudioTMDB struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	OriginCountry tmdb.Optional[string] `json:"origin_country"`
	Description   tmdb.Optional[string] `json:"description"`
	Headquarters  tmdb.Optional[string] `json:"headquarters"`
	Homepage      tmdb.Optional[string] `json:"homepage"`
	LogoPath      tmdb.Optional[string] `json:"logo_path"`
}

// discoverNetwork is upstream's mapNetwork (server/models/Tv.ts).
type discoverNetwork struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	OriginCountry tmdb.Optional[string] `json:"originCountry,omitzero"`
	Headquarters  tmdb.Optional[string] `json:"headquarters,omitzero"`
	Homepage      tmdb.Optional[string] `json:"homepage,omitzero"`
	LogoPath      tmdb.Optional[string] `json:"logoPath,omitzero"`
}

type discoverNetworkTMDB struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	OriginCountry tmdb.Optional[string] `json:"origin_country"`
	Headquarters  tmdb.Optional[string] `json:"headquarters"`
	Homepage      tmdb.Optional[string] `json:"homepage"`
	LogoPath      tmdb.Optional[string] `json:"logo_path"`
}

func (s *Service) handleDiscoverMoviesByLanguage(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve movies by language."
	language, page, ok := s.discoverFindLanguage(w, r, failure)
	if !ok {
		return
	}
	params := discoverBaseParams(discoverMovieMedia, page)
	params["with_original_language"] = language.ISO639_1
	data, err := s.tmdb.GetDiscoverMovies(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverLanguagePage[MovieResult]{ResultsPage: discoverMoviePage(data), Language: language})
}

func (s *Service) handleDiscoverTVByLanguage(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve series by language."
	language, page, ok := s.discoverFindLanguage(w, r, failure)
	if !ok {
		return
	}
	params := discoverBaseParams(discoverTVMedia, page)
	params["with_original_language"] = language.ISO639_1
	data, err := s.tmdb.GetDiscoverTV(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverLanguagePage[TvResult]{ResultsPage: discoverTVResultsPage(data), Language: language})
}

// discoverFindLanguage resolves the {language} of the path against TMDB's
// language list, answering the request itself when it cannot: 400 for a bad
// page, upstream's 404 for a language TMDB does not list, 500 on failure.
func (s *Service) discoverFindLanguage(w http.ResponseWriter, r *http.Request, failure string) (tmdb.Language, string, bool) {
	page, ok := discoverParsePage(r.URL.Query())
	if !ok {
		discoverBadRequest(w, "page must be a positive integer")
		return tmdb.Language{}, "", false
	}
	languages, err := s.tmdb.GetLanguages(r.Context())
	if err != nil {
		discoverFailed(w, failure)
		return tmdb.Language{}, "", false
	}
	code := r.PathValue("language")
	for _, language := range languages {
		if language.ISO639_1 == code {
			return language, page, true
		}
	}
	discoverNotFound(w, "Language not found.")
	return tmdb.Language{}, "", false
}

func (s *Service) handleDiscoverMoviesByGenre(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve movies by genre."
	genre, page, ok := s.discoverFindGenre(w, r, "/genre/movie/list", failure)
	if !ok {
		return
	}
	params := discoverBaseParams(discoverMovieMedia, page)
	params["with_genres"] = r.PathValue("id")
	data, err := s.tmdb.GetDiscoverMovies(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverGenrePage[MovieResult]{ResultsPage: discoverMoviePage(data), Genre: genre})
}

func (s *Service) handleDiscoverTVByGenre(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve series by genre."
	genre, page, ok := s.discoverFindGenre(w, r, "/genre/tv/list", failure)
	if !ok {
		return
	}
	params := discoverBaseParams(discoverTVMedia, page)
	params["with_genres"] = r.PathValue("id")
	data, err := s.tmdb.GetDiscoverTV(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverGenrePage[TvResult]{ResultsPage: discoverTVResultsPage(data), Genre: genre})
}

// discoverFindGenre resolves the {id} of the path against the genre list at
// listPath, answering the request itself when it cannot, as
// discoverFindLanguage does.
func (s *Service) discoverFindGenre(w http.ResponseWriter, r *http.Request, listPath, failure string) (tmdb.Genre, string, bool) {
	id, page, problem := discoverPathIDAndPage(r)
	if problem != "" {
		discoverBadRequest(w, problem)
		return tmdb.Genre{}, "", false
	}
	genres, err := discoverFetchGenres(r.Context(), s.tmdb, listPath)
	if err != nil {
		discoverFailed(w, failure)
		return tmdb.Genre{}, "", false
	}
	for _, genre := range genres {
		if strconv.Itoa(genre.ID) == id {
			return genre, page, true
		}
	}
	discoverNotFound(w, "Genre not found.")
	return tmdb.Genre{}, "", false
}

func (s *Service) handleDiscoverMoviesByStudio(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve movies by studio."
	id, page, problem := discoverPathIDAndPage(r)
	if problem != "" {
		discoverBadRequest(w, problem)
		return
	}
	var studio discoverStudioTMDB
	if err := s.tmdb.Get(r.Context(), "/company/"+id, nil, &studio); err != nil {
		discoverFailed(w, failure)
		return
	}
	params := discoverBaseParams(discoverMovieMedia, page)
	params["with_companies"] = id
	data, err := s.tmdb.GetDiscoverMovies(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverStudioPage{ResultsPage: discoverMoviePage(data), Studio: discoverStudio(studio)})
}

func (s *Service) handleDiscoverTVByNetwork(w http.ResponseWriter, r *http.Request) {
	const failure = "Unable to retrieve series by network."
	id, page, problem := discoverPathIDAndPage(r)
	if problem != "" {
		discoverBadRequest(w, problem)
		return
	}
	var network discoverNetworkTMDB
	if err := s.tmdb.Get(r.Context(), "/network/"+id, nil, &network); err != nil {
		discoverFailed(w, failure)
		return
	}
	params := discoverBaseParams(discoverTVMedia, page)
	params["with_networks"] = id
	data, err := s.tmdb.GetDiscoverTV(r.Context(), params)
	if err != nil {
		discoverFailed(w, failure)
		return
	}
	respond.JSON(w, discoverNetworkPage{ResultsPage: discoverTVResultsPage(data), Network: discoverNetwork(network)})
}

// discoverFetchGenres is upstream's getMovieGenres or getTvGenres: the pt-BR
// list, a name TMDB left empty filled from the English list, genres nameless
// in both dropped, and the rest sorted by name the way lodash's sortBy
// compares strings.
func discoverFetchGenres(ctx context.Context, client *tmdb.Client, listPath string) ([]tmdb.Genre, error) {
	var localised tmdb.GenreList
	if err := client.Get(ctx, listPath, map[string]string{"language": tmdb.DefaultLanguage}, &localised); err != nil {
		return nil, err
	}
	if err := discoverFillEnglishGenreNames(ctx, client, listPath, localised.Genres); err != nil {
		return nil, err
	}
	genres := make([]tmdb.Genre, 0, len(localised.Genres))
	for _, genre := range localised.Genres {
		if genre.Name != "" {
			genres = append(genres, genre)
		}
	}
	sort.SliceStable(genres, func(i, j int) bool { return genres[i].Name < genres[j].Name })
	return genres, nil
}

func discoverFillEnglishGenreNames(ctx context.Context, client *tmdb.Client, listPath string, genres []tmdb.Genre) error {
	nameless := false
	for _, genre := range genres {
		nameless = nameless || genre.Name == ""
	}
	if !nameless {
		return nil
	}
	var english tmdb.GenreList
	if err := client.Get(ctx, listPath, map[string]string{"language": "en"}, &english); err != nil {
		return err
	}
	englishNames := make(map[int]string, len(english.Genres))
	for _, genre := range english.Genres {
		englishNames[genre.ID] = genre.Name
	}
	for i := range genres {
		if genres[i].Name == "" {
			genres[i].Name = englishNames[genres[i].ID]
		}
	}
	return nil
}
