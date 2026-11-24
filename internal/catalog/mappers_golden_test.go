package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Helpers for the mapper tests: they read a TMDB fixture and the golden
// upstream produced from it, and compare by the rule of compareToGolden in
// web/tools/goldens/lib/golden.mjs: names, presence and JSON types
// everywhere, values everywhere except under the volatile keys.

var mapperVolatileKeys = map[string]bool{"popularity": true, "voteCount": true, "voteAverage": true}

func mapperRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package directory")
		}
		dir = parent
	}
}

// mapperFixtureBody returns the recorded TMDB body of a fixture, located
// through testdata/tmdb/index.json.
func mapperFixtureBody(t *testing.T, fixtureID string) []byte {
	t.Helper()
	root := mapperRepoRoot(t)
	var index struct {
		Fixtures []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"fixtures"`
	}
	mapperReadJSON(t, filepath.Join(root, "testdata", "tmdb", "index.json"), &index)
	for _, fixture := range index.Fixtures {
		if fixture.ID != fixtureID {
			continue
		}
		var recorded struct {
			Response struct {
				Body json.RawMessage `json:"body"`
			} `json:"response"`
		}
		mapperReadJSON(t, filepath.Join(root, filepath.FromSlash(fixture.File)), &recorded)
		return recorded.Response.Body
	}
	t.Fatalf("fixture %s is not in testdata/tmdb/index.json", fixtureID)
	return nil
}

// mapperGoldenBody returns the body of testdata/golden/<name>.json, after
// checking that upstream produced it from fixtureID.
func mapperGoldenBody(t *testing.T, name, fixtureID string) any {
	t.Helper()
	var golden struct {
		Fixtures []string `json:"fixtures"`
		Body     any      `json:"body"`
	}
	mapperReadJSON(t, filepath.Join(mapperRepoRoot(t), "testdata", "golden", filepath.FromSlash(name)+".json"), &golden)
	if !slices.Contains(golden.Fixtures, fixtureID) {
		t.Fatalf("golden %s was not produced from fixture %s (it lists %v)", name, fixtureID, golden.Fixtures)
	}
	return golden.Body
}

func mapperReadJSON(t *testing.T, path string, dest any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// mapperJSONValue is what value looks like once serialised, decoded back
// into the generic form a golden body has.
func mapperJSONValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// mapperField walks a golden body: a string step is an object key, an int
// step an array index.
func mapperField(t *testing.T, value any, path ...any) any {
	t.Helper()
	for _, step := range path {
		switch key := step.(type) {
		case string:
			object, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("golden path %v: %q is not under an object", path, key)
			}
			value = object[key]
		case int:
			array, ok := value.([]any)
			if !ok || key >= len(array) {
				t.Fatalf("golden path %v: no element %d", path, key)
			}
			value = array[key]
		}
	}
	return value
}

func mapperJSONType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		return "number"
	case string:
		return "string"
	case bool:
		return "boolean"
	}
	return fmt.Sprintf("%T", value)
}

func mapperGoldenDiffs(actual, golden any) []string {
	var diffs []string
	mapperCompareAt(actual, golden, "$", "", &diffs)
	return diffs
}

func mapperCompareAt(actual, golden any, path, key string, diffs *[]string) {
	actualType := mapperJSONType(actual)
	if mapperVolatileKeys[key] {
		expected := mapperJSONType(golden)
		if marker, ok := golden.(string); ok && len(marker) > 2 && marker[0] == '<' && marker[len(marker)-1] == '>' {
			expected = marker[1 : len(marker)-1]
		}
		if actualType != expected {
			*diffs = append(*diffs, fmt.Sprintf("%s: expected %s, got %s", path, expected, actualType))
		}
		return
	}
	if goldenType := mapperJSONType(golden); actualType != goldenType {
		*diffs = append(*diffs, fmt.Sprintf("%s: expected %s, got %s", path, goldenType, actualType))
		return
	}
	switch golden := golden.(type) {
	case map[string]any:
		object := actual.(map[string]any)
		for name := range golden {
			if _, ok := object[name]; !ok {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: missing", path, name))
			}
		}
		for name, child := range object {
			if _, ok := golden[name]; !ok {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: unexpected field", path, name))
				continue
			}
			mapperCompareAt(child, golden[name], path+"."+name, name, diffs)
		}
	case []any:
		array := actual.([]any)
		if len(array) != len(golden) {
			*diffs = append(*diffs, fmt.Sprintf("%s: expected %d items, got %d", path, len(golden), len(array)))
			return
		}
		for i := range array {
			mapperCompareAt(array[i], golden[i], fmt.Sprintf("%s[%d]", path, i), "", diffs)
		}
	default:
		if actual != golden {
			*diffs = append(*diffs, fmt.Sprintf("%s: expected %v, got %v", path, golden, actual))
		}
	}
}

func mapperRequireGoldenMatch(t *testing.T, what string, actual, golden any) {
	t.Helper()
	diffs := mapperGoldenDiffs(mapperJSONValue(t, actual), golden)
	if len(diffs) == 0 {
		return
	}
	slices.Sort(diffs)
	if len(diffs) > 15 {
		diffs = append(diffs[:15], fmt.Sprintf("... and %d more", len(diffs)-15))
	}
	t.Errorf("%s differs from upstream's golden:\n  %s", what, strings.Join(diffs, "\n  "))
}

func TestMapperGoldenDiffsFollowsTheGoldenRule(t *testing.T) {
	golden := map[string]any{"id": 1.0, "posterPath": nil, "voteCount": "<number>"}
	cases := []struct {
		name   string
		actual map[string]any
		want   string
	}{
		{"identical", map[string]any{"id": 1.0, "posterPath": nil, "voteCount": 9.0}, ""},
		{"volatile value differs", map[string]any{"id": 1.0, "posterPath": nil, "voteCount": 3.0}, ""},
		{"missing field", map[string]any{"id": 1.0, "voteCount": 9.0}, "$.posterPath: missing"},
		{"unexpected field", map[string]any{"id": 1.0, "posterPath": nil, "voteCount": 9.0, "extra": true}, "$.extra: unexpected field"},
		{"null became a string", map[string]any{"id": 1.0, "posterPath": "/a.jpg", "voteCount": 9.0}, "$.posterPath: expected null, got string"},
		{"value differs", map[string]any{"id": 2.0, "posterPath": nil, "voteCount": 9.0}, "$.id: expected 1, got 2"},
		{"volatile type differs", map[string]any{"id": 1.0, "posterPath": nil, "voteCount": "9"}, "$.voteCount: expected number, got string"},
	}
	for _, c := range cases {
		got := strings.Join(mapperGoldenDiffs(c.actual, golden), "; ")
		if got != c.want {
			t.Errorf("%s: diffs %q, want %q", c.name, got, c.want)
		}
	}
}
