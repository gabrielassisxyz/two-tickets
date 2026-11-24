package catalog

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// Lookup routes serve the filter and detail lookups the browse pages need:
// genres, keyword, studio, network, watch providers, languages, regions and
// certifications. Shapes match Seerr's own responses and are pinned by goldens
// under testdata/golden/lookup.

// lookupGenreItem is one genre as the frontend reads it.
type lookupGenreItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// lookupGenreRaw is one genre as TMDB returns it. Name is a pointer so an
// absent name and an empty one both read as missing and trigger the English
// fallback, while a present name survives unchanged.
type lookupGenreRaw struct {
	ID   int     `json:"id"`
	Name *string `json:"name"`
}

type lookupGenreList struct {
	Genres []lookupGenreRaw `json:"genres"`
}

// lookupStudioResponse is Seerr's ProductionCompany shape.
type lookupStudioResponse struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry string  `json:"originCountry"`
	Description   *string `json:"description,omitempty"`
	Headquarters  *string `json:"headquarters,omitempty"`
	Homepage      *string `json:"homepage,omitempty"`
	LogoPath      *string `json:"logoPath,omitempty"`
}

// lookupStudioTMDB is the TMDB company shape the studio route reads.
type lookupStudioTMDB struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry string  `json:"origin_country"`
	Description   *string `json:"description"`
	Headquarters  *string `json:"headquarters"`
	Homepage      *string `json:"homepage"`
	LogoPath      *string `json:"logo_path"`
}

// lookupNetworkResponse is Seerr's TvNetwork shape.
type lookupNetworkResponse struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry *string `json:"originCountry,omitempty"`
	Headquarters  *string `json:"headquarters,omitempty"`
	Homepage      *string `json:"homepage,omitempty"`
	LogoPath      *string `json:"logoPath,omitempty"`
}

// lookupNetworkTMDB is the TMDB network shape the network route reads.
type lookupNetworkTMDB struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry *string `json:"origin_country"`
	Headquarters  *string `json:"headquarters"`
	Homepage      *string `json:"homepage"`
	LogoPath      *string `json:"logo_path"`
}

// lookupWatchProvider is one provider as the frontend reads it.
type lookupWatchProvider struct {
	DisplayPriority *int    `json:"displayPriority,omitempty"`
	LogoPath        *string `json:"logoPath,omitempty"`
	ID              int     `json:"id"`
	Name            string  `json:"name"`
}

// lookupWatchProviderTMDB is one provider as TMDB returns it.
type lookupWatchProviderTMDB struct {
	ProviderID      int     `json:"provider_id"`
	ProviderName    string  `json:"provider_name"`
	LogoPath        *string `json:"logo_path"`
	DisplayPriority *int    `json:"display_priority"`
}

type lookupWatchProvidersTMDB struct {
	Results []lookupWatchProviderTMDB `json:"results"`
}

type lookupWatchRegionsTMDB struct {
	Results []tmdb.WatchProviderRegion `json:"results"`
}

func (s *Service) registerLookups(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/genres/movie", http.HandlerFunc(s.handleLookupMovieGenres))
	mux.Handle("GET /api/v1/genres/tv", http.HandlerFunc(s.handleLookupTVGenres))
	mux.Handle("GET /api/v1/keyword/{id}", http.HandlerFunc(s.handleLookupKeyword))
	mux.Handle("GET /api/v1/studio/{id}", http.HandlerFunc(s.handleLookupStudio))
	mux.Handle("GET /api/v1/network/{id}", http.HandlerFunc(s.handleLookupNetwork))
	mux.Handle("GET /api/v1/watchproviders/regions", http.HandlerFunc(s.handleLookupWatchRegions))
	mux.Handle("GET /api/v1/watchproviders/movies", http.HandlerFunc(s.handleLookupMovieProviders))
	mux.Handle("GET /api/v1/watchproviders/tv", http.HandlerFunc(s.handleLookupTVProviders))
	mux.Handle("GET /api/v1/languages", http.HandlerFunc(s.handleLookupLanguages))
	mux.Handle("GET /api/v1/regions", http.HandlerFunc(s.handleLookupRegions))
	mux.Handle("GET /api/v1/certifications/movie", http.HandlerFunc(s.handleLookupMovieCertifications))
	mux.Handle("GET /api/v1/certifications/tv", http.HandlerFunc(s.handleLookupTVCertifications))
}

// lookupParsePositiveID reads a path id and reports whether it is a positive
// integer. Anything else is a 400: the frontend only ever sends ids from lists
// it already holds.
func lookupParsePositiveID(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return "", false
	}
	return strconv.Itoa(n), true
}

// lookupRequestLanguage is the display locale the genre routes ask TMDB for:
// an explicit ?language wins, otherwise the project default. Upstream reads
// the same value from req.query.language ?? req.locale.
func lookupRequestLanguage(r *http.Request) string {
	if lang := r.URL.Query().Get("language"); lang != "" {
		return lang
	}
	return tmdb.DefaultLanguage
}

// lookupFetchGenres asks TMDB for one genre list, falling back to English
// names where the localised list leaves a name empty, dropping genres with no
// name in either language, and sorting by name. It mirrors upstream's
// getMovieGenres and getTvGenres.
func lookupFetchGenres(ctx context.Context, client *tmdb.Client, path, language string) ([]lookupGenreItem, error) {
	var localised lookupGenreList
	if err := client.Get(ctx, path, map[string]string{"language": language}, &localised); err != nil {
		return nil, err
	}
	byID := make(map[int]string, len(localised.Genres))
	needsEnglish := false
	for _, genre := range localised.Genres {
		name := ""
		if genre.Name != nil {
			name = *genre.Name
		}
		if name == "" {
			needsEnglish = true
		}
		byID[genre.ID] = name
	}
	if needsEnglish && !strings.HasPrefix(language, "en") {
		var english lookupGenreList
		if err := client.Get(ctx, path, map[string]string{"language": "en"}, &english); err != nil {
			return nil, err
		}
		englishByID := make(map[int]string, len(english.Genres))
		for _, genre := range english.Genres {
			if genre.Name != nil {
				englishByID[genre.ID] = *genre.Name
			}
		}
		for id, name := range byID {
			if name == "" {
				byID[id] = englishByID[id]
			}
		}
	}
	out := make([]lookupGenreItem, 0, len(byID))
	for _, genre := range localised.Genres {
		if name := byID[genre.ID]; name != "" {
			out = append(out, lookupGenreItem{ID: genre.ID, Name: name})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Service) handleLookupMovieGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := lookupFetchGenres(r.Context(), s.tmdb, "/genre/movie/list", lookupRequestLanguage(r))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve movie genres.")
		return
	}
	if genres == nil {
		genres = []lookupGenreItem{}
	}
	respond.JSON(w, genres)
}

func (s *Service) handleLookupTVGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := lookupFetchGenres(r.Context(), s.tmdb, "/genre/tv/list", lookupRequestLanguage(r))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve series genres.")
		return
	}
	if genres == nil {
		genres = []lookupGenreItem{}
	}
	respond.JSON(w, genres)
}

func (s *Service) handleLookupKeyword(w http.ResponseWriter, r *http.Request) {
	id, ok := lookupParsePositiveID(r.PathValue("id"))
	if !ok {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "id must be a positive integer")
		return
	}
	var detail *tmdb.KeywordDetails
	if err := s.tmdb.Get(r.Context(), "/keyword/"+id, nil, &detail); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve keyword data.")
		return
	}
	// A missing keyword is upstream's intentional null: status 200 with the
	// JSON body null, and the TMDB client caches that negative.
	respond.JSON(w, detail)
}

// lookupMapProductionCompany maps a TMDB company to Seerr's studio shape.
// The two structs share field names and types; only the JSON tags differ, so
// a conversion keeps every value while emitting the response shape.
func lookupMapProductionCompany(company lookupStudioTMDB) lookupStudioResponse {
	return lookupStudioResponse(company)
}

func (s *Service) handleLookupStudio(w http.ResponseWriter, r *http.Request) {
	id, ok := lookupParsePositiveID(r.PathValue("id"))
	if !ok {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "id must be a positive integer")
		return
	}
	var company lookupStudioTMDB
	if err := s.tmdb.Get(r.Context(), "/company/"+id, nil, &company); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve studio.")
		return
	}
	respond.JSON(w, lookupMapProductionCompany(company))
}

// lookupMapNetwork maps a TMDB network to Seerr's network shape. The structs
// share field names and types; only the JSON tags differ.
func lookupMapNetwork(network lookupNetworkTMDB) lookupNetworkResponse {
	return lookupNetworkResponse(network)
}

func (s *Service) handleLookupNetwork(w http.ResponseWriter, r *http.Request) {
	id, ok := lookupParsePositiveID(r.PathValue("id"))
	if !ok {
		respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "id must be a positive integer")
		return
	}
	var network lookupNetworkTMDB
	if err := s.tmdb.Get(r.Context(), "/network/"+id, nil, &network); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve network.")
		return
	}
	respond.JSON(w, lookupMapNetwork(network))
}

// lookupMapWatchProviderDetails maps TMDB providers to Seerr's shape.
func lookupMapWatchProviderDetails(providers []lookupWatchProviderTMDB) []lookupWatchProvider {
	out := make([]lookupWatchProvider, 0, len(providers))
	for _, provider := range providers {
		out = append(out, lookupWatchProvider{
			DisplayPriority: provider.DisplayPriority,
			LogoPath:        provider.LogoPath,
			ID:              provider.ProviderID,
			Name:            provider.ProviderName,
		})
	}
	return out
}

// lookupWatchRegion is the region the provider lists are asked for: an
// explicit ?watchRegion wins, otherwise the project default.
func lookupWatchRegion(r *http.Request) string {
	if region := r.URL.Query().Get("watchRegion"); region != "" {
		return region
	}
	return tmdb.DefaultRegion
}

func (s *Service) handleLookupWatchRegions(w http.ResponseWriter, r *http.Request) {
	var regions lookupWatchRegionsTMDB
	// Upstream sends language "" (its empty originalLanguage), so the request
	// matches the recorded fixture exactly.
	if err := s.tmdb.Get(r.Context(), "/watch/providers/regions", map[string]string{"language": ""}, &regions); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve watch provider regions.")
		return
	}
	out := regions.Results
	if out == nil {
		out = []tmdb.WatchProviderRegion{}
	}
	respond.JSON(w, out)
}

func (s *Service) handleLookupMovieProviders(w http.ResponseWriter, r *http.Request) {
	var providers lookupWatchProvidersTMDB
	params := map[string]string{"language": "", "watch_region": lookupWatchRegion(r)}
	if err := s.tmdb.Get(r.Context(), "/watch/providers/movie", params, &providers); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve movie watch providers.")
		return
	}
	respond.JSON(w, lookupMapWatchProviderDetails(providers.Results))
}

func (s *Service) handleLookupTVProviders(w http.ResponseWriter, r *http.Request) {
	var providers lookupWatchProvidersTMDB
	params := map[string]string{"language": "", "watch_region": lookupWatchRegion(r)}
	if err := s.tmdb.Get(r.Context(), "/watch/providers/tv", params, &providers); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve tv watch providers.")
		return
	}
	respond.JSON(w, lookupMapWatchProviderDetails(providers.Results))
}

// lookupSortLanguages orders a list by english_name ascending, as upstream
// does for languages and regions. The sort is stable so ties keep TMDB's
// order, matching lodash sortBy.
func lookupSortLanguages(languages []tmdb.Language) {
	sort.SliceStable(languages, func(i, j int) bool { return languages[i].English < languages[j].English })
}

// lookupSortCountries orders a list by english_name ascending, stable on ties.
func lookupSortCountries(countries []tmdb.Country) {
	sort.SliceStable(countries, func(i, j int) bool { return countries[i].English < countries[j].English })
}

func (s *Service) handleLookupLanguages(w http.ResponseWriter, r *http.Request) {
	var languages []tmdb.Language
	if err := s.tmdb.Get(r.Context(), "/configuration/languages", nil, &languages); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve languages.")
		return
	}
	if languages == nil {
		languages = []tmdb.Language{}
	}
	lookupSortLanguages(languages)
	respond.JSON(w, languages)
}

func (s *Service) handleLookupRegions(w http.ResponseWriter, r *http.Request) {
	var countries []tmdb.Country
	if err := s.tmdb.Get(r.Context(), "/configuration/countries", nil, &countries); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve regions.")
		return
	}
	if countries == nil {
		countries = []tmdb.Country{}
	}
	lookupSortCountries(countries)
	respond.JSON(w, countries)
}

func (s *Service) handleLookupMovieCertifications(w http.ResponseWriter, r *http.Request) {
	var certifications tmdb.CertificationList
	if err := s.tmdb.Get(r.Context(), "/certification/movie/list", nil, &certifications); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve movie certifications.")
		return
	}
	respond.JSON(w, certifications)
}

func (s *Service) handleLookupTVCertifications(w http.ResponseWriter, r *http.Request) {
	var certifications tmdb.CertificationList
	if err := s.tmdb.Get(r.Context(), "/certification/tv/list", nil, &certifications); err != nil {
		respond.Error(w, http.StatusInternalServerError, "Unable to retrieve TV certifications.")
		return
	}
	respond.JSON(w, certifications)
}
