// Package config reads the service's settings from the environment and from
// mounted secret files, and refuses to start when a required one is missing.
package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// TMDB cache settings. They live here, apart from the core Config in
// config.go, so a parallel branch adding account settings does not collide.
const (
	// EnvCacheMaxBytes names the maximum size of cache.db before rotation.
	// Unset means the default below.
	EnvCacheMaxBytes = "TMDB_CACHE_MAX_BYTES"

	// DefaultCacheMaxBytes caps cache.db at 200 MB. The file is disposable,
	// so crossing the cap rotates it rather than failing writes.
	DefaultCacheMaxBytes int64 = 200 * 1024 * 1024

	// EnvTMDBBaseURL is a test-only override of TMDB's v3 root, so the e2e
	// stack can point the api at a substitute that answers from the recorded
	// fixtures. Unset means TMDB itself.
	EnvTMDBBaseURL = "TMDB_BASE_URL"
)

// ParseCacheMaxBytes reads the cache size cap: empty means the default, a
// plain byte count otherwise. Errors never carry a secret, only the variable
// name and the bad value's shape.
func ParseCacheMaxBytes(lookup LookupEnv) (int64, error) {
	raw := strings.TrimSpace(envValue(lookup, EnvCacheMaxBytes))
	if raw == "" {
		return DefaultCacheMaxBytes, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: must be a byte count, got %q", EnvCacheMaxBytes, raw)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %q", EnvCacheMaxBytes, raw)
	}
	return parsed, nil
}

// ParseTMDBBaseURL reads the test-only TMDB root: empty means TMDB itself,
// anything else must be an absolute http or https URL with a host, so a typo
// fails at start instead of on the first catalog call.
func ParseTMDBBaseURL(lookup LookupEnv) (string, error) {
	raw := envValue(lookup, EnvTMDBBaseURL)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%s: must be an absolute http or https URL, got %q", EnvTMDBBaseURL, raw)
	}
	return strings.TrimRight(raw, "/"), nil
}
