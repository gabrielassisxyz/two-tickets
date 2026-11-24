package catalog

import (
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// The blocks of a details answer the common.ts mappers read.
type commonMapperInput struct {
	Credits struct {
		Cast []tmdb.TitleCast `json:"cast"`
		Crew []tmdb.TitleCrew `json:"crew"`
	} `json:"credits"`
	AggregateCredits struct {
		Cast []tmdb.AggregateCast `json:"cast"`
	} `json:"aggregate_credits"`
	ExternalIDs    tmdb.ExternalIDs  `json:"external_ids"`
	Videos         *tmdb.VideoResult `json:"videos"`
	WatchProviders struct {
		Results map[string]tmdb.ProviderOffer `json:"results"`
	} `json:"watch/providers"`
}

func commonMapEach[In, Out any](values []In, mapOne func(In) Out) []Out {
	mapped := make([]Out, 0, len(values))
	for _, value := range values {
		mapped = append(mapped, mapOne(value))
	}
	return mapped
}

// Upstream's mapMovieDetails embeds each common.ts mapper's output unchanged,
// so the movie details golden pins all of them, external ids with keys TMDB
// leaves out included.
func TestCommonMappersMatchUpstreamOnMovieDetails(t *testing.T) {
	const fixture = "movie/details-full"
	var movie commonMapperInput
	decodeMapperFixture(t, fixture, &movie)
	golden := mapperGoldenBody(t, "title/movie-details-full", fixture)

	mapperRequireGoldenMatch(t, "MapCast", commonMapEach(movie.Credits.Cast, MapCast), mapperField(t, golden, "credits", "cast"))
	mapperRequireGoldenMatch(t, "MapCrew", commonMapEach(movie.Credits.Crew, MapCrew), mapperField(t, golden, "credits", "crew"))
	mapperRequireGoldenMatch(t, "MapExternalIds", MapExternalIds(movie.ExternalIDs), mapperField(t, golden, "externalIds"))
	mapperRequireGoldenMatch(t, "MapVideos", MapVideos(movie.Videos), mapperField(t, golden, "relatedVideos"))
	mapperRequireGoldenMatch(t, "MapWatchProviders", MapWatchProviders(movie.WatchProviders.Results), mapperField(t, golden, "watchProviders"))
}

// The series golden pins the other half: cast from aggregate credits, with no
// castId, and external ids TMDB sends as null.
func TestCommonMappersMatchUpstreamOnSeriesDetails(t *testing.T) {
	const fixture = "tv/details-full"
	var show commonMapperInput
	decodeMapperFixture(t, fixture, &show)
	golden := mapperGoldenBody(t, "title/tv-details-full", fixture)

	mapperRequireGoldenMatch(t, "MapAggregateCast", commonMapEach(show.AggregateCredits.Cast, MapAggregateCast), mapperField(t, golden, "credits", "cast"))
	mapperRequireGoldenMatch(t, "MapCrew", commonMapEach(show.Credits.Crew, MapCrew), mapperField(t, golden, "credits", "crew"))
	mapperRequireGoldenMatch(t, "MapExternalIds", MapExternalIds(show.ExternalIDs), mapperField(t, golden, "externalIds"))
	mapperRequireGoldenMatch(t, "MapWatchProviders", MapWatchProviders(show.WatchProviders.Results), mapperField(t, golden, "watchProviders"))
}

func TestMapVideosLinksOnlyYouTubeAndKeepsAMissingBlockMissing(t *testing.T) {
	if videos := MapVideos(nil); videos != nil {
		t.Errorf("a missing videos block maps to %#v, want nil", videos)
	}
	videos := MapVideos(&tmdb.VideoResult{Results: []tmdb.Video{
		{Key: "abc", Site: "YouTube"},
		{Key: "def", Site: "Vimeo"},
	}})
	if videos[0].URL == nil || *videos[0].URL != "https://www.youtube.com/watch?v=abc" {
		t.Errorf("YouTube url = %v", videos[0].URL)
	}
	if raw := mapperJSONValue(t, videos[1]).(map[string]any); raw["url"] != nil || len(raw) != 5 {
		t.Errorf("a Vimeo video serialises as %v, want no url field", raw)
	}
}

func TestMapAggregateCastWithoutRolesDoesNotPanic(t *testing.T) {
	cast := MapAggregateCast(tmdb.AggregateCast{TitleCast: tmdb.TitleCast{ID: 7, Character: "stale"}})
	if cast.ID != 7 || cast.Character != "" || cast.CreditID != "" {
		t.Errorf("an actor with no roles maps to %+v", cast)
	}
}
