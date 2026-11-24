package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// discoverFrozenNow is the goldens harness's frozen clock
// (web/tools/goldens/lib/frozen-clock.mjs): the upcoming fixtures were
// recorded with today's date in their parameters.
var discoverFrozenNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

// discoverTestClient talks to baseURL with no rate limit and no wait between
// retries, so the status matrix runs in milliseconds. The attempt timeout is
// short because a timeout fixture holds the request until the client gives up.
func discoverTestClient(baseURL string) *tmdb.Client {
	return tmdb.NewClient(tmdb.Options{
		BaseURL:        baseURL,
		Limiter:        rate.NewLimiter(rate.Inf, 1),
		Sleep:          func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		AttemptTimeout: 100 * time.Millisecond,
	})
}

func discoverTestService(baseURL string) *Service {
	return NewService(discoverTestClient(baseURL), func() time.Time { return discoverFrozenNow })
}

// discoverServe sends one request through the routes the service registers
// on a plain mux; the session gate belongs to httpapi and is tested there.
func discoverServe(t *testing.T, service *Service, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	service.Register(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

// discoverGoldenCase answers golden's request from the fixtures the golden
// lists and returns the response.
func discoverGoldenCase(t *testing.T, golden catalogtest.Golden) *httptest.ResponseRecorder {
	t.Helper()
	method, target, found := strings.Cut(golden.Request, " ")
	if !found {
		t.Fatalf("golden %s: request %q has no method", golden.Case, golden.Request)
	}
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	return discoverServe(t, discoverTestService(server.URL), method, target)
}

// discoverRequireGolden runs golden discover/<name> and compares the answer
// with it. adjust, when set, rewrites the golden body first: it is how a test
// states an intentional normalisation of upstream's answer, so every other
// field is still held to the golden.
func discoverRequireGolden(t *testing.T, name string, adjust func(t *testing.T, body any) any) []byte {
	t.Helper()
	golden := catalogtest.LoadGolden(t, "discover", name)
	response := discoverGoldenCase(t, golden)
	if response.Code != golden.Status {
		t.Fatalf("%s answered %d, want %d: %s", golden.Request, response.Code, golden.Status, response.Body)
	}
	if adjust != nil {
		golden.Body = discoverEncode(t, adjust(t, discoverDecode(t, golden.Body)))
	}
	catalogtest.CompareToGolden(t, response.Body.Bytes(), golden)
	return response.Body.Bytes()
}

// discoverDecode keeps numbers as written, so a golden survives a round trip.
func discoverDecode(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return value
}

func discoverEncode(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// discoverWithErrorsArray is the one normalisation of every error body:
// respond.Error always writes errors, an array, next to upstream's message.
func discoverWithErrorsArray(t *testing.T, body any) any {
	t.Helper()
	object, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("error golden body %v is not an object", body)
	}
	object["errors"] = []any{}
	return object
}

// discoverRecordedFixture is a fixture's recorded request and response body.
type discoverRecordedFixture struct {
	Path   string
	Params map[string]string
	Body   json.RawMessage
}

func discoverReadFixture(t *testing.T, fixtureID string) discoverRecordedFixture {
	t.Helper()
	root := mapperRepoRoot(t)
	var index struct {
		Fixtures []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"fixtures"`
	}
	mapperReadJSON(t, filepath.Join(root, "testdata", "tmdb", "index.json"), &index)
	for _, entry := range index.Fixtures {
		if entry.ID != fixtureID {
			continue
		}
		var recorded struct {
			Request struct {
				Path   string            `json:"path"`
				Params map[string]string `json:"params"`
			} `json:"request"`
			Response struct {
				Body json.RawMessage `json:"body"`
			} `json:"response"`
		}
		mapperReadJSON(t, filepath.Join(root, filepath.FromSlash(entry.File)), &recorded)
		return discoverRecordedFixture{Path: recorded.Request.Path, Params: recorded.Request.Params, Body: recorded.Response.Body}
	}
	t.Fatalf("fixture %s is not in testdata/tmdb/index.json", fixtureID)
	return discoverRecordedFixture{}
}

// discoverStubTMDB answers every request with body and records the query of
// each, for the cases no recorded fixture covers. Its requests are still
// checked against a recorded request by the test that uses it.
type discoverStubTMDB struct {
	server  *httptest.Server
	queries chan *http.Request
}

func discoverStartStub(t *testing.T, body []byte) *discoverStubTMDB {
	t.Helper()
	stub := &discoverStubTMDB{queries: make(chan *http.Request, 16)}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.queries <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

// discoverOnlyRequest returns the one request the stub received, its query
// without the API key.
func (s *discoverStubTMDB) discoverOnlyRequest(t *testing.T) (string, map[string]string) {
	t.Helper()
	if len(s.queries) != 1 {
		t.Fatalf("TMDB received %d requests, want 1", len(s.queries))
	}
	request := <-s.queries
	return request.URL.Path, discoverFlatQuery(request.URL.Query())
}

func discoverFlatQuery(query url.Values) map[string]string {
	flat := map[string]string{}
	for key, values := range query {
		if key != "api_key" {
			flat[key] = strings.Join(values, ",")
		}
	}
	return flat
}

func discoverResults(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var page struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return page.Results
}
