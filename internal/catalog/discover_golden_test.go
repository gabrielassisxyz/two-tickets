package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// discoverGoldenCases is every golden of the family but the status matrix's,
// each with the rewrite that states its intentional normalisation, if any.
// Each case answers the golden's request from the fixtures it lists and is
// held to its body; FakeTMDB fails a case whose route asks TMDB anything
// upstream did not, or skips a fixture upstream used.
var discoverGoldenCases = map[string]func(t *testing.T, body any) any{
	"discover-movies":          nil,
	"discover-movies-keywords": nil,
	"discover-movies-language": nil,
	"discover-movies-genre":    nil,
	"discover-movies-studio":   nil,
	"discover-movies-upcoming": nil,
	"discover-tv":              nil,
	"discover-tv-language":     nil,
	"discover-tv-genre":        nil,
	"discover-tv-network":      nil,
	"discover-tv-upcoming":     nil,
	"discover-trending":        nil,
	"discover-trending-movies": nil,
	"discover-trending-tv":     nil,
	"discover-keyword-movies":  nil,
	"backdrops":                nil,
	// Upstream keeps every genre; the port keeps only those with enough
	// artwork for a card, so the golden is cut the same way and the
	// survivors must still match it field for field.
	"discover-genreslider-movie": discoverKeepGoldenGenresWithFiveBackdrops,
	"discover-genreslider-tv":    discoverKeepGoldenGenresWithFiveBackdrops,
	// Upstream relies on include_adult=false and passes an adult title TMDB
	// still returns; the port drops it, so the golden loses that entry only.
	"discover-movies-adult-entry":   discoverDropGoldenAdultResults,
	"discover-trending-adult-entry": discoverDropGoldenAdultResults,
	"backdrops-adult-entry":         discoverDropGoldenAdultBackdrops,
	"discover-trending-four-kinds":  discoverTypeGoldenCollectionAsCollection,
}

// discoverStatusMatrixGoldens are the goldens the status matrix rows run,
// each in a test of its own.
var discoverStatusMatrixGoldens = []string{
	"discover-movies-tmdb-404", "discover-movies-tmdb-429",
	"discover-movies-tmdb-500", "discover-movies-tmdb-timeout",
}

func TestDiscoverGoldens(t *testing.T) {
	for name, adjust := range discoverGoldenCases {
		label := name
		if adjust != nil {
			label += "/IntentionalNormalisation"
		}
		t.Run(label, func(t *testing.T) { discoverRequireGolden(t, name, adjust) })
	}
}

// Every golden committed for the family is run by a test, so a golden the
// harness adds cannot sit unchecked.
func TestDiscoverGoldensAreAllRun(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(mapperRepoRoot(t), "testdata", "golden", "discover"))
	if err != nil {
		t.Fatal(err)
	}
	var committed, run []string
	for _, entry := range entries {
		committed = append(committed, strings.TrimSuffix(entry.Name(), ".json"))
	}
	for name := range discoverGoldenCases {
		run = append(run, name)
	}
	run = append(run, discoverStatusMatrixGoldens...)
	slices.Sort(committed)
	slices.Sort(run)
	if !slices.Equal(committed, run) {
		t.Errorf("committed goldens %v, run by tests %v", committed, run)
	}
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
func discoverDropGoldenAdultBackdrops(t *testing.T, body any) any {
	t.Helper()
	adultBackdrops := discoverAdultBackdrops(t, "trending/all-week-adult")
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

// discoverTypeGoldenCollectionAsCollection undoes upstream's trending bug:
// its mapper tests for a title first and types a collection as a movie. The
// entry is replaced by the collection upstream's own search route produced
// for the same id, and every other entry still matches the trending golden.
func discoverTypeGoldenCollectionAsCollection(t *testing.T, body any) any {
	t.Helper()
	searchGolden := mapperField(t, mapperGoldenBody(t, "search/search-multi-four-kinds", "search/multi-four-kinds"), "results").([]any)
	results := body.(map[string]any)["results"].([]any)
	replaced := 0
	for i, result := range results {
		entry := result.(map[string]any)
		if id := fmt.Sprint(entry["id"]); id != "2344" {
			continue
		}
		if entry["mediaType"] != "movie" {
			t.Fatalf("the trending golden types collection 2344 as %v; the upstream bug this rewrite undoes is gone", entry["mediaType"])
		}
		results[i] = discoverEncodeRoundTrip(t, searchGoldenEntry(t, searchGolden, 2344))
		replaced++
	}
	if replaced != 1 {
		t.Fatalf("the trending golden holds %d entries for collection 2344, want 1", replaced)
	}
	return body
}

func discoverEncodeRoundTrip(t *testing.T, value any) any {
	t.Helper()
	return discoverDecode(t, discoverEncode(t, value))
}
