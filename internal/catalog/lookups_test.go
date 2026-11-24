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

// lookupTestClient builds a TMDB client over baseURL that never waits: the
// limiter is generous, retries are off and the clock is the real one. Status
// matrix tests set MaxAttempts 1 so a 429 or 5xx fails fast instead of
// retrying through the backoff.
func lookupTestClient(baseURL string) *tmdb.Client {
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

func lookupServe(t *testing.T, service *Service, method, target string) (int, []byte) {
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

// Every committed golden in the lookup family answers through the handlers
// with the golden status and a body matching the golden.
func TestLookupGoldensMatchThroughHandlers(t *testing.T) {
	cases := []string{
		"regions",
		"languages",
		"studio",
		"network",
		"genres-movie",
		"genres-tv",
		"keyword",
		"watchproviders-regions",
		"watchproviders-movies",
		"watchproviders-tv",
		"certifications-movie",
		"certifications-tv",
		"genres-movie-tmdb-404",
		"genres-movie-tmdb-429",
		"genres-movie-tmdb-500",
		"genres-movie-tmdb-timeout",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			golden := catalogtest.LoadGolden(t, "lookup", name)
			server := catalogtest.FakeTMDB(t, golden.Fixtures...)
			service := NewService(lookupTestClient(server.URL), nil)
			method, target, _ := strings.Cut(golden.Request, " ")
			status, body := lookupServe(t, service, method, target)
			if status != golden.Status {
				t.Fatalf("status = %d, want %d", status, golden.Status)
			}
			// The error goldens carry a plain message body, not a full error
			// shape: upstream's error middleware answers {message, errors}
			// with errors omitted when empty, while this service answers
			// through respond.Error which always keeps errors. Compare the
			// message only for those cases.
			if strings.HasSuffix(name, "-tmdb-404") || strings.HasSuffix(name, "-tmdb-429") || strings.HasSuffix(name, "-tmdb-500") || strings.HasSuffix(name, "-tmdb-timeout") {
				var got struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("body %q is not JSON: %v", body, err)
				}
				var want struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(golden.Body, &want); err != nil {
					t.Fatalf("golden body: %v", err)
				}
				if got.Message != want.Message {
					t.Fatalf("message = %q, want %q", got.Message, want.Message)
				}
				return
			}
			catalogtest.CompareToGolden(t, body, golden)
		})
	}
}

// The genre fallback fills an empty localised name from English, drops a genre
// with no name in either language, and sorts by name.
func TestLookupGenreFallbackDropAndSort(t *testing.T) {
	localised := `{"genres":[{"id":1,"name":""},{"id":2,"name":"Com\u00e9dia"},{"id":3,"name":""}]}`
	english := `{"genres":[{"id":1,"name":"Action"},{"id":2,"name":"Comedy"},{"id":3,"name":""}]}`
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("language") == "en" {
			_, _ = w.Write([]byte(english))
			return
		}
		_, _ = w.Write([]byte(localised))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	status, body := lookupServe(t, service, "GET", "/api/v1/genres/movie")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var genres []lookupGenreItem
	if err := json.Unmarshal(body, &genres); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(genres) != 2 {
		t.Fatalf("genres = %+v, want 2 items (id 1 filled, id 3 dropped)", genres)
	}
	if genres[0].ID != 1 || genres[0].Name != "Action" {
		t.Errorf("first = %+v, want id 1 Action (sorted before Com\u00e9dia)", genres[0])
	}
	if genres[1].ID != 2 || genres[1].Name != "Com\u00e9dia" {
		t.Errorf("second = %+v, want id 2 Com\u00e9dia", genres[1])
	}
	if calls != 2 {
		t.Errorf("TMDB calls = %d, want 2 (localised plus the English fallback)", calls)
	}
}

// Sorting by english_name puts languages and regions in ascending order.
func TestLookupLanguagesAndRegionsSortByEnglishName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/configuration/languages":
			_, _ = w.Write([]byte(`[{"iso_639_1":"pt","english_name":"Portuguese","name":"Portugu\u00eas"},{"iso_639_1":"en","english_name":"English","name":"English"}]`))
		case "/configuration/countries":
			_, _ = w.Write([]byte(`[{"iso_3166_1":"BR","english_name":"Brazil","native_name":"Brazil"},{"iso_3166_1":"AF","english_name":"Afghanistan","native_name":"Afghanistan"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	if status, body := lookupServe(t, service, "GET", "/api/v1/languages"); status != http.StatusOK {
		t.Fatalf("languages status = %d, want 200", status)
	} else {
		var languages []tmdb.Language
		if err := json.Unmarshal(body, &languages); err != nil {
			t.Fatalf("languages body: %v", err)
		}
		if len(languages) != 2 || languages[0].English != "English" || languages[1].English != "Portuguese" {
			t.Errorf("languages = %+v, want English before Portuguese", languages)
		}
	}
	if status, body := lookupServe(t, service, "GET", "/api/v1/regions"); status != http.StatusOK {
		t.Fatalf("regions status = %d, want 200", status)
	} else {
		var regions []tmdb.Country
		if err := json.Unmarshal(body, &regions); err != nil {
			t.Fatalf("regions body: %v", err)
		}
		if len(regions) != 2 || regions[0].English != "Afghanistan" || regions[1].English != "Brazil" {
			t.Errorf("regions = %+v, want Afghanistan before Brazil", regions)
		}
	}
}

// Provider lists unwrap results and map display_priority/logo_path/provider_id/
// provider_name to displayPriority/logoPath/id/name.
func TestLookupWatchProviderUnwrapAndMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"display_priority":7,"logo_path":"/p.jpg","provider_id":9,"provider_name":"Nine"}]}`))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	status, body := lookupServe(t, service, "GET", "/api/v1/watchproviders/movies?watchRegion=BR")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var providers []lookupWatchProvider
	if err := json.Unmarshal(body, &providers); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if len(providers) != 1 {
		t.Fatalf("providers = %+v, want 1", providers)
	}
	if providers[0].ID != 9 || providers[0].Name != "Nine" {
		t.Errorf("provider = %+v, want id 9 Nine", providers[0])
	}
	if providers[0].DisplayPriority == nil || *providers[0].DisplayPriority != 7 {
		t.Errorf("displayPriority = %+v, want 7", providers[0].DisplayPriority)
	}
	if providers[0].LogoPath == nil || *providers[0].LogoPath != "/p.jpg" {
		t.Errorf("logoPath = %+v, want /p.jpg", providers[0].LogoPath)
	}
}

func TestLookupMapStudioAndNetwork(t *testing.T) {
	description := "A studio"
	headquarters := "Burbank"
	homepage := "https://example.com"
	logo := "/logo.png"
	origin := "US"
	studio := lookupMapProductionCompany(lookupStudioTMDB{ID: 2, Name: "Studio", OriginCountry: "US", Description: &description, Headquarters: &headquarters, Homepage: &homepage, LogoPath: &logo})
	if studio.ID != 2 || studio.Name != "Studio" || studio.OriginCountry != "US" {
		t.Errorf("studio = %+v, want id 2 Studio US", studio)
	}
	if studio.Description == nil || *studio.Description != description {
		t.Errorf("studio description = %+v, want %q", studio.Description, description)
	}
	_ = origin
	network := lookupMapNetwork(lookupNetworkTMDB{ID: 213, Name: "Network", OriginCountry: &origin, Headquarters: &headquarters, Homepage: &homepage, LogoPath: &logo})
	if network.ID != 213 || network.Name != "Network" {
		t.Errorf("network = %+v, want id 213 Network", network)
	}
	if network.OriginCountry == nil || *network.OriginCountry != "US" {
		t.Errorf("network originCountry = %+v, want US", network.OriginCountry)
	}
}

// Invalid ids answer 400 in the error shape without reaching TMDB.
func TestLookupInvalidIDAnswers400(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	for _, target := range []string{"/api/v1/keyword/abc", "/api/v1/studio/0", "/api/v1/network/-5", "/api/v1/keyword/0"} {
		status, body := lookupServe(t, service, "GET", target)
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
		t.Error("TMDB was reached for an invalid id; validation must run first")
	}
}

// An unknown keyword answers 200 with JSON null (upstream's intentional
// answer), and that negative is served from the TMDB client's cache.
func TestLookupUnknownKeywordAnswersNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"status_code":34,"status_message":"Not found."}`))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	for i := 0; i < 2; i++ {
		status, body := lookupServe(t, service, "GET", "/api/v1/keyword/999999")
		if status != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200 (intentional null, not a 404)", i+1, status)
		}
		if strings.TrimSpace(string(body)) != "null" {
			t.Fatalf("attempt %d: body = %q, want JSON null", i+1, body)
		}
	}
}

// Unknown studio and network ids answer 500, keeping upstream's behaviour.
func TestLookupUnknownStudioAndNetworkAnswer500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"status_code":34,"status_message":"Not found."}`))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	for _, tc := range []struct{ target, message string }{
		{"/api/v1/studio/999999", "Unable to retrieve studio."},
		{"/api/v1/network/999999", "Unable to retrieve network."},
	} {
		status, body := lookupServe(t, service, "GET", tc.target)
		if status != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want 500 (upstream's answer for an unknown id)", tc.target, status)
			continue
		}
		var shape struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &shape); err != nil || shape.Message != tc.message {
			t.Errorf("%s body %q, want message %q", tc.target, body, tc.message)
		}
	}
}

// TMDB timeout, 429 and 5xx all answer 500 with the route's message. Each row
// is an intentional normalisation of a retry-exhausted upstream failure.
func TestLookupTMDBFailuresAnswer500(t *testing.T) {
	cases := []struct {
		name       string
		tmdb       http.HandlerFunc
		target     string
		message    string
		clientOpts func(baseURL string) *tmdb.Client
	}{
		{
			name:    "timeout",
			target:  "/api/v1/genres/movie",
			message: "Unable to retrieve movie genres.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(200 * time.Millisecond)
				_, _ = w.Write([]byte(`{"genres":[]}`))
			},
			clientOpts: func(baseURL string) *tmdb.Client {
				return tmdb.NewClient(tmdb.Options{
					BaseURL:        baseURL,
					APIKey:         config.NewSecret("sentinelsentinelsentinel"),
					Limiter:        rate.NewLimiter(rate.Every(time.Millisecond), 50),
					Sleep:          func(ctx context.Context, d time.Duration) error { return nil },
					Jitter:         func() float64 { return 0 },
					MaxAttempts:    1,
					RetryBudget:    30 * time.Second,
					AttemptTimeout: 20 * time.Millisecond,
					MaxConcurrency: 10,
				})
			},
		},
		{
			name:    "429",
			target:  "/api/v1/languages",
			message: "Unable to retrieve languages.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"success":false,"status_code":25,"status_message":"Rate limited."}`))
			},
		},
		{
			name:    "5xx",
			target:  "/api/v1/regions",
			message: "Unable to retrieve regions.",
			tmdb: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"success":false,"status_code":11,"status_message":"Internal."}`))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.tmdb)
			defer server.Close()
			client := lookupTestClient(server.URL)
			if tc.clientOpts != nil {
				client = tc.clientOpts(server.URL)
			}
			service := NewService(client, nil)
			status, body := lookupServe(t, service, "GET", tc.target)
			if status != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500 (intentional normalisation of a TMDB %s)", status, tc.name)
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

// An empty TMDB list answers 200 with an empty array, never null, so a kept
// renderer can iterate it blindly.
func TestLookupEmptyResultAnswersEmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/genre/"):
			_, _ = w.Write([]byte(`{"genres":[]}`))
		case strings.HasPrefix(r.URL.Path, "/configuration/languages"):
			_, _ = w.Write([]byte(`[]`))
		case strings.HasPrefix(r.URL.Path, "/watch/providers/movie"):
			_, _ = w.Write([]byte(`{"results":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	for _, target := range []string{"/api/v1/genres/movie", "/api/v1/languages", "/api/v1/watchproviders/movies?watchRegion=BR"} {
		status, body := lookupServe(t, service, "GET", target)
		if status != http.StatusOK {
			t.Errorf("%s status = %d, want 200", target, status)
			continue
		}
		if strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("%s body = %q, want an empty array", target, body)
		}
	}
}

// A provider request without a region uses BR, matching the recorded fixture.
func TestLookupWatchRegionDefaultsToBR(t *testing.T) {
	var gotRegion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRegion = r.URL.Query().Get("watch_region")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	if status, _ := lookupServe(t, service, "GET", "/api/v1/watchproviders/movies"); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if gotRegion != "BR" {
		t.Errorf("watch_region = %q, want BR by default", gotRegion)
	}
}

// /certifications/movie returns the raw TMDB shape unchanged.
func TestLookupCertificationsReturnRawShape(t *testing.T) {
	raw := `{"certifications":{"BR":[{"certification":"L","meaning":"General","order":1}]}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(raw))
	}))
	defer server.Close()
	service := NewService(lookupTestClient(server.URL), nil)
	status, body := lookupServe(t, service, "GET", "/api/v1/certifications/movie")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var canonBody, canonRaw string
	for _, tc := range []struct {
		src string
		dst *string
	}{
		{string(body), &canonBody},
		{raw, &canonRaw},
	} {
		var decoded any
		if err := json.Unmarshal([]byte(tc.src), &decoded); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		encoded, _ := json.Marshal(decoded)
		*tc.dst = string(encoded)
	}
	if canonBody != canonRaw {
		t.Errorf("body = %s, want the raw TMDB shape %s", canonBody, canonRaw)
	}
}
