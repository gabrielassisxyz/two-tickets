package catalogtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// tmdbFixture is one recorded TMDB exchange from testdata/tmdb/. A response
// carries either a status and body, or error "timeout": TMDB never answered.
type tmdbFixture struct {
	ID      string `json:"id"`
	Request struct {
		Path   string            `json:"path"`
		Params map[string]string `json:"params"`
	} `json:"request"`
	Response struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
		Error  string          `json:"error"`
	} `json:"response"`
}

const tmdbTimeoutError = "timeout"

// FakeTMDB starts a server that stands in for TMDB's /3 root, so a client
// takes its URL as the base URL. It answers only from the listed fixtures,
// matched by exact path and query parameters with api_key ignored; when two
// listed fixtures match, the first listed wins. A timeout fixture holds the
// request open until the client gives up. A request no listed fixture
// matches fails t and has its connection dropped; a listed fixture that was
// never requested fails t when the test ends.
func FakeTMDB(t testing.TB, fixtureIDs ...string) *httptest.Server {
	t.Helper()
	fake := &fakeTMDBServer{
		t:        t,
		fixtures: loadTMDBFixtures(t, fixtureIDs),
		served:   map[string]bool{},
		stopped:  make(chan struct{}),
	}
	server := httptest.NewServer(http.HandlerFunc(fake.answer))
	t.Cleanup(func() {
		// Release held timeout requests first: Close waits for every
		// request in flight, and a client that never gives up would hang it.
		close(fake.stopped)
		server.Close()
		fake.reportUnused()
	})
	return server
}

type fakeTMDBServer struct {
	t        testing.TB
	fixtures []tmdbFixture
	stopped  chan struct{}

	mu     sync.Mutex
	served map[string]bool
}

func (f *fakeTMDBServer) answer(w http.ResponseWriter, r *http.Request) {
	fixture, found := f.match(r)
	if !found {
		f.t.Errorf("FakeTMDB: no listed fixture matches %s %s", r.Method, requestWithoutAPIKey(r))
		panic(http.ErrAbortHandler)
	}
	if fixture.Response.Error == tmdbTimeoutError {
		select {
		case <-r.Context().Done():
			return
		case <-f.stopped:
			panic(http.ErrAbortHandler)
		}
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(fixture.Response.Status)
	_, _ = w.Write(fixture.Response.Body)
}

func (f *fakeTMDBServer) match(r *http.Request) (tmdbFixture, bool) {
	if r.Method != http.MethodGet {
		return tmdbFixture{}, false
	}
	params := map[string]string{}
	for key, values := range r.URL.Query() {
		if key == "api_key" {
			continue
		}
		// A fixture records one value per parameter, so a repeated one
		// matches nothing rather than an arbitrary pick of its values.
		if len(values) != 1 {
			return tmdbFixture{}, false
		}
		params[key] = values[0]
	}
	for _, fixture := range f.fixtures {
		if fixture.Request.Path == r.URL.Path && sameTMDBParams(fixture.Request.Params, params) {
			f.mu.Lock()
			f.served[fixture.ID] = true
			f.mu.Unlock()
			return fixture, true
		}
	}
	return tmdbFixture{}, false
}

// requestWithoutAPIKey is the request as a failure prints it: path and query,
// with the key left out of the test log.
func requestWithoutAPIKey(r *http.Request) string {
	query := r.URL.Query()
	query.Del("api_key")
	if len(query) == 0 {
		return r.URL.Path
	}
	return r.URL.Path + "?" + query.Encode()
}

func sameTMDBParams(recorded, got map[string]string) bool {
	if len(recorded) != len(got) {
		return false
	}
	for key, value := range recorded {
		if gotValue, exists := got[key]; !exists || gotValue != value {
			return false
		}
	}
	return true
}

func (f *fakeTMDBServer) reportUnused() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, fixture := range f.fixtures {
		if !f.served[fixture.ID] {
			f.t.Errorf("FakeTMDB: fixture %s was listed but never requested", fixture.ID)
		}
	}
}

// loadTMDBFixtures reads each listed fixture through testdata/tmdb/index.json,
// so an id the index does not list fails the test instead of being guessed.
func loadTMDBFixtures(t testing.TB, fixtureIDs []string) []tmdbFixture {
	t.Helper()
	root := repositoryRoot(t)
	files := tmdbFixtureFiles(t, root)
	fixtures := make([]tmdbFixture, 0, len(fixtureIDs))
	for _, id := range fixtureIDs {
		file, listed := files[id]
		if !listed {
			t.Fatalf("FakeTMDB: testdata/tmdb/index.json lists no fixture %q", id)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatalf("FakeTMDB: %v", err)
		}
		var fixture tmdbFixture
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("FakeTMDB: fixture %s: %v", id, err)
		}
		if fixture.ID != id {
			t.Fatalf("FakeTMDB: %s records id %q, want %q", file, fixture.ID, id)
		}
		validateTMDBFixture(t, fixture)
		fixtures = append(fixtures, fixture)
	}
	return fixtures
}

func validateTMDBFixture(t testing.TB, fixture tmdbFixture) {
	t.Helper()
	switch {
	case fixture.Response.Error == tmdbTimeoutError:
	case fixture.Response.Error != "":
		t.Fatalf("FakeTMDB: fixture %s records an unknown error form %q", fixture.ID, fixture.Response.Error)
	case fixture.Response.Status < 100 || len(fixture.Response.Body) == 0:
		t.Fatalf("FakeTMDB: fixture %s records no status and body", fixture.ID)
	}
}

func tmdbFixtureFiles(t testing.TB, root string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "testdata", "tmdb", "index.json"))
	if err != nil {
		t.Fatalf("FakeTMDB: %v", err)
	}
	var index struct {
		Fixtures []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatalf("FakeTMDB: testdata/tmdb/index.json: %v", err)
	}
	files := make(map[string]string, len(index.Fixtures))
	for _, entry := range index.Fixtures {
		files[entry.ID] = entry.File
	}
	return files
}
