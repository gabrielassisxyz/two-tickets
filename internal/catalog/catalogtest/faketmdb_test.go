package catalogtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// readFixtureFile decodes a fixture straight from its file, independently of
// the loader under test.
func readFixtureFile(t *testing.T, id string) tmdbFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "testdata", "tmdb", filepath.FromSlash(id)+".json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", id, err)
	}
	var fixture tmdbFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", id, err)
	}
	return fixture
}

// fixtureURL is the request the fixture recorded, against base, carrying an
// api_key the fake must ignore.
func fixtureURL(base string, fixture tmdbFixture) string {
	query := url.Values{"api_key": {"sentinelsentinelsentinel"}}
	for key, value := range fixture.Request.Params {
		query.Set(key, value)
	}
	return base + fixture.Request.Path + "?" + query.Encode()
}

func fetchFromFake(t *testing.T, client *http.Client, target string) (int, []byte, error) {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, body, err
}

func TestFakeTMDBServesTheRecordedBodyAndStatus(t *testing.T) {
	for _, id := range []string{"genre/movie-list", "errors/discover-movies-404", "errors/discover-movies-429", "errors/discover-movies-500"} {
		t.Run(id, func(t *testing.T) {
			fixture := readFixtureFile(t, id)
			server := FakeTMDB(t, id)
			status, body, err := fetchFromFake(t, server.Client(), fixtureURL(server.URL, fixture))
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			if status != fixture.Response.Status {
				t.Errorf("status = %d, want %d", status, fixture.Response.Status)
			}
			if !bytes.Equal(body, fixture.Response.Body) {
				t.Errorf("body = %s\nwant %s", body, fixture.Response.Body)
			}
		})
	}
}

// The fixture is one whose recorded request carries language, region and
// include_adult, which tmdb.Client adds to every request it sends.
func TestFakeTMDBAnswersTheTMDBClientAtItsBaseURL(t *testing.T) {
	server := FakeTMDB(t, "discover/movies-popular")
	client := tmdb.NewClient(tmdb.Options{BaseURL: server.URL, APIKey: config.NewSecret("sentinelsentinelsentinel")})
	movies, err := client.GetDiscoverMovies(context.Background(), map[string]string{
		"include_video":          "true",
		"page":                   "1",
		"sort_by":                "popularity.desc",
		"with_original_language": "",
	})
	if err != nil {
		t.Fatalf("GetDiscoverMovies: %v", err)
	}
	if movies.Page != 1 || len(movies.Results) == 0 {
		t.Errorf("discover = page %d with %d results, want the recorded first page", movies.Page, len(movies.Results))
	}
}

func TestFakeTMDBServesTheFirstListedFixtureOfTwoThatMatch(t *testing.T) {
	fixture := readFixtureFile(t, "errors/discover-movies-429")
	recorder := newRecordingTB(t)
	server := FakeTMDB(recorder, "errors/discover-movies-429", "errors/discover-movies-500")
	status, _, err := fetchFromFake(t, server.Client(), fixtureURL(server.URL, fixture))
	recorder.runCleanups()
	if err != nil || status != http.StatusTooManyRequests {
		t.Fatalf("status = %d (%v), want 429 from the first listed fixture", status, err)
	}
	if failures := recorder.recorded(); len(failures) != 1 || !strings.Contains(failures[0], "errors/discover-movies-500 was listed but never requested") {
		t.Fatalf("failures %q, want only the second fixture reported unused", failures)
	}
}

func TestFakeTMDBFailsTheTestOnAnUnknownRequest(t *testing.T) {
	fixture := readFixtureFile(t, "genre/movie-list")
	cases := map[string]string{
		"another path":        strings.Replace(fixtureURL("", fixture), "/genre/movie/list", "/genre/tv/list", 1),
		"another param value": strings.Replace(fixtureURL("", fixture), "language=pt-BR", "language=en-US", 1),
		"an extra param":      fixtureURL("", fixture) + "&page=2",
		"a missing param":     "/genre/movie/list?api_key=sentinelsentinelsentinel",
		"a repeated param":    fixtureURL("", fixture) + "&language=pt-BR",
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := newRecordingTB(t)
			server := FakeTMDB(recorder, "genre/movie-list")
			status, body, err := fetchFromFake(t, server.Client(), server.URL+target)
			if err == nil {
				t.Errorf("GET %s answered %d %s, want the connection dropped", target, status, body)
			}
			failures := recorder.recorded()
			if len(failures) != 1 || !strings.Contains(failures[0], "no listed fixture matches GET /genre/") {
				t.Fatalf("failures %q, want one naming the unmatched request", failures)
			}
			if strings.Contains(failures[0], "sentinel") {
				t.Errorf("the failure %q prints the api_key", failures[0])
			}
			recorder.runCleanups()
		})
	}
}

func TestFakeTMDBFailsTheTestOnAListedFixtureNeverRequested(t *testing.T) {
	fixture := readFixtureFile(t, "genre/movie-list")
	recorder := newRecordingTB(t)
	server := FakeTMDB(recorder, "genre/movie-list", "genre/tv-list")
	if _, _, err := fetchFromFake(t, server.Client(), fixtureURL(server.URL, fixture)); err != nil {
		t.Fatalf("GET: %v", err)
	}
	if failures := recorder.recorded(); len(failures) != 0 {
		t.Fatalf("failures before cleanup: %q", failures)
	}
	recorder.runCleanups()
	failures := recorder.recorded()
	if len(failures) != 1 || failures[0] != "FakeTMDB: fixture genre/tv-list was listed but never requested" {
		t.Fatalf("failures %q, want genre/tv-list reported unused", failures)
	}
}

func TestFakeTMDBHoldsATimeoutRequestUntilTheClientGivesUp(t *testing.T) {
	fixture := readFixtureFile(t, "errors/discover-movies-timeout")
	server := FakeTMDB(t, fixture.ID)
	client := server.Client()
	client.Timeout = 100 * time.Millisecond
	status, body, err := fetchFromFake(t, client, fixtureURL(server.URL, fixture))
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("GET answered %d %s (err %v), want the client's own timeout", status, body, err)
	}
}

// A client that never gives up must not hang the test at cleanup: the held
// request is dropped before the server waits for requests in flight.
func TestFakeTMDBReleasesAHeldTimeoutRequestAtCleanup(t *testing.T) {
	fixture := readFixtureFile(t, "errors/discover-movies-timeout")
	recorder := newRecordingTB(t)
	server, fake := startFakeTMDB(recorder, []string{fixture.ID})
	request, err := http.NewRequest(http.MethodGet, fixtureURL(server.URL, fixture), nil)
	if err != nil {
		t.Fatal(err)
	}
	clientDone := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(request)
		if err == nil {
			_ = response.Body.Close()
		}
		clientDone <- err
	}()
	<-fake.holding
	cleanupDone := make(chan struct{})
	go func() {
		recorder.runCleanups()
		close(cleanupDone)
	}()
	select {
	case <-cleanupDone:
	case <-time.After(10 * time.Second):
		t.Fatal("cleanup is still waiting on the held request")
	}
	if err := <-clientDone; err == nil {
		t.Error("the held request was answered, want its connection dropped")
	}
	if failures := recorder.recorded(); len(failures) != 0 {
		t.Errorf("failures %q, want none: the fixture was requested", failures)
	}
}

func TestEveryIndexedFixtureLoads(t *testing.T) {
	files := tmdbFixtureFiles(t, repositoryRoot(t))
	ids := make([]string, 0, len(files))
	for id := range files {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		t.Fatal("testdata/tmdb/index.json lists no fixture")
	}
	timeouts := 0
	for _, fixture := range loadTMDBFixtures(t, ids) {
		if fixture.Response.Error == tmdbTimeoutError {
			timeouts++
		}
	}
	if timeouts == 0 {
		t.Error("no indexed fixture is a timeout, so the timeout form went unloaded")
	}
}
