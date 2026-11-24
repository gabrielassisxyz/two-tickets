package catalog

import (
	"encoding/json"
	"sort"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The collection and person pages: upstream's mapCollection
// (server/models/Collection.ts) and mapPersonDetails, mapCastCredits and
// mapCrewCredits (server/models/Person.ts).

type titleCollectionTMDB struct {
	tmdb.CollectionDetails
	Parts []titleCollectionPartTMDB `json:"parts"`
}

// titleCollectionPartTMDB remembers whether TMDB sent a release date at all,
// because upstream's sort puts a part without one after every dated part,
// while an empty date sorts before them.
type titleCollectionPartTMDB struct {
	tmdb.MovieBrief
	undated bool
}

func (p *titleCollectionPartTMDB) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &p.MovieBrief); err != nil {
		return err
	}
	var probe struct {
		ReleaseDate *string `json:"release_date"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	p.undated = probe.ReleaseDate == nil
	return nil
}

type titleCollection struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Overview     string        `json:"overview"`
	PosterPath   *string       `json:"posterPath"`
	BackdropPath *string       `json:"backdropPath"`
	Parts        []MovieResult `json:"parts"`
}

// mapTitleCollection orders the parts as lodash's sortBy on release_date
// does: a stable ascending string sort, so parts with equal dates keep
// TMDB's order, and parts with no date at all go last. Adult parts are then
// dropped by the one adult policy.
func mapTitleCollection(collection titleCollectionTMDB) titleCollection {
	parts := append([]titleCollectionPartTMDB{}, collection.Parts...)
	sort.SliceStable(parts, func(i, j int) bool {
		left, right := parts[i], parts[j]
		if left.undated || right.undated {
			return !left.undated && right.undated
		}
		return left.ReleaseDate < right.ReleaseDate
	})
	mapped := make([]MovieResult, 0, len(parts))
	for _, part := range parts {
		mapped = append(mapped, MapMovieResult(part.MovieBrief))
	}
	return titleCollection{
		ID:           collection.ID,
		Name:         collection.Name,
		Overview:     collection.Overview,
		PosterPath:   collection.PosterPath,
		BackdropPath: collection.BackdropPath,
		Parts:        WithoutAdult(mapped),
	}
}

type titlePersonTMDB struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	Birthday           *string  `json:"birthday"`
	Deathday           *string  `json:"deathday"`
	KnownForDepartment string   `json:"known_for_department"`
	AlsoKnownAs        []string `json:"also_known_as"`
	Gender             int      `json:"gender"`
	Biography          string   `json:"biography"`
	Popularity         float64  `json:"popularity"`
	PlaceOfBirth       *string  `json:"place_of_birth"`
	ProfilePath        *string  `json:"profile_path"`
	Adult              bool     `json:"adult"`
	IMDbID             *string  `json:"imdb_id"`
	Homepage           *string  `json:"homepage"`
}

type titlePersonDetails struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	Birthday           *string  `json:"birthday"`
	Deathday           *string  `json:"deathday"`
	KnownForDepartment string   `json:"knownForDepartment"`
	AlsoKnownAs        []string `json:"alsoKnownAs"`
	Gender             int      `json:"gender"`
	Biography          string   `json:"biography"`
	Popularity         float64  `json:"popularity"`
	PlaceOfBirth       *string  `json:"placeOfBirth"`
	ProfilePath        *string  `json:"profilePath"`
	Adult              bool     `json:"adult"`
	IMDbID             *string  `json:"imdbId"`
	Homepage           *string  `json:"homepage"`
}

func mapTitlePersonDetails(person titlePersonTMDB) titlePersonDetails {
	mapped := titlePersonDetails(person)
	mapped.AlsoKnownAs = nonNullResultArray(person.AlsoKnownAs)
	return mapped
}

// titlePersonCreditTMDB is one combined credit. A movie credit and a series
// credit carry different fields, and upstream passes on exactly the ones
// TMDB sent, so every field remembers whether it was there.
type titlePersonCreditTMDB struct {
	ID               int                     `json:"id"`
	OriginalLanguage tmdb.Optional[string]   `json:"original_language"`
	EpisodeCount     tmdb.Optional[int]      `json:"episode_count"`
	Overview         tmdb.Optional[string]   `json:"overview"`
	OriginCountry    tmdb.Optional[[]string] `json:"origin_country"`
	OriginalName     tmdb.Optional[string]   `json:"original_name"`
	VoteCount        tmdb.Optional[int]      `json:"vote_count"`
	Name             tmdb.Optional[string]   `json:"name"`
	MediaType        tmdb.Optional[string]   `json:"media_type"`
	Popularity       tmdb.Optional[float64]  `json:"popularity"`
	CreditID         tmdb.Optional[string]   `json:"credit_id"`
	BackdropPath     tmdb.Optional[string]   `json:"backdrop_path"`
	FirstAirDate     tmdb.Optional[string]   `json:"first_air_date"`
	VoteAverage      tmdb.Optional[float64]  `json:"vote_average"`
	GenreIDs         tmdb.Optional[[]int]    `json:"genre_ids"`
	PosterPath       tmdb.Optional[string]   `json:"poster_path"`
	OriginalTitle    tmdb.Optional[string]   `json:"original_title"`
	Video            tmdb.Optional[bool]     `json:"video"`
	Title            tmdb.Optional[string]   `json:"title"`
	Adult            tmdb.Optional[bool]     `json:"adult"`
	ReleaseDate      tmdb.Optional[string]   `json:"release_date"`
	Character        tmdb.Optional[string]   `json:"character"`
	Department       tmdb.Optional[string]   `json:"department"`
	Job              tmdb.Optional[string]   `json:"job"`
}

type titleCombinedCreditsTMDB struct {
	ID   int                     `json:"id"`
	Cast []titlePersonCreditTMDB `json:"cast"`
	Crew []titlePersonCreditTMDB `json:"crew"`
}

// titlePersonCredit is upstream's PersonCredit without mediaInfo.
type titlePersonCredit struct {
	ID               int                     `json:"id"`
	OriginalLanguage tmdb.Optional[string]   `json:"originalLanguage,omitzero"`
	EpisodeCount     tmdb.Optional[int]      `json:"episodeCount,omitzero"`
	Overview         tmdb.Optional[string]   `json:"overview,omitzero"`
	OriginCountry    tmdb.Optional[[]string] `json:"originCountry,omitzero"`
	OriginalName     tmdb.Optional[string]   `json:"originalName,omitzero"`
	VoteCount        tmdb.Optional[int]      `json:"voteCount,omitzero"`
	Name             tmdb.Optional[string]   `json:"name,omitzero"`
	MediaType        tmdb.Optional[string]   `json:"mediaType,omitzero"`
	Popularity       tmdb.Optional[float64]  `json:"popularity,omitzero"`
	CreditID         tmdb.Optional[string]   `json:"creditId,omitzero"`
	BackdropPath     tmdb.Optional[string]   `json:"backdropPath,omitzero"`
	FirstAirDate     tmdb.Optional[string]   `json:"firstAirDate,omitzero"`
	VoteAverage      tmdb.Optional[float64]  `json:"voteAverage,omitzero"`
	GenreIDs         tmdb.Optional[[]int]    `json:"genreIds,omitzero"`
	PosterPath       tmdb.Optional[string]   `json:"posterPath,omitzero"`
	OriginalTitle    tmdb.Optional[string]   `json:"originalTitle,omitzero"`
	Video            tmdb.Optional[bool]     `json:"video,omitzero"`
	Title            tmdb.Optional[string]   `json:"title,omitzero"`
	Adult            tmdb.Optional[bool]     `json:"adult,omitzero"`
	ReleaseDate      tmdb.Optional[string]   `json:"releaseDate,omitzero"`
}

type titlePersonCastCredit struct {
	titlePersonCredit
	Character tmdb.Optional[string] `json:"character,omitzero"`
}

type titlePersonCrewCredit struct {
	titlePersonCredit
	Department tmdb.Optional[string] `json:"department,omitzero"`
	Job        tmdb.Optional[string] `json:"job,omitzero"`
}

type titleCombinedCredits struct {
	Cast []titlePersonCastCredit `json:"cast"`
	Crew []titlePersonCrewCredit `json:"crew"`
	ID   int                     `json:"id"`
}

// mapTitleCombinedCredits drops every adult credit, and the "Thanks"
// entries TMDB files as a character or a job, as upstream does.
func mapTitleCombinedCredits(credits titleCombinedCreditsTMDB) titleCombinedCredits {
	cast := make([]titlePersonCastCredit, 0, len(credits.Cast))
	for _, credit := range credits.Cast {
		if credit.Adult.Value || credit.Character.Value == "Thanks" {
			continue
		}
		cast = append(cast, titlePersonCastCredit{titlePersonCredit: mapTitlePersonCredit(credit), Character: credit.Character})
	}
	crew := make([]titlePersonCrewCredit, 0, len(credits.Crew))
	for _, credit := range credits.Crew {
		if credit.Adult.Value || credit.Job.Value == "Thanks" {
			continue
		}
		crew = append(crew, titlePersonCrewCredit{titlePersonCredit: mapTitlePersonCredit(credit), Department: credit.Department, Job: credit.Job})
	}
	return titleCombinedCredits{Cast: cast, Crew: crew, ID: credits.ID}
}

func mapTitlePersonCredit(credit titlePersonCreditTMDB) titlePersonCredit {
	return titlePersonCredit{
		ID:               credit.ID,
		OriginalLanguage: credit.OriginalLanguage,
		EpisodeCount:     credit.EpisodeCount,
		Overview:         credit.Overview,
		OriginCountry:    credit.OriginCountry,
		OriginalName:     credit.OriginalName,
		VoteCount:        credit.VoteCount,
		Name:             credit.Name,
		MediaType:        credit.MediaType,
		Popularity:       credit.Popularity,
		CreditID:         credit.CreditID,
		BackdropPath:     credit.BackdropPath,
		FirstAirDate:     credit.FirstAirDate,
		VoteAverage:      credit.VoteAverage,
		GenreIDs:         credit.GenreIDs,
		PosterPath:       credit.PosterPath,
		OriginalTitle:    credit.OriginalTitle,
		Video:            credit.Video,
		Title:            credit.Title,
		Adult:            credit.Adult,
		ReleaseDate:      credit.ReleaseDate,
	}
}
