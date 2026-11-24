package catalog

import "github.com/gabrielassisxyz/two-tickets/internal/tmdb"

// The result shapes and mappers of Seerr's server/models/Search.ts at the
// pinned commit. Field names, nullability and presence follow what upstream
// actually serialises, which the goldens under testdata/golden record:
// artwork paths TMDB sends as null stay null, so they carry no omitempty.
// No mapper attaches upstream's mediaInfo; this app has no requests.

// SearchResult is one of MovieResult, TvResult, PersonResult or
// CollectionResult, upstream's Results union.
type SearchResult interface {
	isAdult() bool
}

type MovieResult struct {
	ID               int     `json:"id"`
	MediaType        string  `json:"mediaType"`
	Adult            bool    `json:"adult"`
	GenreIDs         []int   `json:"genreIds"`
	OriginalLanguage string  `json:"originalLanguage"`
	OriginalTitle    string  `json:"originalTitle"`
	Overview         string  `json:"overview"`
	Popularity       float64 `json:"popularity"`
	ReleaseDate      string  `json:"releaseDate"`
	Title            string  `json:"title"`
	Video            bool    `json:"video"`
	VoteAverage      float64 `json:"voteAverage"`
	VoteCount        int     `json:"voteCount"`
	BackdropPath     *string `json:"backdropPath"`
	PosterPath       *string `json:"posterPath"`
}

// TvResult has no adult field upstream; adult remembers TMDB's flag so the
// adult filter treats series like every other kind.
type TvResult struct {
	ID               int      `json:"id"`
	FirstAirDate     string   `json:"firstAirDate"`
	GenreIDs         []int    `json:"genreIds"`
	MediaType        string   `json:"mediaType"`
	Name             string   `json:"name"`
	OriginCountry    []string `json:"originCountry"`
	OriginalLanguage string   `json:"originalLanguage"`
	OriginalName     string   `json:"originalName"`
	Overview         string   `json:"overview"`
	Popularity       float64  `json:"popularity"`
	VoteAverage      float64  `json:"voteAverage"`
	VoteCount        int      `json:"voteCount"`
	BackdropPath     *string  `json:"backdropPath"`
	PosterPath       *string  `json:"posterPath"`
	adult            bool
}

// PersonResult.KnownFor holds only MovieResult and TvResult values.
type PersonResult struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Popularity  float64        `json:"popularity"`
	Adult       bool           `json:"adult"`
	MediaType   string         `json:"mediaType"`
	ProfilePath *string        `json:"profilePath"`
	KnownFor    []SearchResult `json:"knownFor"`
}

type CollectionResult struct {
	ID               int     `json:"id"`
	MediaType        string  `json:"mediaType"`
	Adult            bool    `json:"adult"`
	OriginalLanguage string  `json:"originalLanguage"`
	OriginalTitle    string  `json:"originalTitle"`
	Title            string  `json:"title"`
	Overview         string  `json:"overview"`
	BackdropPath     *string `json:"backdropPath"`
	PosterPath       *string `json:"posterPath"`
}

func (r MovieResult) isAdult() bool      { return r.Adult }
func (r TvResult) isAdult() bool         { return r.adult }
func (r PersonResult) isAdult() bool     { return r.Adult }
func (r CollectionResult) isAdult() bool { return r.Adult }

func MapMovieResult(movie tmdb.MovieBrief) MovieResult {
	return MovieResult{
		ID:               movie.ID,
		MediaType:        "movie",
		Adult:            movie.Adult,
		GenreIDs:         nonNullResultArray(movie.GenreIDs),
		OriginalLanguage: movie.OriginalLanguage,
		OriginalTitle:    movie.OriginalTitle,
		Overview:         movie.Overview,
		Popularity:       movie.Popularity,
		ReleaseDate:      movie.ReleaseDate,
		Title:            movie.Title,
		Video:            movie.Video,
		VoteAverage:      movie.VoteAverage,
		VoteCount:        movie.VoteCount,
		BackdropPath:     movie.BackdropPath,
		PosterPath:       movie.PosterPath,
	}
}

// MapTvResult forces mediaType to "tv" when TMDB leaves it out, as discover
// and similar do, exactly as upstream does.
func MapTvResult(show tmdb.TVBrief) TvResult {
	mediaType := show.MediaType
	if mediaType == "" {
		mediaType = "tv"
	}
	return TvResult{
		ID:               show.ID,
		FirstAirDate:     show.FirstAirDate,
		GenreIDs:         nonNullResultArray(show.GenreIDs),
		MediaType:        mediaType,
		Name:             show.Name,
		OriginCountry:    nonNullResultArray(show.OriginCountry),
		OriginalLanguage: show.OriginalLanguage,
		OriginalName:     show.OriginalName,
		Overview:         show.Overview,
		Popularity:       show.Popularity,
		VoteAverage:      show.VoteAverage,
		VoteCount:        show.VoteCount,
		BackdropPath:     show.BackdropPath,
		PosterPath:       show.PosterPath,
		adult:            show.Adult,
	}
}

func MapCollectionResult(collection tmdb.CollectionBrief) CollectionResult {
	return CollectionResult{
		ID:               collection.ID,
		MediaType:        "collection",
		Adult:            collection.Adult,
		OriginalLanguage: collection.OriginalLanguage,
		OriginalTitle:    collection.OriginalTitle,
		Title:            collection.Title,
		Overview:         collection.Overview,
		BackdropPath:     collection.BackdropPath,
		PosterPath:       collection.PosterPath,
	}
}

// MapPersonResult maps known_for the way upstream does, movies as movies and
// series as series; a row of any other kind there is dropped.
func MapPersonResult(person tmdb.PersonBrief) PersonResult {
	knownFor := make([]SearchResult, 0, len(person.KnownFor))
	for _, item := range person.KnownFor {
		switch {
		case item.Movie != nil:
			knownFor = append(knownFor, MapMovieResult(*item.Movie))
		case item.TV != nil:
			knownFor = append(knownFor, MapTvResult(*item.TV))
		}
	}
	return PersonResult{
		ID:          person.ID,
		Name:        person.Name,
		Popularity:  person.Popularity,
		Adult:       person.Adult,
		MediaType:   "person",
		ProfilePath: person.ProfilePath,
		KnownFor:    knownFor,
	}
}

// MapSearchResults maps a mixed list by its media_type. Upstream's trending
// route instead tests for a title first and so types every collection as a
// movie; going by media_type is the deliberate fix. A row of a kind none of
// the four mappers handles is dropped, where upstream would map it as a
// person.
func MapSearchResults(items []tmdb.MediaItem) []SearchResult {
	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		switch {
		case item.Movie != nil:
			results = append(results, MapMovieResult(*item.Movie))
		case item.TV != nil:
			results = append(results, MapTvResult(*item.TV))
		case item.Collection != nil:
			results = append(results, MapCollectionResult(*item.Collection))
		case item.Person != nil:
			results = append(results, MapPersonResult(*item.Person))
		}
	}
	return results
}

// MapMovieDetailsToResult turns a details answer into the row a search
// list holds, as the tmdb: search prefix does with each probe it makes.
func MapMovieDetailsToResult(details tmdb.MovieDetails) tmdb.MediaItem {
	movie := details.MovieBrief
	movie.GenreIDs = detailsGenreIDs(details.Genres)
	return tmdb.MediaItem{MediaType: "movie", Movie: &movie}
}

func MapTvDetailsToResult(details tmdb.TVDetails) tmdb.MediaItem {
	show := details.TVBrief
	show.MediaType = "tv"
	show.GenreIDs = detailsGenreIDs(details.Genres)
	return tmdb.MediaItem{MediaType: "tv", TV: &show}
}

func MapPersonDetailsToResult(details tmdb.PersonDetails) tmdb.MediaItem {
	person := tmdb.PersonBrief{
		ID:          details.ID,
		Name:        details.Name,
		Popularity:  details.Popularity,
		ProfilePath: details.ProfilePath,
		Adult:       details.Adult,
		KnownFor:    []tmdb.MediaItem{},
	}
	return tmdb.MediaItem{MediaType: "person", Person: &person}
}

// WithoutAdult is the one adult policy for result lists: it drops every
// adult entry, and every adult title from a person's knownFor. Route
// handlers apply it after mapping, to whichever result type they hold.
func WithoutAdult[T SearchResult](results []T) []T {
	kept := make([]T, 0, len(results))
	for _, result := range results {
		if result.isAdult() {
			continue
		}
		if person, ok := any(result).(PersonResult); ok {
			person.KnownFor = WithoutAdult(person.KnownFor)
			result = any(person).(T)
		}
		kept = append(kept, result)
	}
	return kept
}

func detailsGenreIDs(genres []tmdb.Genre) []int {
	ids := make([]int, 0, len(genres))
	for _, genre := range genres {
		ids = append(ids, genre.ID)
	}
	return ids
}

// nonNullResultArray keeps an empty TMDB list an empty JSON array rather than null.
func nonNullResultArray[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
