package catalog

// ResultsPage is upstream's envelope for every paginated list route:
// {page, totalPages, totalResults, results}. A route whose answer carries a
// field of its own (Discover's keywords, for example) embeds it rather than
// declaring a second envelope, so the four route families share one.
type ResultsPage[T any] struct {
	Page         int `json:"page"`
	TotalPages   int `json:"totalPages"`
	TotalResults int `json:"totalResults"`
	Results      []T `json:"results"`
}

// NewResultsPage builds a page whose results are never null: the kept
// renderers iterate results without checking it first.
func NewResultsPage[T any](page, totalPages, totalResults int, results []T) ResultsPage[T] {
	if results == nil {
		results = []T{}
	}
	return ResultsPage[T]{Page: page, TotalPages: totalPages, TotalResults: totalResults, Results: results}
}
