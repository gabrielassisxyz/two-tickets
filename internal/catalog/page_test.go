package catalog

import (
	"encoding/json"
	"testing"
)

func TestResultsPageHasUpstreamsEnvelopeKeys(t *testing.T) {
	body, err := json.Marshal(NewResultsPage(2, 7, 133, []int{1, 2}))
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"page":2,"totalPages":7,"totalResults":133,"results":[1,2]}`
	if string(body) != want {
		t.Fatalf("page = %s, want %s", body, want)
	}
}

func TestResultsPageNeverAnswersNullResults(t *testing.T) {
	body, err := json.Marshal(NewResultsPage[int](1, 0, 0, nil))
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"page":1,"totalPages":0,"totalResults":0,"results":[]}`
	if string(body) != want {
		t.Fatalf("empty page = %s, want %s", body, want)
	}
}

// A route with a field of its own embeds the envelope; its keys stay flat.
func TestResultsPageEmbedsIntoARouteOwnAnswer(t *testing.T) {
	type withKeywords struct {
		ResultsPage[int]
		Keywords []string `json:"keywords"`
	}
	body, err := json.Marshal(withKeywords{ResultsPage: NewResultsPage(1, 1, 1, []int{9}), Keywords: []string{"heist"}})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"page":1,"totalPages":1,"totalResults":1,"results":[9],"keywords":["heist"]}`
	if string(body) != want {
		t.Fatalf("embedded page = %s, want %s", body, want)
	}
}
