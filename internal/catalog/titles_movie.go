package catalog

import (
	"encoding/json"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The movie page: upstream's mapMovieDetails (server/models/Movie.ts), and
// the TMDB answer it reads. A field TMDB sends as null stays null, so the
// nullable ones carry no omitempty. releases, productionCountries and
// spokenLanguages pass TMDB's snake_case objects through untouched, as
// upstream does, because the kept UI reads them that way.

type titleMovieTMDB struct {
	ID                  int                     `json:"id"`
	Adult               bool                    `json:"adult"`
	Budget              int64                   `json:"budget"`
	Genres              []tmdb.Genre            `json:"genres"`
	Videos              *tmdb.VideoResult       `json:"videos"`
	OriginalLanguage    string                  `json:"original_language"`
	OriginalTitle       string                  `json:"original_title"`
	Popularity          float64                 `json:"popularity"`
	ProductionCompanies []titleCompanyTMDB      `json:"production_companies"`
	ProductionCountries json.RawMessage         `json:"production_countries"`
	ReleaseDate         string                  `json:"release_date"`
	ReleaseDates        json.RawMessage         `json:"release_dates"`
	Revenue             int64                   `json:"revenue"`
	SpokenLanguages     json.RawMessage         `json:"spoken_languages"`
	Status              string                  `json:"status"`
	Title               string                  `json:"title"`
	Video               bool                    `json:"video"`
	VoteAverage         float64                 `json:"vote_average"`
	VoteCount           int                     `json:"vote_count"`
	BackdropPath        *string                 `json:"backdrop_path"`
	Homepage            string                  `json:"homepage"`
	IMDbID              *string                 `json:"imdb_id"`
	Overview            string                  `json:"overview"`
	PosterPath          *string                 `json:"poster_path"`
	Runtime             *int                    `json:"runtime"`
	Tagline             string                  `json:"tagline"`
	Credits             titleMovieCreditsTMDB   `json:"credits"`
	Collection          *titleCollectionRefTMDB `json:"belongs_to_collection"`
	ExternalIDs         tmdb.ExternalIDs        `json:"external_ids"`
	WatchProviders      titleWatchProvidersTMDB `json:"watch/providers"`
	Keywords            struct {
		Keywords []titleKeyword `json:"keywords"`
	} `json:"keywords"`
}

type titleMovieCreditsTMDB struct {
	Cast []tmdb.TitleCast `json:"cast"`
	Crew []tmdb.TitleCrew `json:"crew"`
}

type titleCollectionRefTMDB struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
}

// titleCompanyTMDB is a production company or a network as a details answer
// embeds it.
type titleCompanyTMDB struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry string  `json:"origin_country"`
	LogoPath      *string `json:"logo_path"`
}

type titleWatchProvidersTMDB struct {
	Results map[string]tmdb.ProviderOffer `json:"results"`
}

// titleVideosTMDB is the narrow answer of the English video request.
type titleVideosTMDB struct {
	Videos *tmdb.VideoResult `json:"videos"`
}

// titleKeyword is a TMDB keyword, whose shape upstream keeps as it is.
type titleKeyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// titleProductionCompany is upstream's ProductionCompany as the details
// mappers fill it: TMDB's details answers carry no description,
// headquarters or homepage, so those never appear.
type titleProductionCompany struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	OriginCountry string  `json:"originCountry"`
	LogoPath      *string `json:"logoPath"`
}

type titleCredits struct {
	Cast []Cast `json:"cast"`
	Crew []Crew `json:"crew"`
}

type titleMovieCollection struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	PosterPath   *string `json:"posterPath"`
	BackdropPath *string `json:"backdropPath"`
}

// titleMovieDetails is upstream's MovieDetails without mediaInfo, which this
// app has no requests to fill. relatedVideos is absent when TMDB sent no
// videos block, as upstream's optional chaining leaves it.
type titleMovieDetails struct {
	ID                  int                      `json:"id"`
	Adult               bool                     `json:"adult"`
	Budget              int64                    `json:"budget"`
	Genres              []tmdb.Genre             `json:"genres"`
	RelatedVideos       []Video                  `json:"relatedVideos,omitzero"`
	OriginalLanguage    string                   `json:"originalLanguage"`
	OriginalTitle       string                   `json:"originalTitle"`
	Popularity          float64                  `json:"popularity"`
	ProductionCompanies []titleProductionCompany `json:"productionCompanies"`
	ProductionCountries json.RawMessage          `json:"productionCountries"`
	ReleaseDate         string                   `json:"releaseDate"`
	Releases            json.RawMessage          `json:"releases"`
	Revenue             int64                    `json:"revenue"`
	SpokenLanguages     json.RawMessage          `json:"spokenLanguages"`
	Status              string                   `json:"status"`
	Title               string                   `json:"title"`
	Video               bool                     `json:"video"`
	VoteAverage         float64                  `json:"voteAverage"`
	VoteCount           int                      `json:"voteCount"`
	BackdropPath        *string                  `json:"backdropPath"`
	Homepage            string                   `json:"homepage"`
	IMDbID              *string                  `json:"imdbId"`
	Overview            string                   `json:"overview"`
	PosterPath          *string                  `json:"posterPath"`
	Runtime             *int                     `json:"runtime"`
	Tagline             string                   `json:"tagline"`
	Credits             titleCredits             `json:"credits"`
	Collection          *titleMovieCollection    `json:"collection,omitempty"`
	ExternalIds         ExternalIds              `json:"externalIds"`
	WatchProviders      []WatchProviders         `json:"watchProviders"`
	Keywords            []titleKeyword           `json:"keywords"`
	OnUserWatchlist     bool                     `json:"onUserWatchlist"`
}

func mapTitleMovieDetails(movie titleMovieTMDB) titleMovieDetails {
	return titleMovieDetails{
		ID:                  movie.ID,
		Adult:               movie.Adult,
		Budget:              movie.Budget,
		Genres:              nonNullResultArray(movie.Genres),
		RelatedVideos:       MapVideos(movie.Videos),
		OriginalLanguage:    movie.OriginalLanguage,
		OriginalTitle:       movie.OriginalTitle,
		Popularity:          movie.Popularity,
		ProductionCompanies: mapTitleProductionCompanies(movie.ProductionCompanies),
		ProductionCountries: titleRawOr(movie.ProductionCountries, `[]`),
		ReleaseDate:         movie.ReleaseDate,
		Releases:            titleRawOr(movie.ReleaseDates, `{"results":[]}`),
		Revenue:             movie.Revenue,
		SpokenLanguages:     titleRawOr(movie.SpokenLanguages, `[]`),
		Status:              movie.Status,
		Title:               movie.Title,
		Video:               movie.Video,
		VoteAverage:         movie.VoteAverage,
		VoteCount:           movie.VoteCount,
		BackdropPath:        movie.BackdropPath,
		Homepage:            movie.Homepage,
		IMDbID:              movie.IMDbID,
		Overview:            movie.Overview,
		PosterPath:          movie.PosterPath,
		Runtime:             movie.Runtime,
		Tagline:             movie.Tagline,
		Credits:             titleCredits{Cast: mapTitleCast(movie.Credits.Cast), Crew: mapTitleCrew(movie.Credits.Crew)},
		Collection:          mapTitleMovieCollection(movie.Collection),
		ExternalIds:         MapExternalIds(movie.ExternalIDs),
		WatchProviders:      MapWatchProviders(movie.WatchProviders.Results),
		Keywords:            nonNullResultArray(movie.Keywords.Keywords),
		// Watchlist state belongs to the library; the route sets it.
		OnUserWatchlist: false,
	}
}

func mapTitleMovieCollection(collection *titleCollectionRefTMDB) *titleMovieCollection {
	if collection == nil {
		return nil
	}
	return &titleMovieCollection{
		ID:           collection.ID,
		Name:         collection.Name,
		PosterPath:   collection.PosterPath,
		BackdropPath: collection.BackdropPath,
	}
}

func mapTitleProductionCompanies(companies []titleCompanyTMDB) []titleProductionCompany {
	mapped := make([]titleProductionCompany, 0, len(companies))
	for _, company := range companies {
		mapped = append(mapped, titleProductionCompany(company))
	}
	return mapped
}

func mapTitleCast(cast []tmdb.TitleCast) []Cast {
	mapped := make([]Cast, 0, len(cast))
	for _, person := range cast {
		mapped = append(mapped, MapCast(person))
	}
	return mapped
}

func mapTitleCrew(crew []tmdb.TitleCrew) []Crew {
	mapped := make([]Crew, 0, len(crew))
	for _, person := range crew {
		mapped = append(mapped, MapCrew(person))
	}
	return mapped
}

// titleRawOr passes a TMDB value through, or fallback when TMDB sent none,
// so a renderer that iterates the field never meets null.
func titleRawOr(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(fallback)
	}
	return raw
}

// titleHasTrailer is upstream's test for asking TMDB again for English
// videos: the localised answer holds no video of type Trailer.
func titleHasTrailer(videos *tmdb.VideoResult) bool {
	if videos == nil {
		return false
	}
	for _, video := range videos.Results {
		if video.Type == "Trailer" {
			return true
		}
	}
	return false
}

// titleMergeEnglishTrailers appends the English trailers whose keys the
// localised videos do not already hold. Nothing else of the English answer
// is kept, and with no new trailer the localised block is left as it was,
// absent included.
func titleMergeEnglishTrailers(localised, english *tmdb.VideoResult) *tmdb.VideoResult {
	if english == nil {
		return localised
	}
	var kept []tmdb.Video
	if localised != nil {
		kept = localised.Results
	}
	seen := make(map[string]bool, len(kept))
	for _, video := range kept {
		seen[video.Key] = true
	}
	merged := append([]tmdb.Video{}, kept...)
	for _, video := range english.Results {
		if video.Type != "Trailer" || seen[video.Key] {
			continue
		}
		merged = append(merged, video)
	}
	if len(merged) == len(kept) {
		return localised
	}
	return &tmdb.VideoResult{Results: merged}
}
