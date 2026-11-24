package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// The recorded fixtures match only a request that asks exactly what upstream
// asks, so the client must send its caller's parameters and nothing of its own.
func TestGetSendsExactlyTheCallersParameters(t *testing.T) {
	var mu sync.Mutex
	var received []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.URL.Query())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"genres":[]}`))
	}))
	defer server.Close()
	client, _, _ := newTestClient(t, server.URL, testClientArgs{})

	cases := []struct {
		path   string
		params map[string]string
	}{
		{"/genre/movie/list", map[string]string{"language": DefaultLanguage}},
		{"/configuration/languages", nil},
		{"/discover/movie", map[string]string{"language": DefaultLanguage, "region": DefaultRegion, "page": "2", "with_original_language": ""}},
	}
	for i, tc := range cases {
		var out map[string]any
		if err := client.Get(context.Background(), tc.path, tc.params, &out); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		mu.Lock()
		got := received[i]
		mu.Unlock()
		want := url.Values{"api_key": {dummyAPIKey}}
		for k, v := range tc.params {
			want.Set(k, v)
		}
		if got.Encode() != want.Encode() {
			t.Errorf("%s: query = %q, want exactly %q", tc.path, got.Encode(), want.Encode())
		}
	}
}

func TestCacheKeyCarriesExactlyTheGivenParameters(t *testing.T) {
	params := map[string]string{"language": DefaultLanguage}
	got := BuildCacheKey(CacheFormatVersion, http.MethodGet, "/genre/movie/list", params)
	if got != BuildCacheKey(CacheFormatVersion, http.MethodGet, "/genre/movie/list", map[string]string{"language": DefaultLanguage}) {
		t.Fatal("the same parameters built two different keys")
	}
	withRegion := BuildCacheKey(CacheFormatVersion, http.MethodGet, "/genre/movie/list", map[string]string{"language": DefaultLanguage, "region": DefaultRegion})
	if got == withRegion {
		t.Fatal("a key without region equals one with it")
	}
}
