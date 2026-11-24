package catalog

import (
	"encoding/json"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The series and season pages: upstream's mapTvDetails and
// mapSeasonWithEpisodes (server/models/Tv.ts), and the TMDB answers they
// read. contentRatings, createdBy and productionCountries pass TMDB's
// snake_case objects through untouched, as upstream does.

type titleTvTMDB struct {
	ID                  int                     `json:"id"`
	Adult               bool                    `json:"adult"`
	CreatedBy           json.RawMessage         `json:"created_by"`
	EpisodeRunTime      []int                   `json:"episode_run_time"`
	FirstAirDate        *string                 `json:"first_air_date"`
	Genres              []tmdb.Genre            `json:"genres"`
	Videos              *tmdb.VideoResult       `json:"videos"`
	Homepage            string                  `json:"homepage"`
	InProduction        bool                    `json:"in_production"`
	Languages           []string                `json:"languages"`
	LastAirDate         *string                 `json:"last_air_date"`
	Name                string                  `json:"name"`
	Networks            []titleCompanyTMDB      `json:"networks"`
	NumberOfEpisodes    int                     `json:"number_of_episodes"`
	NumberOfSeasons     int                     `json:"number_of_seasons"`
	OriginCountry       []string                `json:"origin_country"`
	OriginalLanguage    string                  `json:"original_language"`
	OriginalName        string                  `json:"original_name"`
	Tagline             string                  `json:"tagline"`
	Overview            string                  `json:"overview"`
	Popularity          float64                 `json:"popularity"`
	ProductionCompanies []titleCompanyTMDB      `json:"production_companies"`
	ProductionCountries json.RawMessage         `json:"production_countries"`
	ContentRatings      json.RawMessage         `json:"content_ratings"`
	SpokenLanguages     []titleTvLanguageTMDB   `json:"spoken_languages"`
	Seasons             []titleTvSeasonTMDB     `json:"seasons"`
	Status              string                  `json:"status"`
	Type                string                  `json:"type"`
	VoteAverage         float64                 `json:"vote_average"`
	VoteCount           int                     `json:"vote_count"`
	BackdropPath        *string                 `json:"backdrop_path"`
	LastEpisodeToAir    *titleEpisodeTMDB       `json:"last_episode_to_air"`
	NextEpisodeToAir    *titleEpisodeTMDB       `json:"next_episode_to_air"`
	PosterPath          *string                 `json:"poster_path"`
	AggregateCredits    titleTvAggregateTMDB    `json:"aggregate_credits"`
	Credits             titleMovieCreditsTMDB   `json:"credits"`
	ExternalIDs         tmdb.ExternalIDs        `json:"external_ids"`
	WatchProviders      titleWatchProvidersTMDB `json:"watch/providers"`
	Keywords            struct {
		Results []titleKeyword `json:"results"`
	} `json:"keywords"`
}

type titleTvAggregateTMDB struct {
	Cast []tmdb.AggregateCast `json:"cast"`
}

type titleTvLanguageTMDB struct {
	EnglishName string `json:"english_name"`
	ISO639_1    string `json:"iso_639_1"`
	Name        string `json:"name"`
}

type titleTvSeasonTMDB struct {
	AirDate      *string `json:"air_date"`
	EpisodeCount int     `json:"episode_count"`
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	SeasonNumber int     `json:"season_number"`
	PosterPath   *string `json:"poster_path"`
}

type titleEpisodeTMDB struct {
	ID             int     `json:"id"`
	AirDate        *string `json:"air_date"`
	EpisodeNumber  int     `json:"episode_number"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	ProductionCode string  `json:"production_code"`
	SeasonNumber   int     `json:"season_number"`
	ShowID         int     `json:"show_id"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	StillPath      *string `json:"still_path"`
}

type titleSeasonTMDB struct {
	AirDate      *string            `json:"air_date"`
	Episodes     []titleEpisodeTMDB `json:"episodes"`
	ExternalIDs  tmdb.ExternalIDs   `json:"external_ids"`
	ID           int                `json:"id"`
	Name         string             `json:"name"`
	Overview     string             `json:"overview"`
	SeasonNumber int                `json:"season_number"`
	PosterPath   *string            `json:"poster_path"`
}

type titleTvLanguage struct {
	EnglishName string `json:"englishName"`
	ISO639_1    string `json:"iso_639_1"`
	Name        string `json:"name"`
}

type titleTvSeason struct {
	AirDate      *string `json:"airDate"`
	EpisodeCount int     `json:"episodeCount"`
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	SeasonNumber int     `json:"seasonNumber"`
	PosterPath   *string `json:"posterPath"`
}

type titleEpisode struct {
	ID             int     `json:"id"`
	AirDate        *string `json:"airDate"`
	EpisodeNumber  int     `json:"episodeNumber"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	ProductionCode string  `json:"productionCode"`
	SeasonNumber   int     `json:"seasonNumber"`
	ShowID         int     `json:"showId"`
	VoteAverage    float64 `json:"voteAverage"`
	VoteCount      int     `json:"voteCount"`
	StillPath      *string `json:"stillPath"`
}

// titleTvDetails is upstream's TvDetails without mediaInfo. Cast comes from
// aggregate_credits, each actor collapsed to the first role, and crew from
// credits, as upstream reads them.
type titleTvDetails struct {
	CreatedBy           json.RawMessage          `json:"createdBy"`
	EpisodeRunTime      []int                    `json:"episodeRunTime"`
	FirstAirDate        *string                  `json:"firstAirDate"`
	Genres              []tmdb.Genre             `json:"genres"`
	RelatedVideos       []Video                  `json:"relatedVideos,omitzero"`
	Homepage            string                   `json:"homepage"`
	ID                  int                      `json:"id"`
	InProduction        bool                     `json:"inProduction"`
	Languages           []string                 `json:"languages"`
	LastAirDate         *string                  `json:"lastAirDate"`
	Name                string                   `json:"name"`
	Networks            []titleProductionCompany `json:"networks"`
	NumberOfEpisodes    int                      `json:"numberOfEpisodes"`
	NumberOfSeasons     int                      `json:"numberOfSeasons"`
	OriginCountry       []string                 `json:"originCountry"`
	OriginalLanguage    string                   `json:"originalLanguage"`
	OriginalName        string                   `json:"originalName"`
	Tagline             string                   `json:"tagline"`
	Overview            string                   `json:"overview"`
	Popularity          float64                  `json:"popularity"`
	ProductionCompanies []titleProductionCompany `json:"productionCompanies"`
	ProductionCountries json.RawMessage          `json:"productionCountries"`
	ContentRatings      json.RawMessage          `json:"contentRatings"`
	SpokenLanguages     []titleTvLanguage        `json:"spokenLanguages"`
	Seasons             []titleTvSeason          `json:"seasons"`
	Status              string                   `json:"status"`
	Type                string                   `json:"type"`
	VoteAverage         float64                  `json:"voteAverage"`
	VoteCount           int                      `json:"voteCount"`
	BackdropPath        *string                  `json:"backdropPath"`
	LastEpisodeToAir    *titleEpisode            `json:"lastEpisodeToAir,omitempty"`
	NextEpisodeToAir    *titleEpisode            `json:"nextEpisodeToAir,omitempty"`
	PosterPath          *string                  `json:"posterPath"`
	Credits             titleCredits             `json:"credits"`
	ExternalIds         ExternalIds              `json:"externalIds"`
	Keywords            []titleKeyword           `json:"keywords"`
	WatchProviders      []WatchProviders         `json:"watchProviders"`
	OnUserWatchlist     bool                     `json:"onUserWatchlist"`
}

// titleSeasonWithEpisodes is upstream's SeasonWithEpisodes.
type titleSeasonWithEpisodes struct {
	AirDate      *string        `json:"airDate"`
	Episodes     []titleEpisode `json:"episodes"`
	ExternalIds  ExternalIds    `json:"externalIds"`
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Overview     string         `json:"overview"`
	SeasonNumber int            `json:"seasonNumber"`
	PosterPath   *string        `json:"posterPath"`
}

func mapTitleTvDetails(show titleTvTMDB) titleTvDetails {
	return titleTvDetails{
		CreatedBy:           titleRawOr(show.CreatedBy, `[]`),
		EpisodeRunTime:      nonNullResultArray(show.EpisodeRunTime),
		FirstAirDate:        show.FirstAirDate,
		Genres:              nonNullResultArray(show.Genres),
		RelatedVideos:       MapVideos(show.Videos),
		Homepage:            show.Homepage,
		ID:                  show.ID,
		InProduction:        show.InProduction,
		Languages:           nonNullResultArray(show.Languages),
		LastAirDate:         show.LastAirDate,
		Name:                show.Name,
		Networks:            mapTitleProductionCompanies(show.Networks),
		NumberOfEpisodes:    show.NumberOfEpisodes,
		NumberOfSeasons:     show.NumberOfSeasons,
		OriginCountry:       nonNullResultArray(show.OriginCountry),
		OriginalLanguage:    show.OriginalLanguage,
		OriginalName:        show.OriginalName,
		Tagline:             show.Tagline,
		Overview:            show.Overview,
		Popularity:          show.Popularity,
		ProductionCompanies: mapTitleProductionCompanies(show.ProductionCompanies),
		ProductionCountries: titleRawOr(show.ProductionCountries, `[]`),
		ContentRatings:      titleRawOr(show.ContentRatings, `{"results":[]}`),
		SpokenLanguages:     mapTitleTvLanguages(show.SpokenLanguages),
		Seasons:             mapTitleTvSeasons(show.Seasons),
		Status:              show.Status,
		Type:                show.Type,
		VoteAverage:         show.VoteAverage,
		VoteCount:           show.VoteCount,
		BackdropPath:        show.BackdropPath,
		LastEpisodeToAir:    mapTitleOptionalEpisode(show.LastEpisodeToAir),
		NextEpisodeToAir:    mapTitleOptionalEpisode(show.NextEpisodeToAir),
		PosterPath:          show.PosterPath,
		Credits:             titleCredits{Cast: mapTitleAggregateCast(show.AggregateCredits.Cast), Crew: mapTitleCrew(show.Credits.Crew)},
		ExternalIds:         MapExternalIds(show.ExternalIDs),
		Keywords:            nonNullResultArray(show.Keywords.Results),
		WatchProviders:      MapWatchProviders(show.WatchProviders.Results),
		OnUserWatchlist:     false,
	}
}

func mapTitleAggregateCast(cast []tmdb.AggregateCast) []Cast {
	mapped := make([]Cast, 0, len(cast))
	for _, person := range cast {
		mapped = append(mapped, MapAggregateCast(person))
	}
	return mapped
}

func mapTitleTvLanguages(languages []titleTvLanguageTMDB) []titleTvLanguage {
	mapped := make([]titleTvLanguage, 0, len(languages))
	for _, language := range languages {
		mapped = append(mapped, titleTvLanguage(language))
	}
	return mapped
}

func mapTitleTvSeasons(seasons []titleTvSeasonTMDB) []titleTvSeason {
	mapped := make([]titleTvSeason, 0, len(seasons))
	for _, season := range seasons {
		mapped = append(mapped, titleTvSeason(season))
	}
	return mapped
}

func mapTitleOptionalEpisode(episode *titleEpisodeTMDB) *titleEpisode {
	if episode == nil {
		return nil
	}
	mapped := titleEpisode(*episode)
	return &mapped
}

// mapTitleSeasonWithEpisodes expands every episode still into a full TMDB
// image URL, because the kept season component hands stillPath straight to
// the image component. The URL is built byte for byte as upstream's
// getTvSeason builds it, doubled slash included; a null still stays null.
func mapTitleSeasonWithEpisodes(season titleSeasonTMDB) titleSeasonWithEpisodes {
	episodes := make([]titleEpisode, 0, len(season.Episodes))
	for _, episode := range season.Episodes {
		mapped := titleEpisode(episode)
		mapped.StillPath = titleExpandStillPath(episode.StillPath)
		episodes = append(episodes, mapped)
	}
	return titleSeasonWithEpisodes{
		AirDate:      season.AirDate,
		Episodes:     episodes,
		ExternalIds:  MapExternalIds(season.ExternalIDs),
		ID:           season.ID,
		Name:         season.Name,
		Overview:     season.Overview,
		SeasonNumber: season.SeasonNumber,
		PosterPath:   season.PosterPath,
	}
}

func titleExpandStillPath(still *string) *string {
	if still == nil || *still == "" {
		return still
	}
	expanded := "https://image.tmdb.org/t/p/original/" + *still
	return &expanded
}
