package tmdb

// Typed TMDB structures. Only the fields the tracker needs are modelled;
// decoding ignores the rest, so recorded fixtures stay small.

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GenreList struct {
	Genres []Genre `json:"genres"`
}

type Language struct {
	ISO639_1 string `json:"iso_639_1"`
	English  string `json:"english_name"`
	Name     string `json:"name"`
}

type Country struct {
	ISO3166_1 string `json:"iso_3166_1"`
	English   string `json:"english_name"`
	Native    string `json:"native_name"`
}

type CompanyDetails struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	LogoPath      *string `json:"logo_path"`
	OriginCountry string  `json:"origin_country"`
}

type NetworkDetails struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	LogoPath      *string `json:"logo_path"`
	OriginCountry string  `json:"origin_country"`
}

type CollectionDetails struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
}

type MovieBrief struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	PosterPath    *string `json:"poster_path"`
	BackdropPath  *string `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	VoteAverage   float64 `json:"vote_average"`
	GenreIDs      []int   `json:"genre_ids"`
}

type MovieDetails struct {
	MovieBrief
	Genres       []Genre `json:"genres"`
	Runtime      *int    `json:"runtime"`
	Status       string  `json:"status"`
	Tagline      string  `json:"tagline"`
	Homepage     string  `json:"homepage"`
	IMDbID       string  `json:"imdb_id"`
	OriginalLang string  `json:"original_language"`
	Budget       int64   `json:"budget"`
	Revenue      int64   `json:"revenue"`
	Adult        bool    `json:"adult"`
}

type TVBrief struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Overview     string  `json:"overview"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
	GenreIDs     []int   `json:"genre_ids"`
}

type TVDetails struct {
	TVBrief
	Genres           []Genre `json:"genres"`
	Status           string  `json:"status"`
	Tagline          string  `json:"tagline"`
	Homepage         string  `json:"homepage"`
	NumberOfSeasons  int     `json:"number_of_seasons"`
	NumberOfEpisodes int     `json:"number_of_episodes"`
	EpisodeRunTime   []int   `json:"episode_run_time"`
}

type EpisodeBrief struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	StillPath     *string `json:"still_path"`
	AirDate       string  `json:"air_date"`
	VoteAverage   float64 `json:"vote_average"`
}

type SeasonDetails struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Overview     string         `json:"overview"`
	PosterPath   *string        `json:"poster_path"`
	AirDate      string         `json:"air_date"`
	SeasonNumber int            `json:"season_number"`
	Episodes     []EpisodeBrief `json:"episodes"`
}

type PersonDetails struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Biography   string  `json:"biography"`
	Birthday    *string `json:"birthday"`
	Deathday    *string `json:"deathday"`
	KnownFor    string  `json:"known_for_department"`
	ProfilePath *string `json:"profile_path"`
}

type CreditCast struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Character   string  `json:"character"`
	ProfilePath *string `json:"profile_path"`
	MediaType   string  `json:"media_type"`
}

type PersonCombinedCredits struct {
	ID   int          `json:"id"`
	Cast []CreditCast `json:"cast"`
}

type KeywordDetails struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type KeywordMoviesResponse struct {
	ID      int          `json:"id"`
	Page    int          `json:"page"`
	Results []MovieBrief `json:"results"`
}

type DiscoverMoviesResponse struct {
	Page         int          `json:"page"`
	Results      []MovieBrief `json:"results"`
	TotalPages   int          `json:"total_pages"`
	TotalResults int          `json:"total_results"`
}

type DiscoverTVResponse struct {
	Page         int       `json:"page"`
	Results      []TVBrief `json:"results"`
	TotalPages   int       `json:"total_pages"`
	TotalResults int       `json:"total_results"`
}

type TrendingResponse struct {
	Page         int         `json:"page"`
	Results      []MediaItem `json:"results"`
	TotalPages   int         `json:"total_pages"`
	TotalResults int         `json:"total_results"`
}

// MediaItem is one trending or multi-search row: a movie, show or person.
type MediaItem struct {
	ID          int     `json:"id"`
	MediaType   string  `json:"media_type"`
	Title       string  `json:"title"`
	Name        string  `json:"name"`
	Overview    string  `json:"overview"`
	PosterPath  *string `json:"poster_path"`
	VoteAverage float64 `json:"vote_average"`
}

type SearchMoviesResponse = DiscoverMoviesResponse

type SearchTVResponse = DiscoverTVResponse

type SearchMultiResponse = TrendingResponse

type KeywordSearchResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SearchKeywordResponse struct {
	Page         int                   `json:"page"`
	Results      []KeywordSearchResult `json:"results"`
	TotalPages   int                   `json:"total_pages"`
	TotalResults int                   `json:"total_results"`
}

type CompanySearchResult struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	LogoPath      *string `json:"logo_path"`
	OriginCountry string  `json:"origin_country"`
}

type SearchCompanyResponse struct {
	Page         int                   `json:"page"`
	Results      []CompanySearchResult `json:"results"`
	TotalPages   int                   `json:"total_pages"`
	TotalResults int                   `json:"total_results"`
}

type ExternalIDResult struct {
	MovieResults  []MovieBrief    `json:"movie_results"`
	TVResults     []TVBrief       `json:"tv_results"`
	PersonResults []PersonDetails `json:"person_results"`
}

type WatchProvider struct {
	ProviderID   int     `json:"provider_id"`
	ProviderName string  `json:"provider_name"`
	LogoPath     *string `json:"logo_path"`
}

type WatchProviderRegion struct {
	ISO3166_1 string `json:"iso_3166_1"`
	English   string `json:"english_name"`
	Native    string `json:"native_name"`
}

type WatchProviderRegions struct {
	Results []WatchProviderRegion `json:"results"`
}

type ProviderOffer struct {
	Link     string          `json:"link"`
	Flatrate []WatchProvider `json:"flatrate"`
	Rent     []WatchProvider `json:"rent"`
	Buy      []WatchProvider `json:"buy"`
}

type WatchProvidersResponse struct {
	ID      int                      `json:"id"`
	Results map[string]ProviderOffer `json:"results"`
}

type CertificationEntry struct {
	Certification string `json:"certification"`
	Meaning       string `json:"meaning"`
	Order         int    `json:"order"`
}

type CertificationList struct {
	Certifications map[string][]CertificationEntry `json:"certifications"`
}

type MovieRecommendationsResponse = DiscoverMoviesResponse

type MovieSimilarResponse = DiscoverMoviesResponse

type TVRecommendationsResponse = DiscoverTVResponse

type TVSimilarResponse = DiscoverTVResponse
