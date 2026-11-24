package catalogtest

import (
	"fmt"
	"strings"
	"testing"
)

// This file is a port of compareToGolden and normalizeForGolden in
// web/tools/goldens/lib/golden.mjs, which produced the committed goldens. The
// two must give the same verdict on the same input, down to the order and
// wording of the differences; compare_test.go ports that file's unit tests.

// goldenVolatileKeys are the fields whose values TMDB changes daily. A golden
// records their presence and JSON type, never their value.
var goldenVolatileKeys = map[string]bool{
	"popularity":  true,
	"voteCount":   true,
	"voteAverage": true,
}

// CompareToGolden fails t, listing every difference, unless gotBody matches
// the golden's body: field names, presence (an absent field and a null one
// differ) and JSON types everywhere, and values everywhere except under
// popularity, voteCount and voteAverage. It compares the body only; the
// status is the caller's to check against golden.Status.
func CompareToGolden(t testing.TB, gotBody []byte, golden Golden) {
	t.Helper()
	actual, err := decodeOrderedJSON(gotBody)
	if err != nil {
		t.Errorf("golden %s/%s: the response body is not JSON: %v\n%s", golden.Family, golden.Case, err, gotBody)
		return
	}
	expected, err := decodeOrderedJSON(golden.Body)
	if err != nil {
		t.Errorf("golden %s/%s: the golden body is not JSON: %v", golden.Family, golden.Case, err)
		return
	}
	if differences := compareToGolden(actual, expected); len(differences) > 0 {
		t.Errorf("response differs from golden %s/%s:\n  %s", golden.Family, golden.Case, strings.Join(differences, "\n  "))
	}
}

// normalizeForGolden replaces the value under every volatile key with a
// marker naming its JSON type ("<number>"), at any depth.
func normalizeForGolden(value any) any {
	switch value := value.(type) {
	case []any:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = normalizeForGolden(item)
		}
		return items
	case *jsonObject:
		normalized := newJSONObject()
		for _, key := range value.objectKeys() {
			child := value.values[key]
			if goldenVolatileKeys[key] {
				normalized.set(key, "<"+jsonTypeOf(child)+">")
			} else {
				normalized.set(key, normalizeForGolden(child))
			}
		}
		return normalized
	default:
		return value
	}
}

// compareToGolden returns every difference as "<path>: <what differs>"; an
// empty result is a match.
func compareToGolden(actual, golden any) []string {
	differences := []string{}
	compareGoldenAt(actual, golden, "$", "", &differences)
	return differences
}

// expectedVolatileType reads the type a golden records under a volatile key:
// the marker's type, or the type of a plain value.
func expectedVolatileType(golden any) string {
	if marker, isString := golden.(string); isString {
		switch marker {
		case "<null>", "<boolean>", "<number>", "<string>", "<array>", "<object>":
			return marker[1 : len(marker)-1]
		}
	}
	return jsonTypeOf(golden)
}

// compareGoldenAt compares one node. key is the object key the node sits
// under, or "" for the root and array items; "" is never a volatile key, so it
// stands in for the JavaScript rule's null.
func compareGoldenAt(actual, golden any, path, key string, differences *[]string) {
	actualType := jsonTypeOf(actual)
	if goldenVolatileKeys[key] {
		if expected := expectedVolatileType(golden); actualType != expected {
			*differences = append(*differences, fmt.Sprintf("%s: expected %s, got %s", path, expected, actualType))
		}
		return
	}
	goldenType := jsonTypeOf(golden)
	if actualType != goldenType {
		*differences = append(*differences, fmt.Sprintf("%s: expected %s, got %s", path, goldenType, actualType))
		return
	}
	switch goldenType {
	case "object":
		compareGoldenObjects(actual.(*jsonObject), golden.(*jsonObject), path, differences)
	case "array":
		compareGoldenArrays(actual.([]any), golden.([]any), path, differences)
	default:
		if actual != golden {
			*differences = append(*differences, fmt.Sprintf("%s: expected %s, got %s", path, stringifyPrimitive(golden), stringifyPrimitive(actual)))
		}
	}
}

func compareGoldenObjects(actual, golden *jsonObject, path string, differences *[]string) {
	for _, key := range golden.objectKeys() {
		if !actual.has(key) {
			*differences = append(*differences, fmt.Sprintf("%s.%s: missing", path, key))
		}
	}
	for _, key := range actual.objectKeys() {
		if !golden.has(key) {
			*differences = append(*differences, fmt.Sprintf("%s.%s: unexpected field", path, key))
			continue
		}
		compareGoldenAt(actual.values[key], golden.values[key], path+"."+key, key, differences)
	}
}

func compareGoldenArrays(actual, golden []any, path string, differences *[]string) {
	if len(actual) != len(golden) {
		*differences = append(*differences, fmt.Sprintf("%s: expected %d items, got %d", path, len(golden), len(actual)))
		return
	}
	for i, item := range actual {
		compareGoldenAt(item, golden[i], fmt.Sprintf("%s[%d]", path, i), "", differences)
	}
}
