package catalog

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
)

// The status matrix of the title routes. Upstream answers 500 for every TMDB
// failure, an unknown title included, and the kept pages rely on that 500, so
// those rows keep it. The rows marked as intentional normalisations differ
// from upstream on purpose. The family has no local lookup and no search
// prefix, so it has no rows for either.

// titleRoute is one title route: a valid request, and the message its
// failure carries.
type titleRoute struct {
	name    string
	target  string
	failure string
}

var titleRoutes = []titleRoute{
	{"movie", "/api/v1/movie/603", "Unable to retrieve movie."},
	{"movie-recommendations", "/api/v1/movie/603/recommendations?page=1", "Unable to retrieve movie recommendations."},
	{"movie-similar", "/api/v1/movie/603/similar?page=1", "Unable to retrieve similar movies."},
	{"tv", "/api/v1/tv/1399", "Unable to retrieve series."},
	{"tv-season", "/api/v1/tv/1399/season/1", "Unable to retrieve season."},
	{"tv-recommendations", "/api/v1/tv/1399/recommendations?page=1", "Unable to retrieve series recommendations."},
	{"tv-similar", "/api/v1/tv/1399/similar?page=1", "Unable to retrieve similar series."},
	{"collection", "/api/v1/collection/2344", "Unable to retrieve collection."},
	{"person", "/api/v1/person/52583", "Unable to retrieve person."},
	{"person-combined-credits", "/api/v1/person/52583/combined_credits", "Unable to retrieve combined credits."},
}

// titleErrorBody checks the error shape every failure shares: the message,
// and errors always an array (an intentional normalisation: upstream leaves
// errors out when it has none).
func titleErrorBody(t *testing.T, body []byte, message string) {
	t.Helper()
	var got struct {
		Message string           `json:"message"`
		Errors  *json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("error body is not JSON: %v\n%s", err, body)
	}
	if got.Message != message {
		t.Errorf("message = %q, want %q", got.Message, message)
	}
	if got.Errors == nil || len(*got.Errors) == 0 || (*got.Errors)[0] != '[' {
		t.Errorf("errors is not an array: %s", body)
	}
}

// Intentional normalisation: upstream's validator answers 400 for a
// non-number id only; a zero, negative or oversized id, a negative season
// and a bad page answer 400 here too, before TMDB is asked anything.
func TestTitleStatusInvalidInputAnswers400(t *testing.T) {
	baseURL := titleFakeServer(t, func(r *http.Request) (int, string) {
		t.Errorf("TMDB was asked %s for invalid input", r.URL.Path)
		return http.StatusInternalServerError, `{}`
	})
	targets := []string{
		"/api/v1/movie/abc", "/api/v1/movie/0", "/api/v1/movie/-1", "/api/v1/movie/1.5", "/api/v1/movie/99999999999",
		"/api/v1/movie/603/recommendations?page=0", "/api/v1/movie/603/similar?page=x", "/api/v1/movie/x/similar?page=1",
		"/api/v1/tv/abc", "/api/v1/tv/0/season/1", "/api/v1/tv/1399/season/-1", "/api/v1/tv/1399/season/x",
		"/api/v1/tv/1399/recommendations?page=-2", "/api/v1/tv/x/similar?page=1",
		"/api/v1/collection/abc", "/api/v1/person/0", "/api/v1/person/abc/combined_credits",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			status, body := titleServe(t, baseURL, 5*time.Second, target)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %s", status, body)
			}
			var got struct {
				Message string   `json:"message"`
				Errors  []string `json:"errors"`
			}
			if err := json.Unmarshal(body, &got); err != nil || got.Message == "" || len(got.Errors) != 1 {
				t.Errorf("body = %s, want a message and one error", body)
			}
		})
	}
}

// The movie error goldens: the status is upstream's, the message is
// upstream's, and errors is the shared normalisation above. The 400 golden
// records upstream's validator wording, which is not ported.
func TestTitleStatusMovieErrorGoldens(t *testing.T) {
	for _, name := range []string{"movie-details-not-found", "movie-details-tmdb-404", "movie-details-tmdb-429", "movie-details-tmdb-500", "movie-details-tmdb-timeout", "movie-details-non-numeric-id"} {
		t.Run(name, func(t *testing.T) {
			golden := catalogtest.LoadGolden(t, "title", name)
			server := catalogtest.FakeTMDB(t, golden.Fixtures...)
			status, body := titleServe(t, server.URL, 200*time.Millisecond, titleGoldenTarget(t, golden))
			if status != golden.Status {
				t.Fatalf("status = %d, want %d; body %s", status, golden.Status, body)
			}
			if golden.Status == http.StatusBadRequest {
				return
			}
			var want struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(golden.Body, &want); err != nil {
				t.Fatalf("golden body: %v", err)
			}
			titleErrorBody(t, body, want.Message)
		})
	}
}

// titleStatusFailureRow answers every TMDB request of every title route with
// one failure and expects upstream's 500 with the route's message.
func titleStatusFailureRow(t *testing.T, status int, body string) {
	t.Helper()
	baseURL := titleFakeServer(t, func(*http.Request) (int, string) { return status, body })
	for _, route := range titleRoutes {
		t.Run(route.name, func(t *testing.T) {
			got, answer := titleServe(t, baseURL, 5*time.Second, route.target)
			if got != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body %s", got, answer)
			}
			titleErrorBody(t, answer, route.failure)
		})
	}
}

// Kept from upstream: a title TMDB does not know answers 500, which the kept
// pages show as their error state.
func TestTitleStatusTMDB404Answers500(t *testing.T) {
	titleStatusFailureRow(t, http.StatusNotFound, `{"success":false,"status_code":34}`)
}

func TestTitleStatusTMDB429Answers500(t *testing.T) {
	titleStatusFailureRow(t, http.StatusTooManyRequests, `{"success":false,"status_code":25}`)
}

func TestTitleStatusTMDB5xxAnswers500(t *testing.T) {
	titleStatusFailureRow(t, http.StatusServiceUnavailable, `{}`)
}

func TestTitleStatusTMDBTimeoutAnswers500(t *testing.T) {
	stop := make(chan struct{})
	server := titleFakeServer(t, func(r *http.Request) (int, string) {
		select {
		case <-r.Context().Done():
		case <-stop:
		}
		return http.StatusOK, `{}`
	})
	t.Cleanup(func() { close(stop) })
	for _, route := range titleRoutes {
		t.Run(route.name, func(t *testing.T) {
			got, answer := titleServe(t, server, 100*time.Millisecond, route.target)
			if got != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body %s", got, answer)
			}
			titleErrorBody(t, answer, route.failure)
		})
	}
}

// An empty TMDB answer is a 200 whose lists are empty arrays, never null:
// the kept renderers iterate them unchecked. The details routes' empty rows
// are the *-empty goldens.
func TestTitleStatusEmptyResultAnswersEmptyArrays(t *testing.T) {
	cases := []struct {
		target string
		tmdb   string
		lists  []string
	}{
		{"/api/v1/movie/603/recommendations?page=1", `{"page":1,"results":[],"total_pages":1,"total_results":0}`, []string{"results"}},
		{"/api/v1/movie/603/similar?page=1", `{"page":1,"results":null,"total_pages":1,"total_results":0}`, []string{"results"}},
		{"/api/v1/tv/1399/recommendations?page=1", `{"page":1,"total_pages":1,"total_results":0}`, []string{"results"}},
		{"/api/v1/tv/1399/similar?page=1", `{"page":1,"results":[],"total_pages":1,"total_results":0}`, []string{"results"}},
		{"/api/v1/collection/2344", `{"id":2344,"name":"c"}`, []string{"parts"}},
		{"/api/v1/person/52583/combined_credits", `{"id":52583}`, []string{"cast", "crew"}},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			baseURL := titleFakeServer(t, func(*http.Request) (int, string) { return http.StatusOK, tc.tmdb })
			status, body := titleServe(t, baseURL, 5*time.Second, tc.target)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200; body %s", status, body)
			}
			fields := titleDecode[map[string]json.RawMessage](t, body)
			for _, list := range tc.lists {
				if string(fields[list]) != "[]" {
					t.Errorf("%s = %s, want []", list, fields[list])
				}
			}
		})
	}
}
