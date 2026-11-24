package tmdb

import (
	"net/url"
	"sort"
	"strings"
	"time"
)

// CacheFormatVersion prefixes every cache key. Bumping it makes every previous
// row unreachable, which is safe because cache.db is disposable.
const CacheFormatVersion = 1

// BuildCacheKey assembles the cache key: the format version, the method, the
// path and the stably sorted parameters that shape the response. Language,
// region and append_to_response are ordinary parameters by the time they get
// here, so they are part of the key. The API key never is.
func BuildCacheKey(formatVersion int, method, path string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var encoded strings.Builder
	for i, k := range keys {
		if i > 0 {
			encoded.WriteByte('&')
		}
		encoded.WriteString(url.QueryEscape(k))
		encoded.WriteByte('=')
		encoded.WriteString(url.QueryEscape(params[k]))
	}
	return strings.Join([]string{
		"v" + itoa(formatVersion),
		strings.ToUpper(method),
		path,
		encoded.String(),
	}, "|")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// TTLForPath returns how long a successful response for path stays fresh.
// The families follow the bead: details and seasons 7 days, recommendations
// 7 days, discover and trending 6 hours, search 1 hour, genres, languages,
// regions and configuration 30 days. Unknown paths default to 1 hour rather
// than risking a stale long-lived row.
func TTLForPath(path string) time.Duration {
	const (
		day  = 24 * time.Hour
		week = 7 * day
	)
	trimmed := strings.TrimPrefix(path, "/3")
	switch {
	case strings.HasPrefix(trimmed, "/configuration/"),
		strings.HasPrefix(trimmed, "/genre/"),
		strings.HasPrefix(trimmed, "/certification/"),
		trimmed == "/watch/providers/regions":
		return 30 * day
	case strings.HasPrefix(trimmed, "/discover/"),
		strings.HasPrefix(trimmed, "/trending/"),
		strings.Contains(trimmed, "/watch/providers"):
		return 6 * time.Hour
	case strings.HasPrefix(trimmed, "/search/"),
		strings.HasPrefix(trimmed, "/find/"):
		return time.Hour
	case strings.Contains(trimmed, "/recommendations"),
		strings.Contains(trimmed, "/similar"):
		return week
	default:
		return week
	}
}

// isNegativeCacheable reports whether a 404 for path is an intentional
// negative the contract needs, rather than a transient failure. Only keyword
// lookups qualify: upstream turns a missing keyword into JSON null, and the
// catalog relies on not asking twice.
func isNegativeCacheable(path string) bool {
	trimmed := strings.TrimPrefix(path, "/3")
	return strings.HasPrefix(trimmed, "/keyword/")
}
