package catalog

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
)

// upstreamDiscoverSliderTypes is upstream's DiscoverSliderType enum at the
// pinned commit (server/constants/discover.ts, mirrored in
// web/src/compat/constants/discover.ts), written out so the manifest is
// checked against the numbers the renderer switches on.
var upstreamDiscoverSliderTypes = map[string]int{
	"TRENDING": 4, "POPULAR_MOVIES": 5, "MOVIE_GENRES": 6, "UPCOMING_MOVIES": 7, "STUDIOS": 8,
	"POPULAR_TV": 9, "TV_GENRES": 10, "UPCOMING_TV": 11, "NETWORKS": 12,
}

// projectDiscoverSliderTypes are this app's own slider types, written out as
// the literals web/src/compat/sliderIds.ts declares.
var projectDiscoverSliderTypes = map[string]int{"FOR_YOU": 100, "MY_LIST": 101}

func discoverSliderTypeOf(name string) int {
	if value, ok := projectDiscoverSliderTypes[name]; ok {
		return value
	}
	return upstreamDiscoverSliderTypes[name]
}

func TestDiscoverSliderManifestIsTheForYouListThenMyListThenTheNineKeptBuiltIns(t *testing.T) {
	response := discoverServe(t, discoverTestService("http://tmdb.invalid"), http.MethodGet, "/api/v1/settings/discover")
	if response.Code != http.StatusOK {
		t.Fatalf("settings/discover = %d %s", response.Code, response.Body)
	}
	var sliders []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &sliders); err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"FOR_YOU", "MY_LIST", "TRENDING", "POPULAR_MOVIES", "MOVIE_GENRES", "UPCOMING_MOVIES", "STUDIOS", "POPULAR_TV", "TV_GENRES", "UPCOMING_TV", "NETWORKS"}
	if len(sliders) != len(wantOrder) {
		t.Fatalf("the manifest holds %d sliders, want %d: %s", len(sliders), len(wantOrder), response.Body)
	}
	ids := map[float64]bool{}
	for i, slider := range sliders {
		if got, want := slider["type"], float64(discoverSliderTypeOf(wantOrder[i])); got != want {
			t.Errorf("slider %d has type %v, want %s (%v)", i, got, wantOrder[i], want)
		}
		if slider["enabled"] != true || slider["isBuiltIn"] != true {
			t.Errorf("slider %d is not an enabled built-in: %v", i, slider)
		}
		if order, _ := slider["order"].(float64); i > 0 && order <= sliders[i-1]["order"].(float64) {
			t.Errorf("slider %d has order %v, not after %v", i, order, sliders[i-1]["order"])
		}
		id, _ := slider["id"].(float64)
		if ids[id] {
			t.Errorf("slider id %v appears twice", id)
		}
		ids[id] = true
		if _, own := projectDiscoverSliderTypes[wantOrder[i]]; own {
			if id < 100 {
				t.Errorf("slider %d (%s) has id %v, want 100 or above", i, wantOrder[i], id)
			}
			for name, upstream := range upstreamDiscoverSliderTypes {
				if float64(upstream) == slider["type"] {
					t.Errorf("slider %d (%s) collides with upstream %s", i, wantOrder[i], name)
				}
			}
		}
		for _, key := range []string{"title", "data"} {
			if value, present := slider[key]; !present || value != nil {
				t.Errorf("slider %d %s = %v (present %v), want null", i, key, value, present)
			}
		}
	}
}

// The display language stays pt-BR while language=ja filters by original
// language; no recorded movie fixture filters by Japanese, so the request is
// checked against the recorded Portuguese one with only that value changed.
func TestDiscoverMoviesLanguageJaFiltersByOriginalLanguage(t *testing.T) {
	recorded := discoverReadFixture(t, "discover/movies-language-pt")
	stub := discoverStartStub(t, recorded.Body)
	response := discoverServe(t, discoverTestService(stub.server.URL), http.MethodGet, "/api/v1/discover/movies?page=1&language=ja")
	if response.Code != http.StatusOK {
		t.Fatalf("discover/movies?language=ja = %d %s", response.Code, response.Body)
	}
	path, params := stub.discoverOnlyRequest(t)
	want := maps.Clone(recorded.Params)
	want["with_original_language"] = "ja"
	if path != recorded.Path || !maps.Equal(params, want) {
		t.Errorf("TMDB was asked %s %v, want %s %v", path, params, recorded.Path, want)
	}
	if params["language"] != "pt-BR" {
		t.Errorf("display language = %q, want pt-BR", params["language"])
	}
}

// The same split on the series route, where a recorded fixture exists.
func TestDiscoverTVLanguageJaMatchesTheRecordedRequest(t *testing.T) {
	server := catalogtest.FakeTMDB(t, "discover/tv-language-ja")
	response := discoverServe(t, discoverTestService(server.URL), http.MethodGet, "/api/v1/discover/tv?page=1&language=ja")
	if response.Code != http.StatusOK {
		t.Fatalf("discover/tv?language=ja = %d %s", response.Code, response.Body)
	}
}

// Regression test for upstream's trending mapper, which types a collection
// as a movie.
func TestDiscoverTrendingTypesTheCollectionAsACollection(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "discover", "discover-trending-four-kinds")
	response := discoverGoldenCase(t, golden)
	kinds := map[string]int{}
	for _, result := range discoverResults(t, response.Body.Bytes()) {
		kinds[result["mediaType"].(string)]++
		if result["id"] != float64(2344) {
			continue
		}
		if result["mediaType"] != "collection" {
			t.Errorf("collection 2344 is typed %v, want collection", result["mediaType"])
		}
		if _, movieField := result["releaseDate"]; movieField {
			t.Errorf("collection 2344 has the movie shape: %v", result)
		}
	}
	for _, kind := range []string{"movie", "tv", "person", "collection"} {
		if kinds[kind] == 0 {
			t.Errorf("trending returned no %s entry: %v", kind, kinds)
		}
	}
}

func TestDiscoverGenreSliderKeepsOnlyGenresWithFiveBackdrops(t *testing.T) {
	for _, c := range []struct {
		golden  string
		present int
		absent  []int // the genres of the 0, 1 and 4 backdrop fixtures
	}{
		{"discover-genreslider-movie", 35, []int{28, 12, 16}},
		{"discover-genreslider-tv", 80, []int{10759, 16, 35}},
	} {
		response := discoverGoldenCase(t, catalogtest.LoadGolden(t, "discover", c.golden))
		var items []discoverGenreSliderItem
		if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
			t.Fatalf("%s: %v", c.golden, err)
		}
		ids := []int{}
		for _, item := range items {
			ids = append(ids, item.ID)
			if item.ID == c.present && len(item.Backdrops) < 5 {
				t.Errorf("%s: genre %d has %d backdrops, want at least 5", c.golden, item.ID, len(item.Backdrops))
			}
		}
		if !slices.Contains(ids, c.present) {
			t.Errorf("%s: genre %d with five backdrops is missing from %v", c.golden, c.present, ids)
		}
		for _, id := range c.absent {
			if slices.Contains(ids, id) {
				t.Errorf("%s: genre %d with too few backdrops is present in %v", c.golden, id, ids)
			}
		}
	}
}

// The fixtures leave one genre past the threshold, so the order is pinned on
// the filter itself: by the localised name, compared as upstream's sortBy
// compares strings.
func TestDiscoverGenreSliderSortsByLocalisedName(t *testing.T) {
	five := []string{"/1.jpg", "/2.jpg", "/3.jpg", "/4.jpg", "/5.jpg"}
	kept := discoverKeepDrawableGenres([]discoverGenreSliderItem{
		{ID: 35, Name: "Comédia", Backdrops: five},
		{ID: 28, Name: "Ação", Backdrops: five},
		{ID: 99, Name: "Documentário", Backdrops: five[:4]},
		{ID: 16, Name: "Animação", Backdrops: five},
		{ID: 18, Name: "Drama", Backdrops: append(five, "/6.jpg")},
	})
	var names []string
	for _, item := range kept {
		names = append(names, item.Name)
	}
	if want := []string{"Animação", "Ação", "Comédia", "Drama"}; !slices.Equal(names, want) {
		t.Errorf("genres = %v, want %v", names, want)
	}
}

func TestDiscoverAdultFixturesReturnNoAdultEntry(t *testing.T) {
	for _, name := range []string{"discover-movies-adult-entry", "discover-trending-adult-entry"} {
		response := discoverGoldenCase(t, catalogtest.LoadGolden(t, "discover", name))
		results := discoverResults(t, response.Body.Bytes())
		if len(results) == 0 {
			t.Fatalf("%s returned no results", name)
		}
		for _, result := range results {
			if result["adult"] == true {
				t.Errorf("%s returned adult entry %v", name, result["id"])
			}
		}
	}
	adultBackdrops := discoverAdultBackdrops(t, "trending/all-week-adult")
	response := discoverGoldenCase(t, catalogtest.LoadGolden(t, "discover", "backdrops-adult-entry"))
	var backdrops []string
	if err := json.Unmarshal(response.Body.Bytes(), &backdrops); err != nil {
		t.Fatal(err)
	}
	if len(backdrops) == 0 {
		t.Fatal("backdrops returned nothing")
	}
	for _, path := range backdrops {
		if adultBackdrops[path] {
			t.Errorf("backdrops returned the adult entry's artwork %s", path)
		}
	}
}

// No recorded genre list leaves a name empty, so the English fallback is
// exercised against a stand-in that answers both languages.
func TestDiscoverGenresFallBackToEnglishAndDropNameless(t *testing.T) {
	var mu sync.Mutex
	asked := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		language := r.URL.Query().Get("language")
		mu.Lock()
		asked = append(asked, r.URL.Path+"?language="+language)
		mu.Unlock()
		body := `{"genres":[{"id":3,"name":"Drama"},{"id":1,"name":""},{"id":2,"name":"Ação"},{"id":4}]}`
		if language == "en" {
			body = `{"genres":[{"id":1,"name":"Western"},{"id":2,"name":"Action"},{"id":3,"name":"Drama"}]}`
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	genres, err := discoverFetchGenres(t.Context(), discoverTestClient(server.URL), "/genre/movie/list")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, genre := range genres {
		names = append(names, genre.Name)
	}
	if want := []string{"Ação", "Drama", "Western"}; !slices.Equal(names, want) {
		t.Errorf("genres = %v, want %v", names, want)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"/genre/movie/list?language=pt-BR", "/genre/movie/list?language=en"}; !slices.Equal(asked, want) {
		t.Errorf("TMDB was asked %v, want %v", asked, want)
	}
}
