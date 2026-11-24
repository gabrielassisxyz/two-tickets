package catalog

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog/catalogtest"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// titleFixtureBody is the TMDB body a recorded fixture holds, read straight
// from its file so a test can state what the fixture really contains.
func titleFixtureBody(t *testing.T, id string) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "tmdb", filepath.FromSlash(id)+".json"))
	if err != nil {
		t.Fatalf("fixture %s: %v", id, err)
	}
	var fixture struct {
		Response struct {
			Body json.RawMessage `json:"body"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("fixture %s: %v", id, err)
	}
	return fixture.Response.Body
}

// titleRecordingProxy stands in front of target and records every request
// the catalog sends, so a test can assert on the outgoing query.
func titleRecordingProxy(t *testing.T, target string) (string, func() []*url.URL) {
	t.Helper()
	var mu sync.Mutex
	var seen []*url.URL
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL)
		mu.Unlock()
		response, err := http.Get(target + r.URL.RequestURI())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = response.Body.Close() }()
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(proxy.Close)
	return proxy.URL, func() []*url.URL {
		mu.Lock()
		defer mu.Unlock()
		return append([]*url.URL{}, seen...)
	}
}

// titleFakeServer answers every request with answer(r), which returns a
// status and a body.
func titleFakeServer(t *testing.T, answer func(*http.Request) (int, string)) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := answer(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func titleDecode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("body is not the expected JSON: %v\n%s", err, body)
	}
	return value
}

// An empty pt-BR overview is filled from a retry in English, and the retry
// asks TMDB for English: upstream's own retry inherits its configured
// locale, which under pt-BR would ask for pt-BR again.
func TestTitleMovieOverviewFallbackRetriesInEnglish(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", "movie-details-empty-overview")
	localised := titleDecode[titleMovieTMDB](t, titleFixtureBody(t, "movie/details-empty-overview"))
	english := titleDecode[titleMovieTMDB](t, titleFixtureBody(t, "movie/details-en-empty-overview"))
	if localised.Overview != "" || english.Overview == "" {
		t.Fatalf("fixtures no longer hold an empty pt-BR and a filled English overview")
	}
	fake := catalogtest.FakeTMDB(t, golden.Fixtures...)
	proxyURL, requests := titleRecordingProxy(t, fake.URL)

	status, body := titleServe(t, proxyURL, 5*time.Second, titleGoldenTarget(t, golden))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	if got := titleDecode[titleMovieDetails](t, body).Overview; got != english.Overview {
		t.Errorf("overview = %q, want the English one %q", got, english.Overview)
	}
	var retries []string
	for _, request := range requests() {
		if request.Query().Get("append_to_response") == titleMovieAppend && request.Query().Get("language") != tmdb.DefaultLanguage {
			retries = append(retries, request.Query().Get("language"))
		}
	}
	if len(retries) != 1 || retries[0] != titleEnglishLanguage {
		t.Errorf("overview retries sent language %v, want exactly one with %q", retries, titleEnglishLanguage)
	}
}

// The series overview falls back to the English details the route already
// asked for.
func TestTitleTvOverviewFallbackUsesTheEnglishDetails(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", "tv-details-empty-overview")
	english := titleDecode[titleTvTMDB](t, titleFixtureBody(t, "tv/details-en-empty-overview"))
	if titleDecode[titleTvTMDB](t, titleFixtureBody(t, "tv/details-empty-overview")).Overview != "" || english.Overview == "" {
		t.Fatalf("fixtures no longer hold an empty pt-BR and a filled English overview")
	}
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	if got := titleDecode[titleTvDetails](t, body).Overview; got != english.Overview {
		t.Errorf("overview = %q, want the English one %q", got, english.Overview)
	}
}

// Intentional normalisation: upstream fails the whole page when the English
// overview retry fails; here the page is served with the overview empty.
func TestTitleMovieOverviewRetryFailureStillAnswers(t *testing.T) {
	body := string(titleFixtureBody(t, "movie/details-empty-overview"))
	baseURL := titleFakeServer(t, func(r *http.Request) (int, string) {
		if r.URL.Query().Get("language") == titleEnglishLanguage {
			return http.StatusInternalServerError, `{}`
		}
		return http.StatusOK, body
	})
	status, answer := titleServe(t, baseURL, 5*time.Second, "/api/v1/movie/11441")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, answer)
	}
	if got := titleDecode[titleMovieDetails](t, answer).Overview; got != "" {
		t.Errorf("overview = %q, want empty", got)
	}
}

func TestTitleMergeEnglishTrailersAddsOnlyNewTrailers(t *testing.T) {
	localised := &tmdb.VideoResult{Results: []tmdb.Video{
		{Key: "k1", Type: "Teaser", Site: "YouTube"},
		{Key: "k2", Type: "Clip", Site: "YouTube"},
	}}
	english := &tmdb.VideoResult{Results: []tmdb.Video{
		{Key: "k1", Type: "Trailer", Site: "YouTube"},
		{Key: "k3", Type: "Trailer", Site: "YouTube"},
		{Key: "k4", Type: "Featurette", Site: "YouTube"},
	}}
	merged := titleMergeEnglishTrailers(localised, english)
	var keys []string
	for _, video := range merged.Results {
		keys = append(keys, video.Key)
	}
	if strings.Join(keys, ",") != "k1,k2,k3" {
		t.Errorf("merged keys = %v, want k1,k2,k3: the localised videos, then the one new English trailer", keys)
	}
	if got := titleMergeEnglishTrailers(nil, &tmdb.VideoResult{Results: []tmdb.Video{{Key: "k4", Type: "Clip"}}}); got != nil {
		t.Errorf("no new trailer turned an absent videos block into %+v", got)
	}
}

// A movie with no localised trailer gets the English trailers TMDB has,
// after its own videos.
func TestTitleMovieAddsEnglishTrailersAfterTheLocalisedVideos(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", "movie-details-without-local-trailer")
	localised := titleDecode[titleMovieTMDB](t, titleFixtureBody(t, "movie/details-without-local-trailer"))
	english := titleDecode[titleVideosTMDB](t, titleFixtureBody(t, "movie/videos-en-without-local-trailer"))
	want := titleMergeEnglishTrailers(localised.Videos, english.Videos)
	if want == nil || len(want.Results) <= len(localised.Videos.Results) {
		t.Fatalf("fixtures no longer hold an English trailer the localised videos lack")
	}
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	got := titleDecode[titleMovieDetails](t, body).RelatedVideos
	if len(got) != len(want.Results) {
		t.Fatalf("relatedVideos holds %d videos, want %d", len(got), len(want.Results))
	}
	for i, video := range want.Results {
		if got[i].Key != video.Key {
			t.Errorf("relatedVideos[%d] = %s, want %s", i, got[i].Key, video.Key)
		}
	}
}

// A failing English video request is swallowed: the page answers with the
// localised videos alone.
func TestTitleMovieTrailerRequestFailureKeepsTheLocalisedVideos(t *testing.T) {
	// The recorded movie without a local trailer has no localised video at
	// all, so this body carries one localised teaser to keep.
	body := `{"id":26564,"overview":"Sinopse.","videos":{"results":[{"key":"local","site":"YouTube","type":"Teaser"}]}}`
	localised := titleDecode[titleMovieTMDB](t, []byte(body))
	var videoRequests int
	baseURL := titleFakeServer(t, func(r *http.Request) (int, string) {
		if r.URL.Query().Get("append_to_response") == "videos" {
			videoRequests++
			return http.StatusInternalServerError, `{}`
		}
		return http.StatusOK, body
	})
	status, answer := titleServe(t, baseURL, 5*time.Second, "/api/v1/movie/26564")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, answer)
	}
	if videoRequests != 1 {
		t.Fatalf("English video requests = %d, want 1", videoRequests)
	}
	got := titleDecode[titleMovieDetails](t, answer).RelatedVideos
	if len(got) != len(localised.Videos.Results) || got[0].Key != "local" {
		t.Errorf("relatedVideos = %+v, want the localised teaser alone", got)
	}
}

func TestTitleSeasonExpandsStillsAndKeepsNullStills(t *testing.T) {
	still := "/still.jpg"
	season := mapTitleSeasonWithEpisodes(titleSeasonTMDB{Episodes: []titleEpisodeTMDB{
		{ID: 1, StillPath: &still},
		{ID: 2, StillPath: nil},
	}})
	if got := season.Episodes[0].StillPath; got == nil || *got != "https://image.tmdb.org/t/p/original//still.jpg" {
		t.Errorf("still = %v, want the full TMDB URL upstream builds", got)
	}
	if got := season.Episodes[1].StillPath; got != nil {
		t.Errorf("null still = %q, want null", *got)
	}
	if still != "/still.jpg" {
		t.Errorf("expanding the still rewrote the TMDB answer to %q", still)
	}
}

// A series with the anime keyword is answered from TMDB alone: the fake
// answers only the two recorded TMDB fixtures and fails on anything else.
func TestTitleAnimeSeriesIsServedFromTMDB(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", "tv-details-anime")
	show := titleDecode[titleTvTMDB](t, titleFixtureBody(t, "tv/details-anime"))
	animeKeyword := false
	for _, keyword := range show.Keywords.Results {
		animeKeyword = animeKeyword || keyword.ID == 210024
	}
	if !animeKeyword {
		t.Fatalf("fixture no longer carries the anime keyword")
	}
	fake := catalogtest.FakeTMDB(t, golden.Fixtures...)
	proxyURL, requests := titleRecordingProxy(t, fake.URL)
	status, body := titleServe(t, proxyURL, 5*time.Second, titleGoldenTarget(t, golden))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	for _, request := range requests() {
		if request.Path != "/tv/97009" {
			t.Errorf("request to %s, want only the series' TMDB details", request.Path)
		}
	}
	if got := len(requests()); got != len(golden.Fixtures) {
		t.Errorf("sent %d requests, want %d", got, len(golden.Fixtures))
	}
	catalogtest.CompareToGolden(t, body, golden)
}

func TestTitleCollectionOrdersPartsAsUpstream(t *testing.T) {
	raw := `{"id":1,"name":"c","parts":[
		{"id":1,"release_date":"2003-05-15"},
		{"id":2},
		{"id":3,"release_date":""},
		{"id":4,"release_date":"1999-03-31"},
		{"id":5,"release_date":"2003-05-15"},
		{"id":6,"release_date":null},
		{"id":7,"release_date":"2001-01-01","adult":true}]}`
	collection := mapTitleCollection(titleDecode[titleCollectionTMDB](t, []byte(raw)))
	var ids []int
	for _, part := range collection.Parts {
		ids = append(ids, part.ID)
	}
	// An empty date sorts first, equal dates keep TMDB's order, a missing
	// or null date goes last, and the adult part is gone.
	want := []int{3, 4, 1, 5, 2, 6}
	if len(ids) != len(want) {
		t.Fatalf("part ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("part ids = %v, want %v", ids, want)
		}
	}
}

// titleAdultMovieGolden records upstream serving an adult movie.
const titleAdultMovieGolden = "movie-details-adult"

// A direct request for an adult title answers 404 after the first TMDB
// request, so the fallback requests the golden lists are never sent.
func TestTitleAdultMovieAnswers404(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", titleAdultMovieGolden)
	if !titleDecode[titleMovieTMDB](t, titleFixtureBody(t, "movie/details-adult")).Adult {
		t.Fatalf("fixture movie/details-adult is no longer adult")
	}
	server := catalogtest.FakeTMDB(t, "movie/details-adult")
	if status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden)); status != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body %s", status, body)
	}
}

func TestTitleAdultSeriesAndItsSeasonsAnswer404(t *testing.T) {
	baseURL := titleFakeServer(t, func(r *http.Request) (int, string) {
		if strings.Contains(r.URL.Path, "/season/") {
			t.Errorf("season of an adult series was requested")
		}
		return http.StatusOK, `{"id":7,"adult":true,"name":"x","videos":{"results":[{"key":"t","type":"Trailer"}]}}`
	})
	for _, target := range []string{"/api/v1/tv/7", "/api/v1/tv/7/season/1"} {
		if status, body := titleServe(t, baseURL, 5*time.Second, target); status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404; body %s", target, status, body)
		}
	}
}

func TestTitleCombinedCreditsDropAdultAndThanksCredits(t *testing.T) {
	raw := `{"id":9,
		"cast":[{"id":1,"character":"Hero"},{"id":2,"character":"Villain","adult":true},{"id":3,"character":"Thanks"}],
		"crew":[{"id":4,"job":"Director"},{"id":5,"job":"Writer","adult":true},{"id":6,"job":"Thanks"}]}`
	credits := mapTitleCombinedCredits(titleDecode[titleCombinedCreditsTMDB](t, []byte(raw)))
	if len(credits.Cast) != 1 || credits.Cast[0].ID != 1 {
		t.Errorf("cast = %+v, want only id 1", credits.Cast)
	}
	if len(credits.Crew) != 1 || credits.Crew[0].ID != 4 {
		t.Errorf("crew = %+v, want only id 4", credits.Crew)
	}
}

// A non-adult person with one adult credit keeps every other credit.
func TestTitlePersonWithAnAdultCreditLosesOnlyThatCredit(t *testing.T) {
	golden := catalogtest.LoadGolden(t, "title", "person-combined-credits-adult-credit")
	fixture := titleDecode[titleCombinedCreditsTMDB](t, titleFixtureBody(t, "person/combined-credits-adult-credit"))
	server := catalogtest.FakeTMDB(t, golden.Fixtures...)
	status, body := titleServe(t, server.URL, 5*time.Second, titleGoldenTarget(t, golden))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	got := titleDecode[titleCombinedCredits](t, body)
	if len(got.Cast) != len(fixture.Cast)-1 {
		t.Errorf("cast holds %d credits, want %d (all but the adult one)", len(got.Cast), len(fixture.Cast)-1)
	}
	for _, credit := range got.Cast {
		if credit.ID == 1316092 {
			t.Errorf("the adult credit 1316092 is still listed")
		}
	}
}
