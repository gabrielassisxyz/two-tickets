package catalogtest

import (
	"reflect"
	"testing"
)

// The cases below are lib/golden.test.mjs, one Go test per JavaScript one,
// asserting the same verdict. The JavaScript fixtures are object literals;
// here they are JSON text, edited with the same operations the JavaScript
// spread and delete perform.

const ruleMovieJSON = `{
  "id": 603,
  "mediaType": "movie",
  "title": "Matrix",
  "posterPath": "/poster.jpg",
  "backdropPath": null,
  "popularity": 41.5,
  "voteCount": 27000,
  "voteAverage": 8.2,
  "releases": {"results": [{"iso_3166_1": "BR", "release_dates": []}]}
}`

func mustDecodeOrdered(t *testing.T, text string) any {
	t.Helper()
	value, err := decodeOrderedJSON([]byte(text))
	if err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	return value
}

func ruleGolden(t *testing.T) *jsonObject {
	t.Helper()
	return normalizeForGolden(mustDecodeOrdered(t, `{"page": 1, "results": [`+ruleMovieJSON+`]}`)).(*jsonObject)
}

func firstResult(page *jsonObject) *jsonObject {
	return page.values["results"].([]any)[0].(*jsonObject)
}

// withMovie is the JavaScript ({ page: 1, results: [{ ...movie, ...changes }] }):
// a changed key keeps its place, a new one is appended.
func withMovie(t *testing.T, changes ...any) *jsonObject {
	t.Helper()
	movie := mustDecodeOrdered(t, ruleMovieJSON).(*jsonObject)
	for i := 0; i < len(changes); i += 2 {
		movie.set(changes[i].(string), changes[i+1])
	}
	page := newJSONObject()
	page.set("page", 1.0)
	page.set("results", []any{movie})
	return page
}

// deleteKey is the JavaScript delete operator.
func deleteKey(object *jsonObject, key string) {
	delete(object.values, key)
	for i, existing := range object.keys {
		if existing == key {
			object.keys = append(object.keys[:i], object.keys[i+1:]...)
			return
		}
	}
}

func assertDifferences(t *testing.T, actual, golden any, want ...string) {
	t.Helper()
	got := compareToGolden(actual, golden)
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("differences:\n got  %q\n want %q", got, want)
	}
}

func TestNormalizeReplacesVolatileValuesWithTheirJSONType(t *testing.T) {
	result := firstResult(ruleGolden(t))
	for _, key := range []string{"popularity", "voteCount", "voteAverage"} {
		if got := result.values[key]; got != "<number>" {
			t.Errorf("%s = %#v, want \"<number>\"", key, got)
		}
	}
}

func TestNormalizeKeepsEveryOtherValueAtAnyDepth(t *testing.T) {
	result := firstResult(ruleGolden(t))
	if got := result.values["title"]; got != "Matrix" {
		t.Errorf("title = %#v, want \"Matrix\"", got)
	}
	if got, present := result.values["backdropPath"]; !present || got != nil {
		t.Errorf("backdropPath = %#v (present %v), want null", got, present)
	}
	movie := mustDecodeOrdered(t, ruleMovieJSON).(*jsonObject)
	if !reflect.DeepEqual(result.values["releases"], movie.values["releases"]) {
		t.Errorf("releases = %#v, want %#v", result.values["releases"], movie.values["releases"])
	}
}

func TestNormalizeRecordsANullVolatileValueAsNull(t *testing.T) {
	got := normalizeForGolden(mustDecodeOrdered(t, `{"popularity": null}`))
	want := mustDecodeOrdered(t, `{"popularity": "<null>"}`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalized = %#v, want %#v", got, want)
	}
}

func TestCompareMatchesTheResponseTheGoldenWasMadeFrom(t *testing.T) {
	assertDifferences(t, withMovie(t), ruleGolden(t))
}

func TestCompareIgnoresTheValuesOfVolatileFields(t *testing.T) {
	drifted := withMovie(t, "popularity", 3.1, "voteCount", 1.0, "voteAverage", 0.0)
	assertDifferences(t, drifted, ruleGolden(t))
}

func TestCompareReportsAVolatileFieldThatIsMissing(t *testing.T) {
	response := withMovie(t)
	deleteKey(firstResult(response), "voteCount")
	assertDifferences(t, response, ruleGolden(t), "$.results[0].voteCount: missing")
}

func TestCompareReportsAVolatileFieldWhoseTypeChanged(t *testing.T) {
	assertDifferences(t, withMovie(t, "voteAverage", "8.2"), ruleGolden(t),
		"$.results[0].voteAverage: expected number, got string")
}

func TestCompareReportsAMissingFieldAndAnUnexpectedOne(t *testing.T) {
	response := withMovie(t, "posterUrl", "/poster.jpg")
	deleteKey(firstResult(response), "posterPath")
	assertDifferences(t, response, ruleGolden(t),
		"$.results[0].posterPath: missing",
		"$.results[0].posterUrl: unexpected field")
}

func TestCompareTellsNullFromAbsent(t *testing.T) {
	response := withMovie(t)
	deleteKey(firstResult(response), "backdropPath")
	assertDifferences(t, response, ruleGolden(t), "$.results[0].backdropPath: missing")
}

func TestCompareReportsAChangedTypeOutsideVolatileFields(t *testing.T) {
	assertDifferences(t, withMovie(t, "backdropPath", "/b.jpg"), ruleGolden(t),
		"$.results[0].backdropPath: expected null, got string")
}

func TestCompareComparesValuesOutsideVolatileFields(t *testing.T) {
	assertDifferences(t, withMovie(t, "title", "The Matrix"), ruleGolden(t),
		`$.results[0].title: expected "Matrix", got "The Matrix"`)
}

func TestCompareReportsAnArrayOfADifferentLength(t *testing.T) {
	assertDifferences(t, mustDecodeOrdered(t, `{"page": 1, "results": []}`), ruleGolden(t),
		"$.results: expected 1 items, got 0")
}

func TestCompareKeepsSnakeCaseKeysNestedInCamelCaseObjectsExact(t *testing.T) {
	releases := mustDecodeOrdered(t, `{"results": [{"iso3166_1": "BR", "release_dates": []}]}`)
	assertDifferences(t, withMovie(t, "releases", releases), ruleGolden(t),
		"$.results[0].releases.results[0].iso_3166_1: missing",
		"$.results[0].releases.results[0].iso3166_1: unexpected field")
}

// The cases below pin where Go and JavaScript would otherwise part ways: key
// order, number and string printing. Each expected string is what node prints
// for the same input under golden.mjs.

func TestCompareVisitsKeysInObjectKeysOrder(t *testing.T) {
	golden := mustDecodeOrdered(t, `{"b": 1, "10": 1, "2": 1, "a": 1, "02": 1}`)
	assertDifferences(t, mustDecodeOrdered(t, `{}`), golden,
		"$.2: missing", "$.10: missing", "$.b: missing", "$.a: missing", "$.02: missing")
}

func TestCompareKeepsTheLastValueOfARepeatedKeyAtItsFirstPlace(t *testing.T) {
	actual := mustDecodeOrdered(t, `{"a": 1, "b": 2, "a": 3}`)
	assertDifferences(t, actual, mustDecodeOrdered(t, `{"b": 0, "a": 0}`),
		"$.a: expected 0, got 3", "$.b: expected 0, got 2")
}

func TestCompareTreatsEqualNumbersAsEqualWhateverTheirSpelling(t *testing.T) {
	assertDifferences(t, mustDecodeOrdered(t, `[1.0, -0, 1e2]`), mustDecodeOrdered(t, `[1, 0, 100]`))
}

func TestComparePrintsNumbersAsJavaScriptDoes(t *testing.T) {
	assertDifferences(t,
		mustDecodeOrdered(t, `[1e21, 1.5e-7, 0.000001, 123456789012345680000, -2.5, 0.1]`),
		mustDecodeOrdered(t, `[0, 0, 0, 0, 0, 0]`),
		"$[0]: expected 0, got 1e+21",
		"$[1]: expected 0, got 1.5e-7",
		"$[2]: expected 0, got 0.000001",
		"$[3]: expected 0, got 123456789012345680000",
		"$[4]: expected 0, got -2.5",
		"$[5]: expected 0, got 0.1")
}

func TestComparePrintsStringsAndBooleansAsJavaScriptDoes(t *testing.T) {
	assertDifferences(t,
		mustDecodeOrdered(t, `["tab\tquote\"back\\slash\u0001 <&>   ação", true]`),
		mustDecodeOrdered(t, `["", false]`),
		`$[0]: expected "", got "tab\tquote\"back\\slash\u0001 <&> `+" "+` ação"`,
		"$[1]: expected false, got true")
}

func TestCompareReadsAPlainValueUnderAVolatileKeyAsItsType(t *testing.T) {
	golden := mustDecodeOrdered(t, `{"popularity": 1, "voteCount": "<string>", "voteAverage": "high"}`)
	actual := mustDecodeOrdered(t, `{"popularity": "x", "voteCount": "y", "voteAverage": 2}`)
	assertDifferences(t, actual, golden,
		"$.popularity: expected number, got string",
		"$.voteAverage: expected string, got number")
}

func TestCompareChecksVolatileKeysOnlyAsObjectKeys(t *testing.T) {
	assertDifferences(t, mustDecodeOrdered(t, `["popularity", 2]`), mustDecodeOrdered(t, `["popularity", 1]`),
		"$[1]: expected 1, got 2")
}
