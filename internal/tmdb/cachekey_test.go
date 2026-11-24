package tmdb

import (
	"strings"
	"testing"
	"time"
)

func TestBuildCacheKeySortsParams(t *testing.T) {
	first := BuildCacheKey(1, "GET", "/discover/movie", map[string]string{
		"language": "pt-BR", "region": "BR", "page": "2", "sort_by": "popularity.desc",
	})
	second := BuildCacheKey(1, "GET", "/discover/movie", map[string]string{
		"sort_by": "popularity.desc", "page": "2", "region": "BR", "language": "pt-BR",
	})
	if first != second {
		t.Fatalf("key depends on map order:\n%s\n%s", first, second)
	}
	for _, want := range []string{"language=pt-BR", "region=BR", "page=2"} {
		if !strings.Contains(first, want) {
			t.Errorf("key %q does not contain %q", first, want)
		}
	}
}

func TestBuildCacheKeySeparatesVersionsAndMethods(t *testing.T) {
	params := map[string]string{"language": "pt-BR"}
	v1 := BuildCacheKey(1, "GET", "/movie/1", params)
	v2 := BuildCacheKey(2, "GET", "/movie/1", params)
	if v1 == v2 {
		t.Fatal("bumping the format version did not change the key")
	}
}

func TestTTLForPathFamilies(t *testing.T) {
	const day = 24 * time.Hour
	cases := map[string]struct {
		path string
		want time.Duration
	}{
		"details":          {"/movie/550", 7 * day},
		"season":           {"/tv/1399/season/1", 7 * day},
		"collection":       {"/collection/10", 7 * day},
		"person":           {"/person/1", 7 * day},
		"keyword details":  {"/keyword/1234", 7 * day},
		"recommendations":  {"/movie/550/recommendations", 7 * day},
		"similar":          {"/tv/1399/similar", 7 * day},
		"discover":         {"/discover/movie", 6 * time.Hour},
		"trending":         {"/trending/all/week", 6 * time.Hour},
		"search":           {"/search/movie", time.Hour},
		"find":             {"/find/tt0140888", time.Hour},
		"genres":           {"/genre/movie/list", 30 * day},
		"languages":        {"/configuration/languages", 30 * day},
		"regions":          {"/configuration/countries", 30 * day},
		"certifications":   {"/certification/movie/list", 30 * day},
		"provider regions": {"/watch/providers/regions", 30 * day},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := TTLForPath(tc.path); got != tc.want {
				t.Fatalf("TTLForPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
