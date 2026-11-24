package catalog

import (
	"encoding/json"
	"fmt"
	"testing"
)

// One test per route of the family, each answering a golden's request from
// the fixtures it lists and held to its body. FakeTMDB fails a test whose
// route asks TMDB anything upstream did not, or skips a fixture upstream used.

func TestDiscoverGoldenMovies(t *testing.T) { discoverRequireGolden(t, "discover-movies", nil) }

func TestDiscoverGoldenMoviesWithKeywords(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-keywords", nil)
}

func TestDiscoverGoldenMoviesByLanguage(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-language", nil)
}

func TestDiscoverGoldenMoviesByGenre(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-genre", nil)
}

func TestDiscoverGoldenMoviesByStudio(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-studio", nil)
}

func TestDiscoverGoldenMoviesUpcoming(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-upcoming", nil)
}

func TestDiscoverGoldenTV(t *testing.T) { discoverRequireGolden(t, "discover-tv", nil) }

func TestDiscoverGoldenTVByLanguage(t *testing.T) {
	discoverRequireGolden(t, "discover-tv-language", nil)
}

func TestDiscoverGoldenTVByGenre(t *testing.T) { discoverRequireGolden(t, "discover-tv-genre", nil) }

func TestDiscoverGoldenTVByNetwork(t *testing.T) {
	discoverRequireGolden(t, "discover-tv-network", nil)
}

func TestDiscoverGoldenTVUpcoming(t *testing.T) {
	discoverRequireGolden(t, "discover-tv-upcoming", nil)
}

func TestDiscoverGoldenTrending(t *testing.T) { discoverRequireGolden(t, "discover-trending", nil) }

func TestDiscoverGoldenTrendingMovies(t *testing.T) {
	discoverRequireGolden(t, "discover-trending-movies", nil)
}

func TestDiscoverGoldenTrendingTV(t *testing.T) {
	discoverRequireGolden(t, "discover-trending-tv", nil)
}

func TestDiscoverGoldenKeywordMovies(t *testing.T) {
	discoverRequireGolden(t, "discover-keyword-movies", nil)
}

func TestDiscoverGoldenBackdrops(t *testing.T) { discoverRequireGolden(t, "backdrops", nil) }

// Upstream keeps every genre; the port keeps only those with enough artwork
// for a card, so the golden is cut the same way and the survivors must still
// match it field for field.
func TestDiscoverGoldenMovieGenreSliderIntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-genreslider-movie", discoverKeepGoldenGenresWithFiveBackdrops)
}

func TestDiscoverGoldenTVGenreSliderIntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-genreslider-tv", discoverKeepGoldenGenresWithFiveBackdrops)
}

func discoverKeepGoldenGenresWithFiveBackdrops(t *testing.T, body any) any {
	t.Helper()
	genres, ok := body.([]any)
	if !ok {
		t.Fatalf("genre slider golden %v is not an array", body)
	}
	kept := []any{}
	for _, genre := range genres {
		if backdrops, _ := genre.(map[string]any)["backdrops"].([]any); len(backdrops) >= 5 {
			kept = append(kept, genre)
		}
	}
	if len(kept) == 0 || len(kept) == len(genres) {
		t.Fatalf("the genre slider golden keeps %d of %d genres; the fixtures no longer test the threshold", len(kept), len(genres))
	}
	return kept
}

// Upstream relies on include_adult=false and passes an adult title TMDB still
// returns; the port drops it, so the golden loses exactly that entry.
func TestDiscoverGoldenMoviesAdultEntryIntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-adult-entry", discoverDropGoldenAdultResults)
}

func TestDiscoverGoldenTrendingAdultEntryIntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-trending-adult-entry", discoverDropGoldenAdultResults)
}

func TestDiscoverGoldenBackdropsAdultEntryIntentionalNormalisation(t *testing.T) {
	adultBackdrops := discoverAdultBackdrops(t, "trending/all-week-adult")
	discoverRequireGolden(t, "backdrops-adult-entry", func(t *testing.T, body any) any {
		paths, ok := body.([]any)
		if !ok {
			t.Fatalf("backdrops golden %v is not an array", body)
		}
		kept := []any{}
		for _, path := range paths {
			if !adultBackdrops[fmt.Sprint(path)] {
				kept = append(kept, path)
			}
		}
		if len(kept) == len(paths) {
			t.Fatal("the backdrops golden holds no adult backdrop; the fixture no longer tests the filter")
		}
		return kept
	})
}

func discoverDropGoldenAdultResults(t *testing.T, body any) any {
	t.Helper()
	page := body.(map[string]any)
	results := page["results"].([]any)
	kept := []any{}
	for _, result := range results {
		if result.(map[string]any)["adult"] != true {
			kept = append(kept, result)
		}
	}
	if len(kept) == len(results) {
		t.Fatal("the golden holds no adult entry; the fixture no longer tests the filter")
	}
	page["results"] = kept
	return page
}

// discoverAdultBackdrops reads the artwork of every adult entry of a fixture.
func discoverAdultBackdrops(t *testing.T, fixtureID string) map[string]bool {
	t.Helper()
	var body struct {
		Results []struct {
			Adult        bool    `json:"adult"`
			BackdropPath *string `json:"backdrop_path"`
		} `json:"results"`
	}
	if err := json.Unmarshal(discoverReadFixture(t, fixtureID).Body, &body); err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, result := range body.Results {
		if result.Adult && result.BackdropPath != nil {
			paths[*result.BackdropPath] = true
		}
	}
	if len(paths) == 0 {
		t.Fatalf("fixture %s holds no adult entry with artwork", fixtureID)
	}
	return paths
}

// Upstream's trending mapper types a collection as a movie because it tests
// for a title first. The port types it as a collection: the entry must match
// the collection upstream's own search route produced for the same id, and
// every other entry must still match the trending golden.
func TestDiscoverGoldenTrendingFourKindsIntentionalNormalisation(t *testing.T) {
	searchGolden := mapperField(t, mapperGoldenBody(t, "search/search-multi-four-kinds", "search/multi-four-kinds"), "results").([]any)
	discoverRequireGolden(t, "discover-trending-four-kinds", func(t *testing.T, body any) any {
		results := body.(map[string]any)["results"].([]any)
		replaced := 0
		for i, result := range results {
			entry := result.(map[string]any)
			id := fmt.Sprint(entry["id"])
			if id != "2344" {
				continue
			}
			if entry["mediaType"] != "movie" {
				t.Fatalf("the trending golden types collection %s as %v; the upstream bug this test pins is gone", id, entry["mediaType"])
			}
			results[i] = discoverEncodeRoundTrip(t, searchGoldenEntry(t, searchGolden, 2344))
			replaced++
		}
		if replaced != 1 {
			t.Fatalf("the trending golden holds %d entries for collection 2344, want 1", replaced)
		}
		return body
	})
}

func discoverEncodeRoundTrip(t *testing.T, value any) any {
	t.Helper()
	return discoverDecode(t, discoverEncode(t, value))
}
