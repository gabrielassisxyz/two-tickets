package tmdb

import (
	"context"
	"fmt"
	"net/url"
)

// Thin wrappers over Get. The catalog beads add only what one route family
// needs; this file stays a list of paths and parameter shapes so the core in
// client.go keeps the limiter, retry and cache.

func moviePath(id string) string { return "/movie/" + url.PathEscape(id) }
func tvPath(id string) string    { return "/tv/" + url.PathEscape(id) }

func paramsWithPage(params map[string]string, page int) map[string]string {
	merged := make(map[string]string, len(params)+1)
	for k, v := range params {
		merged[k] = v
	}
	if page > 0 {
		merged["page"] = fmt.Sprintf("%d", page)
	}
	return merged
}

// GetDiscoverMovies mirrors Seerr's getDiscoverMovies.
func (c *Client) GetDiscoverMovies(ctx context.Context, params map[string]string) (*DiscoverMoviesResponse, error) {
	var out DiscoverMoviesResponse
	if err := c.Get(ctx, "/discover/movie", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDiscoverTV mirrors Seerr's getDiscoverTv.
func (c *Client) GetDiscoverTV(ctx context.Context, params map[string]string) (*DiscoverTVResponse, error) {
	var out DiscoverTVResponse
	if err := c.Get(ctx, "/discover/tv", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAllTrending mirrors Seerr's getAllTrending.
func (c *Client) GetAllTrending(ctx context.Context, timeWindow string, params map[string]string) (*TrendingResponse, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	var out TrendingResponse
	if err := c.Get(ctx, "/trending/all/"+url.PathEscape(timeWindow), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMovieTrending mirrors Seerr's getMovieTrending.
func (c *Client) GetMovieTrending(ctx context.Context, timeWindow string, params map[string]string) (*TrendingResponse, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	var out TrendingResponse
	if err := c.Get(ctx, "/trending/movie/"+url.PathEscape(timeWindow), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVTrending mirrors Seerr's getTvTrending.
func (c *Client) GetTVTrending(ctx context.Context, timeWindow string, params map[string]string) (*TrendingResponse, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	var out TrendingResponse
	if err := c.Get(ctx, "/trending/tv/"+url.PathEscape(timeWindow), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMoviesByKeyword mirrors Seerr's getMoviesByKeyword.
func (c *Client) GetMoviesByKeyword(ctx context.Context, keywordID string, params map[string]string) (*KeywordMoviesResponse, error) {
	var out KeywordMoviesResponse
	if err := c.Get(ctx, "/keyword/"+url.PathEscape(keywordID)+"/movies", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetKeywordDetails mirrors Seerr's getKeywordDetails. A missing keyword is
// the intentional negative: upstream answers 404, the client caches JSON null
// and returns a nil detail with no error.
func (c *Client) GetKeywordDetails(ctx context.Context, keywordID string) (*KeywordDetails, error) {
	var out *KeywordDetails
	if err := c.Get(ctx, "/keyword/"+url.PathEscape(keywordID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetLanguages mirrors Seerr's getLanguages.
func (c *Client) GetLanguages(ctx context.Context) ([]Language, error) {
	var out []Language
	if err := c.Get(ctx, "/configuration/languages", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetMovieGenres mirrors Seerr's getMovieGenres.
func (c *Client) GetMovieGenres(ctx context.Context) (*GenreList, error) {
	var out GenreList
	if err := c.Get(ctx, "/genre/movie/list", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVGenres mirrors Seerr's getTvGenres.
func (c *Client) GetTVGenres(ctx context.Context) (*GenreList, error) {
	var out GenreList
	if err := c.Get(ctx, "/genre/tv/list", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStudio mirrors Seerr's getStudio. TMDB models studios as companies.
func (c *Client) GetStudio(ctx context.Context, id string) (*CompanyDetails, error) {
	var out CompanyDetails
	if err := c.Get(ctx, "/company/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNetwork mirrors Seerr's getNetwork.
func (c *Client) GetNetwork(ctx context.Context, id string) (*NetworkDetails, error) {
	var out NetworkDetails
	if err := c.Get(ctx, "/network/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMovie mirrors Seerr's getMovie.
func (c *Client) GetMovie(ctx context.Context, id string, params map[string]string) (*MovieDetails, error) {
	var out MovieDetails
	if err := c.Get(ctx, moviePath(id), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVShow mirrors Seerr's getTvShow.
func (c *Client) GetTVShow(ctx context.Context, id string, params map[string]string) (*TVDetails, error) {
	var out TVDetails
	if err := c.Get(ctx, tvPath(id), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVSeason mirrors Seerr's getTvSeason.
func (c *Client) GetTVSeason(ctx context.Context, tvID string, seasonNumber int, params map[string]string) (*SeasonDetails, error) {
	var out SeasonDetails
	path := tvPath(tvID) + "/season/" + fmt.Sprintf("%d", seasonNumber)
	if err := c.Get(ctx, path, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMovieRecommendations mirrors Seerr's getMovieRecommendations.
func (c *Client) GetMovieRecommendations(ctx context.Context, id string, params map[string]string) (*MovieRecommendationsResponse, error) {
	var out MovieRecommendationsResponse
	if err := c.Get(ctx, moviePath(id)+"/recommendations", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMovieSimilar mirrors Seerr's getMovieSimilar.
func (c *Client) GetMovieSimilar(ctx context.Context, id string, params map[string]string) (*MovieSimilarResponse, error) {
	var out MovieSimilarResponse
	if err := c.Get(ctx, moviePath(id)+"/similar", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVRecommendations mirrors Seerr's getTvRecommendations.
func (c *Client) GetTVRecommendations(ctx context.Context, id string, params map[string]string) (*TVRecommendationsResponse, error) {
	var out TVRecommendationsResponse
	if err := c.Get(ctx, tvPath(id)+"/recommendations", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVSimilar mirrors Seerr's getTvSimilar.
func (c *Client) GetTVSimilar(ctx context.Context, id string, params map[string]string) (*TVSimilarResponse, error) {
	var out TVSimilarResponse
	if err := c.Get(ctx, tvPath(id)+"/similar", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCollection mirrors Seerr's getCollection.
func (c *Client) GetCollection(ctx context.Context, id string, params map[string]string) (*CollectionDetails, error) {
	var out CollectionDetails
	if err := c.Get(ctx, "/collection/"+url.PathEscape(id), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPerson mirrors Seerr's getPerson.
func (c *Client) GetPerson(ctx context.Context, id string, params map[string]string) (*PersonDetails, error) {
	var out PersonDetails
	if err := c.Get(ctx, "/person/"+url.PathEscape(id), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPersonCombinedCredits mirrors Seerr's getPersonCombinedCredits.
func (c *Client) GetPersonCombinedCredits(ctx context.Context, id string, params map[string]string) (*PersonCombinedCredits, error) {
	var out PersonCombinedCredits
	if err := c.Get(ctx, "/person/"+url.PathEscape(id)+"/combined_credits", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchMulti mirrors Seerr's searchMulti.
func (c *Client) SearchMulti(ctx context.Context, query string, params map[string]string) (*SearchMultiResponse, error) {
	merged := paramsWithPage(params, 0)
	merged["query"] = query
	var out SearchMultiResponse
	if err := c.Get(ctx, "/search/multi", merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchMovies mirrors Seerr's searchMovies.
func (c *Client) SearchMovies(ctx context.Context, query string, params map[string]string) (*SearchMoviesResponse, error) {
	merged := paramsWithPage(params, 0)
	merged["query"] = query
	var out SearchMoviesResponse
	if err := c.Get(ctx, "/search/movie", merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchTVShows mirrors Seerr's searchTvShows.
func (c *Client) SearchTVShows(ctx context.Context, query string, params map[string]string) (*SearchTVResponse, error) {
	merged := paramsWithPage(params, 0)
	merged["query"] = query
	var out SearchTVResponse
	if err := c.Get(ctx, "/search/tv", merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetByExternalID mirrors Seerr's getByExternalId.
func (c *Client) GetByExternalID(ctx context.Context, externalID, source string, params map[string]string) (*ExternalIDResult, error) {
	merged := make(map[string]string, len(params)+1)
	for k, v := range params {
		merged[k] = v
	}
	merged["external_source"] = source
	var out ExternalIDResult
	if err := c.Get(ctx, "/find/"+url.PathEscape(externalID), merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchKeyword mirrors Seerr's searchKeyword.
func (c *Client) SearchKeyword(ctx context.Context, query string, params map[string]string) (*SearchKeywordResponse, error) {
	merged := paramsWithPage(params, 0)
	merged["query"] = query
	var out SearchKeywordResponse
	if err := c.Get(ctx, "/search/keyword", merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchCompany mirrors Seerr's searchCompany.
func (c *Client) SearchCompany(ctx context.Context, query string, params map[string]string) (*SearchCompanyResponse, error) {
	merged := paramsWithPage(params, 0)
	merged["query"] = query
	var out SearchCompanyResponse
	if err := c.Get(ctx, "/search/company", merged, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAvailableWatchProviderRegions mirrors Seerr's getAvailableWatchProviderRegions.
func (c *Client) GetAvailableWatchProviderRegions(ctx context.Context) (*WatchProviderRegions, error) {
	var out WatchProviderRegions
	if err := c.Get(ctx, "/watch/providers/regions", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMovieWatchProviders mirrors Seerr's getMovieWatchProviders.
func (c *Client) GetMovieWatchProviders(ctx context.Context, id string) (*WatchProvidersResponse, error) {
	var out WatchProvidersResponse
	if err := c.Get(ctx, moviePath(id)+"/watch/providers", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVWatchProviders mirrors Seerr's getTvWatchProviders.
func (c *Client) GetTVWatchProviders(ctx context.Context, id string) (*WatchProvidersResponse, error) {
	var out WatchProvidersResponse
	if err := c.Get(ctx, tvPath(id)+"/watch/providers", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRegions mirrors Seerr's getRegions.
func (c *Client) GetRegions(ctx context.Context) ([]Country, error) {
	var out struct {
		Results []Country `json:"results"`
	}
	if err := c.Get(ctx, "/configuration/countries", nil, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// GetMovieCertifications mirrors Seerr's getMovieCertifications.
func (c *Client) GetMovieCertifications(ctx context.Context) (*CertificationList, error) {
	var out CertificationList
	if err := c.Get(ctx, "/certification/movie/list", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTVCertifications mirrors Seerr's getTvCertifications.
func (c *Client) GetTVCertifications(ctx context.Context) (*CertificationList, error) {
	var out CertificationList
	if err := c.Get(ctx, "/certification/tv/list", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
