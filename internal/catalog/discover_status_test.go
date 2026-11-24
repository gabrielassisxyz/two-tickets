package catalog

import (
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/respond"
)

// The Discover family's status matrix, one handler test per row. Upstream's
// behaviour is kept where the kept UI relies on it: its 500 with the route's
// message when TMDB fails, its 404 for a genre or language TMDB does not
// list, its empty page. A row whose answer differs from upstream's in any way
// says "IntentionalNormalisation" in its name. Every error body differs in
// one way: respond.Error adds errors, an array, next to upstream's message.
// The family has no prefix search, so the matrix has no partial-success row.

func discoverRequireErrorBody(t *testing.T, response []byte, status int, message string) {
	t.Helper()
	var body respond.ErrorBody
	if err := json.Unmarshal(response, &body); err != nil {
		t.Fatalf("error body %s: %v", response, err)
	}
	if body.Message != message || body.Errors == nil {
		t.Errorf("status %d answered %s, want message %q and an errors array", status, response, message)
	}
}

// Upstream never refuses a discover query: a bad page reaches TMDB as NaN
// and comes back as a 500. No TMDB fixture is listed, so a request that
// reached TMDB would fail the test.
func TestDiscoverStatusInvalidInputAnswers400IntentionalNormalisation(t *testing.T) {
	service := discoverTestService(catalogtest.FakeTMDB(t).URL)
	for _, target := range []string{
		"/api/v1/discover/movies?page=0",
		"/api/v1/discover/tv?page=abc",
		"/api/v1/discover/movies?sortBy=first_air_date.desc",
		"/api/v1/discover/tv?firstAirDateGte=tomorrow",
		"/api/v1/discover/movies/upcoming?page=-2",
		"/api/v1/discover/movies/genre/drama?page=1",
		"/api/v1/discover/tv/network/0?page=1",
		"/api/v1/discover/movies/studio/2?page=x",
		"/api/v1/discover/keyword/9715a/movies?page=1",
		"/api/v1/discover/movies/language/pt?page=0",
		"/api/v1/discover/trending?mediaType=person",
		"/api/v1/discover/trending?timeWindow=month",
	} {
		response := discoverServe(t, service, http.MethodGet, target)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %s, want 400", target, response.Code, response.Body)
			continue
		}
		discoverRequireErrorBody(t, response.Body.Bytes(), response.Code, http.StatusText(http.StatusBadRequest))
	}
}

func TestDiscoverStatusTMDB404KeepsUpstreams500IntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-tmdb-404", discoverWithErrorsArray)
}

func TestDiscoverStatusTMDBTimeoutKeepsUpstreams500IntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-tmdb-timeout", discoverWithErrorsArray)
}

func TestDiscoverStatusTMDB429KeepsUpstreams500IntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-tmdb-429", discoverWithErrorsArray)
}

func TestDiscoverStatusTMDB5xxKeepsUpstreams500IntentionalNormalisation(t *testing.T) {
	discoverRequireGolden(t, "discover-movies-tmdb-500", discoverWithErrorsArray)
}

// A missing local lookup is a genre or a language TMDB's list does not hold;
// upstream answers 404 before asking TMDB for the page.
func TestDiscoverStatusMissingGenreKeepsUpstreams404IntentionalNormalisation(t *testing.T) {
	service := discoverTestService(catalogtest.FakeTMDB(t, "genre/movie-list").URL)
	response := discoverServe(t, service, http.MethodGet, "/api/v1/discover/movies/genre/424242?page=1")
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown genre = %d %s, want 404", response.Code, response.Body)
	}
	discoverRequireErrorBody(t, response.Body.Bytes(), response.Code, "Genre not found.")
}

func TestDiscoverStatusMissingLanguageKeepsUpstreams404IntentionalNormalisation(t *testing.T) {
	service := discoverTestService(catalogtest.FakeTMDB(t, "configuration/languages").URL)
	response := discoverServe(t, service, http.MethodGet, "/api/v1/discover/tv/language/qq?page=1")
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown language = %d %s, want 404", response.Code, response.Body)
	}
	discoverRequireErrorBody(t, response.Body.Bytes(), response.Code, "Language not found.")
}

// TMDB answers an empty page; upstream answers it unchanged, and so does the
// port, with empty arrays rather than null. No recorded fixture is empty, so
// the stand-in answers one and the request is checked against the recorded
// first page.
func TestDiscoverStatusEmptyResultAnswersAnEmptyPage(t *testing.T) {
	recorded := discoverReadFixture(t, "discover/movies-popular")
	stub := discoverStartStub(t, []byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
	response := discoverServe(t, discoverTestService(stub.server.URL), http.MethodGet, "/api/v1/discover/movies?page=1")
	if response.Code != http.StatusOK {
		t.Fatalf("empty discover = %d %s, want 200", response.Code, response.Body)
	}
	if got, want := response.Body.String(), `{"page":1,"totalPages":1,"totalResults":0,"results":[],"keywords":[]}`+"\n"; got != want {
		t.Errorf("empty discover body = %s, want %s", got, want)
	}
	if path, params := stub.discoverOnlyRequest(t); path != recorded.Path || !maps.Equal(params, recorded.Params) {
		t.Errorf("TMDB was asked %s %v, want the recorded %s %v", path, params, recorded.Path, recorded.Params)
	}
}
