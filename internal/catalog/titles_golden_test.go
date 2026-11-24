package catalog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// titleTestClient is a TMDB client over baseURL that never waits and tries
// once, so a 429 or a 5xx fails at once instead of retrying through the
// backoff. attemptTimeout bounds the timeout rows.
func titleTestClient(baseURL string, attemptTimeout time.Duration) *tmdb.Client {
	return tmdb.NewClient(tmdb.Options{
		BaseURL:        baseURL,
		APIKey:         config.NewSecret("sentinelsentinelsentinel"),
		Limiter:        rate.NewLimiter(rate.Every(time.Millisecond), 50),
		Sleep:          func(context.Context, time.Duration) error { return nil },
		Jitter:         func() float64 { return 0 },
		MaxAttempts:    1,
		RetryBudget:    30 * time.Second,
		AttemptTimeout: attemptTimeout,
		MaxConcurrency: 10,
	})
}

// titleServe registers the catalog on a plain mux and answers target. The
// session gate belongs to httpapi and is tested there.
func titleServe(t *testing.T, baseURL string, attemptTimeout time.Duration, target string) (int, []byte) {
	t.Helper()
	mux := http.NewServeMux()
	NewService(titleTestClient(baseURL, attemptTimeout), nil).Register(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return recorder.Code, body
}

func titleGoldenTarget(t *testing.T, golden catalogtest.Golden) string {
	t.Helper()
	method, target, found := strings.Cut(golden.Request, " ")
	if !found || method != http.MethodGet {
		t.Fatalf("golden %s records request %q, want a GET", golden.Case, golden.Request)
	}
	return target
}

// Every title golden upstream answered with a page answers through the
// handlers with the golden's status and a body matching the golden.
func TestTitleGoldensMatchThroughHandlers(t *testing.T) {
	cases := []string{
		"movie-details-full",
		"movie-details-minimal",
		"movie-details-empty",
		"movie-details-empty-overview",
		"movie-details-without-local-trailer",
		"movie-recommendations",
		"movie-similar",
		"tv-details-full",
		"tv-details-minimal",
		"tv-details-empty",
		"tv-details-empty-overview",
		"tv-details-without-local-trailer",
		"tv-details-anime",
		"tv-season-with-stills",
		"tv-season-null-stills",
		"tv-season-empty",
		"tv-recommendations",
		"tv-similar",
		"collection-full",
		"collection-minimal",
		"collection-empty",
		"collection-missing-and-equal-release-dates",
		"person-details-full",
		"person-details-minimal",
		"person-details-empty",
		"person-combined-credits",
		"person-combined-credits-adult-credit",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			golden := catalogtest.LoadGolden(t, "title", name)
			server := catalogtest.FakeTMDB(t, golden.Fixtures...)
			status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden))
			if status != golden.Status {
				t.Fatalf("status = %d, want %d; body %s", status, golden.Status, body)
			}
			catalogtest.CompareToGolden(t, body, golden)
		})
	}
}

// Upstream keeps adult entries in these lists and the goldens record them.
// The adult policy is the intentional difference: each answer must match its
// golden with exactly the adult entries removed, and nothing else changed.
func TestTitleAdultGoldensMatchWithTheAdultEntriesRemoved(t *testing.T) {
	cases := []struct {
		name    string
		list    string
		adultID float64
	}{
		{"movie-recommendations-adult-entry", "results", 87101},
		{"movie-similar-adult-entry", "results", 563},
		{"tv-recommendations-adult-entry", "results", 211358},
		{"tv-similar-adult-entry", "results", 873},
		{"collection-adult-part", "parts", 604},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			golden := catalogtest.LoadGolden(t, "title", tc.name)
			server := catalogtest.FakeTMDB(t, golden.Fixtures...)
			status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden))
			if status != golden.Status {
				t.Fatalf("status = %d, want %d; body %s", status, golden.Status, body)
			}
			catalogtest.CompareToGolden(t, body, titleGoldenWithout(t, golden, tc.list, tc.adultID))
		})
	}
}

// titleGoldenWithout is golden with the one entry whose id is id removed
// from the list under key. It fails t unless exactly one entry goes, so a
// wrong id cannot make the comparison pass vacuously.
func titleGoldenWithout(t *testing.T, golden catalogtest.Golden, key string, id float64) catalogtest.Golden {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(golden.Body, &body); err != nil {
		t.Fatalf("golden %s: %v", golden.Case, err)
	}
	entries, _ := body[key].([]any)
	kept := slices.DeleteFunc(slices.Clone(entries), func(entry any) bool {
		fields, _ := entry.(map[string]any)
		return fields["id"] == id
	})
	if len(kept) != len(entries)-1 {
		t.Fatalf("golden %s: %d entries of %s have id %v, want exactly 1", golden.Case, len(entries)-len(kept), key, id)
	}
	body[key] = kept
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("golden %s: %v", golden.Case, err)
	}
	golden.Body = raw
	return golden
}
