package catalog

import (
	"encoding/json"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

func decodeMapperFixture(t *testing.T, fixtureID string, dest any) {
	t.Helper()
	if err := json.Unmarshal(mapperFixtureBody(t, fixtureID), dest); err != nil {
		t.Fatalf("decode fixture %s: %v", fixtureID, err)
	}
}

// The multi search fixture holds all four kinds, and upstream's search route
// maps it with mapSearchResults alone, so its golden pins every result type.
func TestMapSearchResultsMatchesUpstreamForAllFourKinds(t *testing.T) {
	const fixture = "search/multi-four-kinds"
	var page tmdb.SearchMultiResponse
	decodeMapperFixture(t, fixture, &page)
	golden := mapperField(t, mapperGoldenBody(t, "search/search-multi-four-kinds", fixture), "results")

	kinds := map[string]bool{}
	for _, entry := range golden.([]any) {
		kinds[entry.(map[string]any)["mediaType"].(string)] = true
	}
	for _, kind := range []string{"movie", "tv", "person", "collection"} {
		if !kinds[kind] {
			t.Fatalf("the golden holds no %s, so it cannot pin that kind's shape", kind)
		}
	}
	mapperRequireGoldenMatch(t, "MapSearchResults", MapSearchResults(page.Results), golden)
}

func TestMapMovieAndTvResultMatchUpstreamOnSingleKindLists(t *testing.T) {
	cases := []struct {
		fixture, golden string
		mapPage         func(t *testing.T, fixture string) any
	}{
		{"discover/movies-popular", "discover/discover-movies", mapMoviePageFixture},
		{"movie/recommendations", "title/movie-recommendations", mapMoviePageFixture},
		{"discover/tv-popular", "discover/discover-tv", mapTvPageFixture},
		{"tv/similar", "title/tv-similar", mapTvPageFixture},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			golden := mapperField(t, mapperGoldenBody(t, c.golden, c.fixture), "results")
			mapperRequireGoldenMatch(t, c.fixture, c.mapPage(t, c.fixture), golden)
		})
	}
}

func mapMoviePageFixture(t *testing.T, fixture string) any {
	var page tmdb.DiscoverMoviesResponse
	decodeMapperFixture(t, fixture, &page)
	mapped := make([]MovieResult, 0, len(page.Results))
	for _, movie := range page.Results {
		mapped = append(mapped, MapMovieResult(movie))
	}
	return mapped
}

func mapTvPageFixture(t *testing.T, fixture string) any {
	var page tmdb.DiscoverTVResponse
	decodeMapperFixture(t, fixture, &page)
	mapped := make([]TvResult, 0, len(page.Results))
	for _, show := range page.Results {
		mapped = append(mapped, MapTvResult(show))
	}
	return mapped
}

// Upstream's trending route tests for a title before anything else, so its
// golden records the collection in this fixture as a movie. The port types it
// as a collection: every other entry still matches the trending golden, and
// the collection matches the collection upstream's search route produced
// from the same TMDB entry.
func TestMapSearchResultsTypesTheTrendingCollectionAsACollection(t *testing.T) {
	const fixture = "trending/all-day-four-kinds"
	var page tmdb.TrendingResponse
	decodeMapperFixture(t, fixture, &page)
	golden := mapperField(t, mapperGoldenBody(t, "discover/discover-trending-four-kinds", fixture), "results").([]any)
	searchGolden := mapperField(t, mapperGoldenBody(t, "search/search-multi-four-kinds", "search/multi-four-kinds"), "results").([]any)

	mapped := mapperJSONValue(t, MapSearchResults(page.Results)).([]any)
	if len(mapped) != len(golden) {
		t.Fatalf("mapped %d entries, the golden has %d", len(mapped), len(golden))
	}
	collections := 0
	for i, item := range page.Results {
		if item.Collection == nil {
			mapperRequireGoldenMatch(t, "trending entry", mapped[i], golden[i])
			continue
		}
		collections++
		if kind := mapperField(t, golden[i], "mediaType"); kind != "movie" {
			t.Fatalf("the trending golden types collection %d as %v; the upstream bug this test pins is gone", item.Collection.ID, kind)
		}
		if kind := mapperField(t, mapped[i], "mediaType"); kind != "collection" {
			t.Errorf("collection %d is typed %v, want collection", item.Collection.ID, kind)
		}
		mapperRequireGoldenMatch(t, "trending collection", mapped[i], searchGoldenEntry(t, searchGolden, item.Collection.ID))
	}
	if collections == 0 {
		t.Fatal("the trending fixture holds no collection")
	}
}

func searchGoldenEntry(t *testing.T, results []any, id int) any {
	t.Helper()
	for _, entry := range results {
		if mapperField(t, entry, "id") == float64(id) {
			return entry
		}
	}
	t.Fatalf("no entry %d in the search golden", id)
	return nil
}

// The tmdb: search prefix maps a movie details answer into a result row and
// then through mapSearchResults; the golden is the case where only the movie
// probe succeeds.
func TestMapMovieDetailsToResultMatchesTheTmdbPrefixSearch(t *testing.T) {
	const fixture = "movie/details-minimal"
	var details tmdb.MovieDetails
	decodeMapperFixture(t, fixture, &details)
	golden := mapperField(t, mapperGoldenBody(t, "search/search-tmdb-prefix", fixture), "results")
	mapped := MapSearchResults([]tmdb.MediaItem{MapMovieDetailsToResult(details)})
	mapperRequireGoldenMatch(t, "MapMovieDetailsToResult", mapped, golden)
}

// No golden holds a series or person reached through tmdb:, so these two are
// held to the shape of a series and a person result upstream did produce,
// and to the values they copy from the details.
func TestMapTvAndPersonDetailsToResultProduceResultShapes(t *testing.T) {
	var show tmdb.TVDetails
	decodeMapperFixture(t, "tv/details-full", &show)
	showRow := MapTvDetailsToResult(show)
	if showRow.MediaType != "tv" || showRow.TV == nil || showRow.TV.ID != show.ID || len(showRow.TV.GenreIDs) != len(show.Genres) || showRow.TV.GenreIDs[0] != show.Genres[0].ID {
		t.Errorf("series row = %+v, want the details' id and genre ids", showRow.TV)
	}
	tvGolden := mapperField(t, mapperGoldenBody(t, "discover/discover-tv", "discover/tv-popular"), "results", 0)
	requireSameTopLevelShape(t, "series details row", MapSearchResults([]tmdb.MediaItem{showRow})[0], tvGolden)

	var person tmdb.PersonDetails
	decodeMapperFixture(t, "person/details-full", &person)
	personRow := MapPersonDetailsToResult(person)
	if personRow.MediaType != "person" || personRow.Person == nil || personRow.Person.ID != person.ID || personRow.Person.Popularity != person.Popularity {
		t.Errorf("person row = %+v, want the details' id and popularity", personRow.Person)
	}
	mappedPerson := MapSearchResults([]tmdb.MediaItem{personRow})[0]
	if known := mappedPerson.(PersonResult).KnownFor; known == nil || len(known) != 0 {
		t.Errorf("knownFor = %#v, want an empty array", known)
	}
	personGolden := searchGoldenEntry(t, mapperField(t, mapperGoldenBody(t, "search/search-multi", "search/multi-cooper"), "results").([]any), 51329)
	requireSameTopLevelShape(t, "person details row", mappedPerson, personGolden)
}

func requireSameTopLevelShape(t *testing.T, what string, actual any, golden any) {
	t.Helper()
	got := mapperJSONValue(t, actual).(map[string]any)
	want := golden.(map[string]any)
	for key, value := range want {
		if mapperVolatileKeys[key] {
			value = 0.0
		}
		if _, ok := got[key]; !ok {
			t.Errorf("%s: %s missing", what, key)
		} else if gotType, wantType := mapperJSONType(got[key]), mapperJSONType(value); gotType != wantType && value != nil && got[key] != nil {
			t.Errorf("%s: %s is %s, want %s", what, key, gotType, wantType)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("%s: unexpected field %s", what, key)
		}
	}
}
