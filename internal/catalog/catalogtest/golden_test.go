package catalogtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// recordingTB stands in for a test so a test here can watch the kit fail one.
// Errorf is recorded instead of failing the real test; cleanups are held
// until runCleanups. Anything else goes to the real test.
type recordingTB struct {
	testing.TB
	mu       sync.Mutex
	failures []string
	cleanups []func()
}

func newRecordingTB(t *testing.T) *recordingTB {
	return &recordingTB{TB: t}
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures = append(r.failures, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (r *recordingTB) Cleanup(cleanup func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanups = append(r.cleanups, cleanup)
}

func (r *recordingTB) runCleanups() {
	r.mu.Lock()
	cleanups := r.cleanups
	r.cleanups = nil
	r.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func (r *recordingTB) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.failures...)
}

// encodeOrdered writes a decoded value back as JSON, keys in Object.keys order.
func encodeOrdered(value any) []byte {
	var out bytes.Buffer
	writeOrdered(&out, value)
	return out.Bytes()
}

func writeOrdered(out *bytes.Buffer, value any) {
	switch value := value.(type) {
	case *jsonObject:
		out.WriteByte('{')
		for i, key := range value.objectKeys() {
			if i > 0 {
				out.WriteByte(',')
			}
			encodedKey, _ := json.Marshal(key)
			out.Write(encodedKey)
			out.WriteByte(':')
			writeOrdered(out, value.values[key])
		}
		out.WriteByte('}')
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			writeOrdered(out, item)
		}
		out.WriteByte(']')
	default:
		encoded, _ := json.Marshal(value)
		out.Write(encoded)
	}
}

// markerInstances are the values a golden's type markers turn back into:
// one arbitrary value of each type, standing for whatever TMDB said that day.
var markerInstances = map[string]func() any{
	"<null>":    func() any { return nil },
	"<boolean>": func() any { return true },
	"<number>":  func() any { return 7.25 },
	"<string>":  func() any { return "volatile" },
	"<array>":   func() any { return []any{} },
	"<object>":  func() any { return newJSONObject() },
}

// instantiateMarkers turns a golden body back into a response the golden
// could have been made from, counting the markers it replaced.
func instantiateMarkers(value any, replaced *int) any {
	switch value := value.(type) {
	case []any:
		for i, item := range value {
			value[i] = instantiateMarkers(item, replaced)
		}
	case *jsonObject:
		for _, key := range value.keys {
			marker, isString := value.values[key].(string)
			if instance, isMarker := markerInstances[marker]; isString && isMarker && goldenVolatileKeys[key] {
				value.values[key] = instance()
				*replaced++
				continue
			}
			value.values[key] = instantiateMarkers(value.values[key], replaced)
		}
	}
	return value
}

func committedGoldens(t *testing.T) [][2]string {
	t.Helper()
	root := filepath.Join(repositoryRoot(t), "testdata", "golden")
	var found [][2]string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		family := filepath.Base(filepath.Dir(path))
		found = append(found, [2]string{family, strings.TrimSuffix(entry.Name(), ".json")})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(found) == 0 {
		t.Fatalf("no golden found under %s", root)
	}
	return found
}

// The JavaScript rule wins over the bead's wording here: a golden's body
// holds "<number>" where TMDB's value was, and the rule reads that marker as
// "a number belongs here", so the literal body is not a match for itself.
// What must match is the response the golden was made from, which is the body
// with each marker turned back into a value of the type it records.
func TestEveryCommittedGoldenMatchesTheResponseItWasMadeFrom(t *testing.T) {
	markers := 0
	for _, entry := range committedGoldens(t) {
		golden := LoadGolden(t, entry[0], entry[1])
		body, err := decodeOrderedJSON(golden.Body)
		if err != nil {
			t.Fatalf("%s/%s: %v", entry[0], entry[1], err)
		}
		response := encodeOrdered(instantiateMarkers(body, &markers))
		t.Run(entry[0]+"/"+entry[1], func(t *testing.T) {
			CompareToGolden(t, response, golden)
		})
	}
	if markers == 0 {
		t.Fatal("no golden carried a volatile marker, so the volatile rule went unexercised")
	}
}

func TestAGoldenBodyIsNotAMatchForItselfUnderAVolatileKey(t *testing.T) {
	golden := LoadGolden(t, "title", "movie-details-minimal")
	if !bytes.Contains(golden.Body, []byte(`"<number>"`)) {
		t.Fatal("the golden carries no volatile marker; pick one that does")
	}
	recorder := newRecordingTB(t)
	CompareToGolden(recorder, golden.Body, golden)
	failures := recorder.recorded()
	if len(failures) != 1 || !strings.Contains(failures[0], ": expected number, got string") {
		t.Fatalf("comparing the literal body: %q, want one failure naming a marker read as a string", failures)
	}
}

func TestEveryFixtureACommittedGoldenListsLoads(t *testing.T) {
	for _, entry := range committedGoldens(t) {
		golden := LoadGolden(t, entry[0], entry[1])
		if got := loadTMDBFixtures(t, golden.Fixtures); len(got) != len(golden.Fixtures) {
			t.Errorf("%s/%s: loaded %d of %d fixtures", entry[0], entry[1], len(got), len(golden.Fixtures))
		}
	}
}

func TestLoadGoldenReturnsTheCommittedCase(t *testing.T) {
	golden := LoadGolden(t, "discover", "discover-movies-tmdb-timeout")
	want := Golden{
		Case:     "discover-movies-tmdb-timeout",
		Family:   "discover",
		Request:  "GET /api/v1/discover/movies?page=1",
		Fixtures: []string{"errors/discover-movies-timeout"},
		Status:   500,
	}
	body := golden.Body
	golden.Body = nil
	if !equalGoldenHeaders(golden, want) {
		t.Errorf("LoadGolden = %+v, want %+v", golden, want)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil || compact.String() != `{"message":"Unable to retrieve popular movies."}` {
		t.Errorf("body = %s (%v)", body, err)
	}
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "testdata", "golden", "discover", "discover-movies-tmdb-timeout.json"))
	if err != nil || !bytes.Contains(raw, body) {
		t.Errorf("Body is not the file's raw body bytes (%v)", err)
	}
}

func equalGoldenHeaders(left, right Golden) bool {
	return left.Case == right.Case && left.Family == right.Family && left.Request == right.Request &&
		left.Status == right.Status && strings.Join(left.Fixtures, ",") == strings.Join(right.Fixtures, ",")
}

func TestCompareToGoldenFailsOnEachBrokenGuard(t *testing.T) {
	golden := LoadGolden(t, "title", "movie-details-minimal")
	cases := []struct {
		name    string
		edit    func(body *jsonObject)
		failure string
	}{
		{"unchanged", func(*jsonObject) {}, ""},
		{"a volatile value changed", func(body *jsonObject) { body.set("popularity", 9999.5) }, ""},
		{"a value changed under a non-volatile key", func(body *jsonObject) { body.set("title", "Something else") }, "$.title: expected"},
		{"a type changed under a volatile key", func(body *jsonObject) { body.set("voteAverage", "8.2") }, "$.voteAverage: expected number, got string"},
		{"a field removed", func(body *jsonObject) { deleteKey(body, "title") }, "$.title: missing"},
		{"a field added", func(body *jsonObject) { body.set("posterUrl", "/p.jpg") }, "$.posterUrl: unexpected field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			markers := 0
			body := instantiateMarkers(mustDecodeOrdered(t, string(golden.Body)), &markers).(*jsonObject)
			if !body.has("title") || !body.has("popularity") || !body.has("voteAverage") {
				t.Fatal("the golden lacks a field the cases edit; pick one that has them")
			}
			tc.edit(body)
			recorder := newRecordingTB(t)
			CompareToGolden(recorder, encodeOrdered(body), golden)
			failures := recorder.recorded()
			if tc.failure == "" {
				if len(failures) != 0 {
					t.Fatalf("want a match, got %q", failures)
				}
				return
			}
			if len(failures) != 1 || !strings.Contains(failures[0], tc.failure) {
				t.Fatalf("failures %q, want one containing %q", failures, tc.failure)
			}
		})
	}
}

func TestCompareToGoldenFailsOnABodyThatIsNotJSON(t *testing.T) {
	recorder := newRecordingTB(t)
	CompareToGolden(recorder, []byte(`{"page": 1} trailing`), Golden{Case: "c", Family: "f", Body: json.RawMessage(`{"page": 1}`)})
	if failures := recorder.recorded(); len(failures) != 1 || !strings.Contains(failures[0], "is not JSON") {
		t.Fatalf("failures %q, want one saying the body is not JSON", failures)
	}
}
