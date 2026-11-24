package catalog

import (
	"encoding/json"
	"sort"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The shared shapes and mappers of Seerr's server/models/common.ts at the
// pinned commit: credits, external ids, videos and watch providers, which
// the movie and series detail mappers both embed. Presence follows upstream
// as the goldens record it: castId is absent for series, whose aggregate
// credits carry none, and an external id TMDB omits stays absent while one
// it sends as null stays null.

type Cast struct {
	ID          int     `json:"id"`
	CastID      *int    `json:"castId,omitempty"`
	Character   string  `json:"character"`
	CreditID    string  `json:"creditId"`
	Gender      int     `json:"gender"`
	Name        string  `json:"name"`
	Order       int     `json:"order"`
	ProfilePath *string `json:"profilePath"`
}

type Crew struct {
	ID          int     `json:"id"`
	CreditID    string  `json:"creditId"`
	Department  string  `json:"department"`
	Gender      int     `json:"gender"`
	Job         string  `json:"job"`
	Name        string  `json:"name"`
	ProfilePath *string `json:"profilePath"`
}

type ExternalIds struct {
	ImdbID      tmdb.Optional[string]          `json:"imdbId,omitzero"`
	FreebaseMid tmdb.Optional[string]          `json:"freebaseMid,omitzero"`
	FreebaseID  tmdb.Optional[string]          `json:"freebaseId,omitzero"`
	TvdbID      tmdb.Optional[int]             `json:"tvdbId,omitzero"`
	TvrageID    tmdb.Optional[json.RawMessage] `json:"tvrageId,omitzero"`
	FacebookID  tmdb.Optional[string]          `json:"facebookId,omitzero"`
	InstagramID tmdb.Optional[string]          `json:"instagramId,omitzero"`
	TwitterID   tmdb.Optional[string]          `json:"twitterId,omitzero"`
}

// Video is upstream's Video from server/models/Movie.ts, which mapVideos
// produces. URL is absent for any site but YouTube.
type Video struct {
	Site string  `json:"site"`
	Key  string  `json:"key"`
	Name string  `json:"name"`
	Size int     `json:"size"`
	Type string  `json:"type"`
	URL  *string `json:"url,omitempty"`
}

// WatchProviders keeps TMDB's snake_case iso_3166_1 inside an otherwise
// camelCase object, because upstream does and the kept UI reads it so.
type WatchProviders struct {
	ISO3166_1 string                 `json:"iso_3166_1"`
	Link      string                 `json:"link"`
	Buy       []WatchProviderDetails `json:"buy"`
	Flatrate  []WatchProviderDetails `json:"flatrate"`
}

type WatchProviderDetails struct {
	DisplayPriority int     `json:"displayPriority"`
	LogoPath        *string `json:"logoPath"`
	ID              int     `json:"id"`
	Name            string  `json:"name"`
}

func MapCast(person tmdb.TitleCast) Cast {
	return Cast{
		ID:          person.ID,
		CastID:      person.CastID,
		Character:   person.Character,
		CreditID:    person.CreditID,
		Gender:      person.Gender,
		Name:        person.Name,
		Order:       person.Order,
		ProfilePath: person.ProfilePath,
	}
}

// MapAggregateCast collapses a series actor to the first role, the one with
// the most episodes. An actor with no role at all, which would throw
// upstream, keeps an empty character and credit id.
func MapAggregateCast(person tmdb.AggregateCast) Cast {
	cast := MapCast(person.TitleCast)
	cast.Character, cast.CreditID = "", ""
	if len(person.Roles) > 0 {
		cast.Character = person.Roles[0].Character
		cast.CreditID = person.Roles[0].CreditID
	}
	return cast
}

func MapCrew(person tmdb.TitleCrew) Crew {
	return Crew{
		ID:          person.ID,
		CreditID:    person.CreditID,
		Department:  person.Department,
		Gender:      person.Gender,
		Job:         person.Job,
		Name:        person.Name,
		ProfilePath: person.ProfilePath,
	}
}

func MapExternalIds(ids tmdb.ExternalIDs) ExternalIds {
	return ExternalIds{
		ImdbID:      ids.IMDbID,
		FreebaseMid: ids.FreebaseMID,
		FreebaseID:  ids.FreebaseID,
		TvdbID:      ids.TVDBID,
		TvrageID:    ids.TVRageID,
		FacebookID:  ids.FacebookID,
		InstagramID: ids.InstagramID,
		TwitterID:   ids.TwitterID,
	}
}

// MapVideos returns nil for a missing videos block, where upstream's
// optional chaining yields undefined and the field is dropped.
func MapVideos(videos *tmdb.VideoResult) []Video {
	if videos == nil {
		return nil
	}
	mapped := make([]Video, 0, len(videos.Results))
	for _, video := range videos.Results {
		mapped = append(mapped, Video{
			Site: video.Site,
			Key:  video.Key,
			Name: video.Name,
			Size: video.Size,
			Type: video.Type,
			URL:  videoSiteURL(video.Site, video.Key),
		})
	}
	return mapped
}

func videoSiteURL(site, key string) *string {
	if site != "YouTube" {
		return nil
	}
	url := "https://www.youtube.com/watch?v=" + key
	return &url
}

// MapWatchProviders lists the regions in ISO code order. Upstream keeps the
// order of TMDB's object, which TMDB sends sorted; a Go map has none, so the
// sort restores it. Rent offers are dropped, as upstream drops them.
func MapWatchProviders(regions map[string]tmdb.ProviderOffer) []WatchProviders {
	codes := make([]string, 0, len(regions))
	for code := range regions {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	mapped := make([]WatchProviders, 0, len(codes))
	for _, code := range codes {
		offer := regions[code]
		mapped = append(mapped, WatchProviders{
			ISO3166_1: code,
			Link:      offer.Link,
			Buy:       MapWatchProviderDetails(offer.Buy),
			Flatrate:  MapWatchProviderDetails(offer.Flatrate),
		})
	}
	return mapped
}

func MapWatchProviderDetails(providers []tmdb.WatchProvider) []WatchProviderDetails {
	mapped := make([]WatchProviderDetails, 0, len(providers))
	for _, provider := range providers {
		mapped = append(mapped, WatchProviderDetails{
			DisplayPriority: provider.DisplayPriority,
			LogoPath:        provider.LogoPath,
			ID:              provider.ProviderID,
			Name:            provider.ProviderName,
		})
	}
	return mapped
}
