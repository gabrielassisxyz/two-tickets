package catalog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// searchTestClient builds a TMDB client over baseURL that never waits: the
// limiter is generous, retries are off and the clock is the real one. Status
// matrix tests set MaxAttempts 1 so a 429 or 5xx fails fast instead of
// retrying through the backoff.
func searchTestClient(baseURL string) *tmdb.Client {
	return tmdb.NewClient(tmdb.Options{
		BaseURL:        baseURL,
		APIKey:         config.NewSecret("sentinelsentinelsentinel"),
		Limiter:        rate.NewLimiter(rate.Every(time.Millisecond), 50),
		Sleep:          func(ctx context.Context, d time.Duration) error { return nil },
		Jitter:         func() float64 { return 0 },
		MaxAttempts:    1,
		RetryBudget:    30 * time.Second,
		AttemptTimeout: 5 * time.Second,
		MaxConcurrency: 10,
	})
}

func searchServe(t *testing.T, service *Service, method, target string) (int, []byte) {
	t.Helper()
	mux := http.NewServeMux()
	service.Register(mux)
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return rec.Code, body
}

// Every committed golden in the search family answers through the handlers
// with the golden status and a body matching the golden, except the missing
// query case: upstream answers through its OpenAPI validator, this service
// through respond.Error, so only the status is compared there and the shape is
// covered by the validation tests below as an intentional normalisation.
func TestSearchGoldensMatchThroughHandlers(t *testing.T) {
	cases := []string{
		"search-multi",
		"search-multi-four-kinds",
		"search-tmdb-prefix",
		"search-tmdb-prefix-invalid-id",
		"search-imdb-prefix",
		"search-tvdb-prefix",
		"search-year-prefix",
		"search-year-prefix-tv-fails",
		"search-year-field-only",
		"search-keyword",
		"search-company",
		"search-multi-tmdb-404",
		"search-multi-tmdb-429",
		"search-multi-tmdb-500",
		"search-multi-tmdb-timeout",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			golden := catalogtest.LoadGolden(t, "search", name)
			server := catalogtest.FakeTMDB(t, golden.Fixtures...)
			service := NewService(searchTestClient(server.URL), nil)
			method, target, _ := strings.Cut(golden.Request, " ")
			status, body := searchServe(t, service, method, target)
			if status != golden.Status {
				t.Fatalf("status = %d, want %d", status, golden.Status)
			}
			catalogtest.CompareToGolden(t, body, golden)
		})
	}
}

// The missing query golden keeps upstream's OpenAPI validator shape. This
// service answers the same 400 through respond.Error instead, so only the
// status is pinned here; the shape is asserted in the validation tests.
func TestSearchMissingQueryAnswers400(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "search", "search-missing-query")
	service := NewService(searchTestClient("http://127.0.0.1:1"), nil)
	method, target, _ := strings.Cut(golden.Request, " ")
	status, body := searchServe(t, service, method, target)
	if status != golden.Status {
		t.Fatalf("status = %d, want %d", status, golden.Status)
	}
	var shape struct {
		Message string   `json:"message"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(body, &shape); err != nil || shape.Message == "" || shape.Errors == nil {
		t.Fatalf("body %q is not the error shape", body)
	}
}

// The adult search fixture is upstream keeping its adult entry, so the handler
// must return the golden minus that entry. The adult TV entry carries no
// serialized adult flag, so the test pins the removal by id and count.
func TestSearchAdultEntryIsRemoved(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "search", "search-multi-adult-entry")
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	service := NewService(searchTestClient(server.URL), nil)
	method, target, _ := strings.Cut(golden.Request, " ")
	status, body := searchServe(t, service, method, target)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var page struct {
		Page         int              `json:"page"`
		TotalPages   int              `json:"totalPages"`
		TotalResults int              `json:"totalResults"`
		Results      []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(page.Results) != 19 {
		t.Fatalf("results = %d, want 19 (the adult entry removed)", len(page.Results))
	}
	for _, result := range page.Results {
		if id, _ := result["id"].(float64); id == 3508 {
			t.Errorf("adult entry 3508 kept")
		}
	}
	if page.TotalResults != 3516 || page.TotalPages != 176 || page.Page != 1 {
		t.Errorf("pagination = %d/%d/%d, want 1/176/3516 (TMDB counts survive filtering)", page.Page, page.TotalPages, page.TotalResults)
	}
}

// The prefix parser detects each prefix case-insensitively and extracts its
// id, and reports no prefix for a plain query.
func TestSearchPrefixParser(t *testing.T) {
	if id, ok := searchExtractTmdbID("tmdb:603"); !ok || id != "603" {
		t.Errorf("tmdb:603 = %q,%v, want 603,true", id, ok)
	}
	if _, ok := searchExtractTmdbID("cooper"); ok {
		t.Error("plain query matched tmdb:")
	}
	if _, ok := searchExtractTmdbID("tmdb:abc"); ok {
		t.Error("tmdb:abc matched, want no digits")
	}
	if id, ok := searchExtractImdbID("imdb:tt0133093"); !ok || id != "tt0133093" {
		t.Errorf("imdb:tt0133093 = %q,%v", id, ok)
	}
	if id, ok := searchExtractImdbID("imdb:nm0000151"); !ok || id != "nm0000151" {
		t.Errorf("imdb:nm0000151 = %q,%v", id, ok)
	}
	if _, ok := searchExtractImdbID("imdb:123"); ok {
		t.Error("imdb:123 matched, want tt/nm prefix")
	}
	if id, ok := searchExtractTvdbID("tvdb:81189"); !ok || id != "81189" {
		t.Errorf("tvdb:81189 = %q,%v", id, ok)
	}
	if year, ok := searchExtractYear("friends year:1994"); !ok || year != "1994" {
		t.Errorf("year = %q,%v, want 1994,true", year, ok)
	}
	if stripped := searchStripYear("friends year:1994"); stripped != "friends " {
		t.Errorf("stripped = %q, want %q", stripped, "friends ")
	}
	if stripped := searchStripYear("year:1999"); stripped != "" {
		t.Errorf("year-only stripped = %q, want empty", stripped)
	}
}

// A tmdb: query with a non-numeric id answers 400 without reaching TMDB.
// Upstream would search the literal text; refusing it is the intentional
// normalisation.
func TestSearchTmdbNonNumericIDAnswers400(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	for _, target := range []string{"/api/v1/search?query=tmdb%3Aabc", "/api/v1/search?query=tmdb%3A"} {
		status, body := searchServe(t, service, "GET", target)
		if status != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", target, status)
			continue
		}
		var shape struct {
			Message string   `json:"message"`
			Errors  []string `json:"errors"`
		}
		if err := json.Unmarshal(body, &shape); err != nil || shape.Message == "" || shape.Errors == nil {
			t.Errorf("%s body %q is not the error shape", target, body)
		}
	}
	if reached {
		t.Error("TMDB was reached for a non-numeric tmdb: id")
	}
}

// A missing or empty query answers 400 without reaching TMDB.
func TestSearchMissingAndEmptyQueryAnswer400(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	for _, target := range []string{"/api/v1/search", "/api/v1/search?query=", "/api/v1/search?query=%20%20", "/api/v1/search/keyword?query=", "/api/v1/search/company"} {
		status, _ := searchServe(t, service, "GET", target)
		if status != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", target, status)
		}
	}
	if reached {
		t.Error("TMDB was reached for a missing or empty query")
	}
}

// An invalid page answers 400 without reaching TMDB.
func TestSearchInvalidPageAnswers400(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	for _, target := range []string{"/api/v1/search?query=cooper&page=0", "/api/v1/search?query=cooper&page=-1", "/api/v1/search?query=cooper&page=abc", "/api/v1/search/keyword?query=anime&page=0"} {
		status, _ := searchServe(t, service, "GET", target)
		if status != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", target, status)
		}
	}
	if reached {
		t.Error("TMDB was reached for an invalid page")
	}
}

// Partial success: the movie and the person probes succeed and the series
// probe 404s, so the search returns exactly those two results with status 200.
func TestSearchTmdbPartialSuccessReturnsTheTwoHits(t *testing.T) {
	movieBody := []byte(`{"id":603,"title":"Matrix","original_title":"The Matrix","overview":"","popularity":1,"release_date":"1999-03-31","video":false,"vote_average":8,"vote_count":10,"adult":false,"original_language":"en","genres":[{"id":28,"name":"Action"}],"videos":{"results":[{"id":"1","key":"k","name":"T","site":"YouTube","size":1080,"type":"Trailer"}]}}`)
	personBody := []byte(`{"id":603,"name":"Someone","popularity":1,"adult":false,"profile_path":null,"known_for_department":"Acting"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		switch r.URL.Path {
		case "/movie/603":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(movieBody)
		case "/person/603":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(personBody)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"status_code":34,"status_message":"Not found."}`))
		}
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	status, body := searchServe(t, service, "GET", "/api/v1/search?query=tmdb%3A603")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var page struct {
		Page         int              `json:"page"`
		TotalPages   int              `json:"totalPages"`
		TotalResults int              `json:"totalResults"`
		Results      []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(page.Results) != 2 {
		t.Fatalf("results = %d, want 2 (movie and person, series 404s)", len(page.Results))
	}
	kinds := map[string]int{}
	for _, result := range page.Results {
		kind, _ := result["mediaType"].(string)
		kinds[kind]++
	}
	if kinds["movie"] != 1 || kinds["person"] != 1 {
		t.Errorf("kinds = %v, want one movie and one person", kinds)
	}
}

// Unknown result kinds are dropped: a list holding a movie, a series, a
// person, a collection and an unknown kind maps to the first four.
func TestSearchUnknownKindIsDropped(t *testing.T) {
	items := []tmdb.MediaItem{
		{MediaType: "movie", Movie: &tmdb.MovieBrief{ID: 1, Title: "M"}},
		{MediaType: "tv", TV: &tmdb.TVBrief{ID: 2, Name: "S"}},
		{MediaType: "person", Person: &tmdb.PersonBrief{ID: 3, Name: "P"}},
		{MediaType: "collection", Collection: &tmdb.CollectionBrief{ID: 4, Title: "C"}},
		{MediaType: "season"},
	}
	mapped := MapSearchResults(items)
	if len(mapped) != 4 {
		t.Fatalf("mapped = %d, want 4 (unknown kind dropped)", len(mapped))
	}
}

// An adult title returned by external-id lookup never appears: the find
// answer holds one adult movie, the search answers an empty page with 200.
func TestSearchExternalIDAdultIsRemoved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"movie_results":[{"adult":true,"backdrop_path":null,"genre_ids":[28],"id":603,"title":"Matrix","original_language":"en","original_title":"The Matrix","overview":"","popularity":1,"poster_path":null,"release_date":"1999-03-31","video":false,"vote_average":8,"vote_count":10}],"tv_results":[],"person_results":[]}`))
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	status, body := searchServe(t, service, "GET", "/api/v1/search?query=imdb%3Att0133093")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var page struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(page.Results) != 0 {
		t.Errorf("results = %d, want 0 (adult external-id match removed)", len(page.Results))
	}
}

// Keyword and company failures answer 500 with the route message, keeping
// upstream's behaviour. Each row is a handler test, not a golden: upstream
// throws, the route catches, and this service does the same.
func TestSearchKeywordAndCompanyFailuresAnswer500(t *testing.T) {
	cases := []struct {
		name    string
		tmdb    http.HandlerFunc
		target  string
		message string
	}{
		{
			name:    "keyword-429",
			target:  "/api/v1/search/keyword?query=anime&page=1",
			message: "Unable to retrieve keyword search results.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"success":false,"status_code":25,"status_message":"Rate limited."}`))
			},
		},
		{
			name:    "keyword-500",
			target:  "/api/v1/search/keyword?query=anime&page=1",
			message: "Unable to retrieve keyword search results.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"success":false,"status_code":11,"status_message":"Internal."}`))
			},
		},
		{
			name:    "company-500",
			target:  "/api/v1/search/company?query=marvel&page=1",
			message: "Unable to retrieve company search results.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"success":false,"status_code":11,"status_message":"Internal."}`))
			},
		},
		{
			name:    "external-id-404",
			target:  "/api/v1/search?query=imdb%3Att0133093",
			message: "Unable to retrieve search results.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"success":false,"status_code":34,"status_message":"Not found."}`))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.tmdb)
			defer server.Close()
			service := NewService(searchTestClient(server.URL), nil)
			status, body := searchServe(t, service, "GET", tc.target)
			if status != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", status)
			}
			var shape struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &shape); err != nil || shape.Message != tc.message {
				t.Fatalf("body %q, want message %q", body, tc.message)
			}
		})
	}
}

// A TMDB timeout on keyword search answers 500 with the route message.
func TestSearchKeywordTimeoutAnswers500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
	}))
	defer server.Close()
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:        server.URL,
		APIKey:         config.NewSecret("sentinelsentinelsentinel"),
		Limiter:        rate.NewLimiter(rate.Every(time.Millisecond), 50),
		Sleep:          func(ctx context.Context, d time.Duration) error { return nil },
		Jitter:         func() float64 { return 0 },
		MaxAttempts:    1,
		RetryBudget:    30 * time.Second,
		AttemptTimeout: 20 * time.Millisecond,
		MaxConcurrency: 10,
	})
	service := NewService(client, nil)
	status, body := searchServe(t, service, "GET", "/api/v1/search/keyword?query=anime&page=1")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (intentional normalisation of a retry-exhausted timeout)", status)
	}
	var shape struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &shape); err != nil || shape.Message != "Unable to retrieve keyword search results." {
		t.Fatalf("body %q, want the keyword message", body)
	}
}

// An adult movie probed through tmdb: never appears: the movie probe returns
// an adult title and the other probes 404, so the search answers an empty
// page with 200.
func TestSearchTmdbAdultIsRemoved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		if r.URL.Path == "/movie/603" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":603,"title":"Adult","original_title":"Adult","overview":"","popularity":1,"release_date":"1999-03-31","video":false,"vote_average":1,"vote_count":1,"adult":true,"original_language":"en","genres":[{"id":28,"name":"Action"}],"videos":{"results":[]}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"status_code":34,"status_message":"Not found."}`))
	}))
	defer server.Close()
	service := NewService(searchTestClient(server.URL), nil)
	status, body := searchServe(t, service, "GET", "/api/v1/search?query=tmdb%3A603")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var page struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(page.Results) != 0 {
		t.Errorf("results = %d, want 0 (adult tmdb: probe removed)", len(page.Results))
	}
}

// An empty year-only search answers an empty page with 200, as the golden
// pins; this test names the row in the status matrix.
func TestSearchYearOnlyEmptyAnswersEmptyPage(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "search", "search-year-field-only")
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	service := NewService(searchTestClient(server.URL), nil)
	method, target, _ := strings.Cut(golden.Request, " ")
	status, body := searchServe(t, service, method, target)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var page struct {
		TotalResults int              `json:"totalResults"`
		Results      []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if page.TotalResults != 0 || len(page.Results) != 0 {
		t.Errorf("page = %+v, want empty", page)
	}
}
