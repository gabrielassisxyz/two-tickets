package catalog

import (
	"maps"
	"net/url"
	"testing"
	"time"
)

func discoverMustParse(t *testing.T, media discoverMedia, rawQuery string) map[string]string {
	t.Helper()
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	params, problem := discoverParseQuery(media, query, discoverFrozenNow)
	if problem != "" {
		t.Fatalf("%s refused: %s", rawQuery, problem)
	}
	return params
}

func discoverMustRefuse(t *testing.T, media discoverMedia, rawQuery string) {
	t.Helper()
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if params, problem := discoverParseQuery(media, query, discoverFrozenNow); problem == "" {
		t.Errorf("%s accepted as %v, want it refused", rawQuery, params)
	}
}

// The parameters a plain page request sends are exactly what upstream sent
// for the recorded first pages.
func TestDiscoverQueryDefaultsMatchTheRecordedRequests(t *testing.T) {
	for _, c := range []struct {
		media   discoverMedia
		fixture string
	}{
		{discoverMovieMedia, "discover/movies-popular"},
		{discoverTVMedia, "discover/tv-popular"},
	} {
		got := discoverMustParse(t, c.media, "page=1")
		if want := discoverReadFixture(t, c.fixture).Params; !maps.Equal(got, want) {
			t.Errorf("%s: params %v, want the recorded %v", c.media.path, got, want)
		}
	}
}

func TestDiscoverQuerySortValuesArePerMediaType(t *testing.T) {
	if got := discoverMustParse(t, discoverMovieMedia, "sortBy=primary_release_date.desc")["sort_by"]; got != "primary_release_date.desc" {
		t.Errorf("movie sort_by = %q", got)
	}
	if got := discoverMustParse(t, discoverTVMedia, "sortBy=first_air_date.asc")["sort_by"]; got != "first_air_date.asc" {
		t.Errorf("tv sort_by = %q", got)
	}
	for _, sort := range []string{"vote_count.desc", "popularity.asc"} {
		discoverMustParse(t, discoverMovieMedia, "sortBy="+sort)
		discoverMustParse(t, discoverTVMedia, "sortBy="+sort)
	}
}

// Upstream's .catch(undefined) turns an unknown sort into the default.
func TestDiscoverQueryRefusesAnotherMediaTypesSortIntentionalNormalisation(t *testing.T) {
	discoverMustRefuse(t, discoverMovieMedia, "sortBy=first_air_date.desc")
	discoverMustRefuse(t, discoverTVMedia, "sortBy=revenue.desc")
	discoverMustRefuse(t, discoverTVMedia, "sortBy=popularity")
}

func TestDiscoverQuerySeparatorsAndCoercion(t *testing.T) {
	params := discoverMustParse(t, discoverMovieMedia, "genre=18,35&keywords=9715|180547&watchProviders=8|337&watchRegion=BR&voteAverageGte=7.5&withRuntimeLte=120")
	for param, want := range map[string]string{
		"with_genres":          "18,35",
		"with_keywords":        "9715|180547",
		"with_watch_providers": "8|337",
		"watch_region":         "BR",
		"vote_average.gte":     "7.5",
		"with_runtime.lte":     "120",
	} {
		if params[param] != want {
			t.Errorf("%s = %q, want %q", param, params[param], want)
		}
	}
	// z.coerce.string() turns a repeated parameter into a comma list.
	if got := discoverMustParse(t, discoverMovieMedia, "genre=18&genre=35")["with_genres"]; got != "18,35" {
		t.Errorf("repeated genre = %q, want 18,35", got)
	}
	if got := discoverMustParse(t, discoverTVMedia, "status=0|3&network=213")["with_status"]; got != "0|3" {
		t.Errorf("tv with_status = %q, want 0|3", got)
	}
	// An empty filter is no filter, as TMDB reads it.
	if _, sent := discoverMustParse(t, discoverMovieMedia, "genre=")["with_genres"]; sent {
		t.Error("an empty genre was sent to TMDB")
	}
}

// The final upstream schemas omit certificationMode, so it never reaches TMDB.
func TestDiscoverQueryDropsCertificationMode(t *testing.T) {
	params := discoverMustParse(t, discoverMovieMedia, "certification=PG-13|R&certificationCountry=US&certificationMode=range")
	if params["certification"] != "PG-13|R" || params["certification_country"] != "US" {
		t.Errorf("certification params = %v", params)
	}
	for key := range params {
		if key == "certificationMode" || key == "certification_mode" {
			t.Errorf("certificationMode reached TMDB as %s", key)
		}
	}
}

func TestDiscoverQueryFieldsOfTheOtherMediaTypeAreIgnored(t *testing.T) {
	movie := discoverMustParse(t, discoverMovieMedia, "network=213&firstAirDateGte=2020-01-01&status=0")
	tv := discoverMustParse(t, discoverTVMedia, "studio=2&primaryReleaseDateGte=2020-01-01")
	if !maps.Equal(movie, discoverMustParse(t, discoverMovieMedia, "")) {
		t.Errorf("movie params carry series fields: %v", movie)
	}
	if !maps.Equal(tv, discoverMustParse(t, discoverTVMedia, "")) {
		t.Errorf("tv params carry movie fields: %v", tv)
	}
}

func TestDiscoverQuerySynthesisesTheMissingDateBound(t *testing.T) {
	onlyGte := discoverMustParse(t, discoverMovieMedia, "primaryReleaseDateGte=2026-01-01")
	if onlyGte["primary_release_date.gte"] != "2026-01-01" || onlyGte["primary_release_date.lte"] != "2028-05-25" {
		t.Errorf("only gte: %v, want lte 1.5 years past the frozen clock", onlyGte)
	}
	onlyLte := discoverMustParse(t, discoverTVMedia, "firstAirDateLte=2000-12-31")
	if onlyLte["first_air_date.gte"] != discoverPastBound || onlyLte["first_air_date.lte"] != "2000-12-31" {
		t.Errorf("only lte: %v, want gte %s", onlyLte, discoverPastBound)
	}
	both := discoverMustParse(t, discoverMovieMedia, "primaryReleaseDateGte=2001-01-01&primaryReleaseDateLte=2002-01-01")
	if both["primary_release_date.gte"] != "2001-01-01" || both["primary_release_date.lte"] != "2002-01-01" {
		t.Errorf("both bounds: %v", both)
	}
	// new Date(x).toISOString().split('T')[0] sends the UTC date of a timestamp.
	stamp := discoverMustParse(t, discoverMovieMedia, "primaryReleaseDateLte=2026-03-01T23:30:00-03:00")
	if stamp["primary_release_date.lte"] != "2026-03-02" {
		t.Errorf("timestamp bound = %q, want its UTC date 2026-03-02", stamp["primary_release_date.lte"])
	}
}

func TestDiscoverQueryRefusesInvalidInputIntentionalNormalisation(t *testing.T) {
	for _, raw := range []string{
		"page=0", "page=-1", "page=abc", "page=01", "page=1&page=2",
		"genre=drama", "genre=18%3B35", "genre=0", "keywords=1,,2",
		"primaryReleaseDateGte=2026-13-01", "primaryReleaseDateLte=yesterday",
		"voteAverageGte=11", "voteAverageLte=NaN", "voteCountGte=-3", "withRuntimeGte=1.5",
		"watchRegion=br", "certificationCountry=USA",
		"language=japanese", "language=JA",
	} {
		discoverMustRefuse(t, discoverMovieMedia, raw)
	}
	for _, raw := range []string{"network=213|49", "status=6", "firstAirDateGte=24/11/2026"} {
		discoverMustRefuse(t, discoverTVMedia, raw)
	}
}

// language on /discover/movies and /discover/tv is the original-language
// filter; the display language stays pt-BR whatever the query says.
func TestDiscoverQueryLanguageOnlyFilters(t *testing.T) {
	for _, media := range []discoverMedia{discoverMovieMedia, discoverTVMedia} {
		ja := discoverMustParse(t, media, "language=ja")
		if ja["with_original_language"] != "ja" || ja["language"] != "pt-BR" {
			t.Errorf("%s language=ja: %v, want with_original_language ja and language pt-BR", media.path, ja)
		}
		if _, sent := discoverMustParse(t, media, "language=all")["with_original_language"]; sent {
			t.Errorf("%s language=all still filters by original language", media.path)
		}
		if got, sent := discoverMustParse(t, media, "")["with_original_language"]; !sent || got != "" {
			t.Errorf("%s without language: with_original_language = %q (sent %v), want upstream's empty value", media.path, got, sent)
		}
	}
}

func TestDiscoverUpcomingWindowFollowsTheClock(t *testing.T) {
	for _, c := range []struct {
		now      time.Time
		gte, lte string
	}{
		{discoverFrozenNow, "2026-11-24", "2028-05-25"},
		{time.Date(2027, 2, 28, 9, 0, 0, 0, time.UTC), "2027-02-28", "2028-08-28"},
		// Today is the server's local date, as upstream computes it.
		{time.Date(2026, 11, 24, 22, 0, 0, 0, time.FixedZone("BRT", -3*3600)), "2026-11-24", "2028-05-25"},
	} {
		movie := discoverUpcomingParams(discoverMovieMedia, "1", c.now)
		if movie["primary_release_date.gte"] != c.gte || movie["primary_release_date.lte"] != c.lte {
			t.Errorf("movies at %v: %s..%s, want %s..%s", c.now, movie["primary_release_date.gte"], movie["primary_release_date.lte"], c.gte, c.lte)
		}
		tv := discoverUpcomingParams(discoverTVMedia, "1", c.now)
		if tv["first_air_date.gte"] != c.gte || tv["first_air_date.lte"] != c.lte {
			t.Errorf("series at %v: %s..%s, want %s..%s", c.now, tv["first_air_date.gte"], tv["first_air_date.lte"], c.gte, c.lte)
		}
	}
}
