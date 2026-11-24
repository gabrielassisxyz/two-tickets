// Package config reads the service's settings from the environment and from
// mounted secret files, and refuses to start when a required one is missing.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Environment variable names. .env.example lists the same set.
const (
	EnvListenAddr = "LISTEN_ADDR"
	EnvDataDir    = "DATA_DIR"
	EnvTMDBAPIKey = "TMDB_API_KEY"

	// secretFileSuffix names the variable that points at a mounted secret file
	// instead of carrying the value, e.g. TMDB_API_KEY_FILE.
	secretFileSuffix = "_FILE"

	defaultListenAddr = ":8080"
)

// Config is the service's validated configuration.
type Config struct {
	// ListenAddr is the host:port the HTTP server binds.
	ListenAddr string
	// DataDir holds state.db, cache.db and the pre-migration copies.
	DataDir string
	// TMDBAPIKey is the credential for The Movie Database API.
	TMDBAPIKey Secret
	// CacheMaxBytes caps cache.db before rotation. See tmdb.go.
	CacheMaxBytes int64
	// TMDBBaseURL overrides TMDB's root for the e2e stack only. Empty means
	// TMDB itself. See tmdb.go.
	TMDBBaseURL string
}

// LookupEnv has the signature of os.LookupEnv, so a test can pass a fake environment.
type LookupEnv func(key string) (string, bool)

// Load reads and validates the configuration. Every missing required variable
// is named in one error, so a broken deployment is fixed in one pass.
func Load(lookup LookupEnv) (Config, error) {
	var missing []string
	var problems []error

	cfg := Config{ListenAddr: defaultListenAddr}
	if v := envValue(lookup, EnvListenAddr); v != "" {
		cfg.ListenAddr = v
	}

	cfg.DataDir = envValue(lookup, EnvDataDir)
	switch {
	case cfg.DataDir == "":
		missing = append(missing, EnvDataDir)
	case !filepath.IsAbs(cfg.DataDir):
		// A relative path would resolve against whatever directory the process
		// happened to start in, and the state would land somewhere unexpected.
		problems = append(problems, fmt.Errorf("%s must be an absolute path", EnvDataDir))
	}

	key, err := secretValue(lookup, EnvTMDBAPIKey)
	switch {
	case err != nil:
		problems = append(problems, err)
	case key == "":
		missing = append(missing, EnvTMDBAPIKey)
	default:
		cfg.TMDBAPIKey = NewSecret(key)
	}

	// The cache cap is optional: an absent or blank value means the default.
	// Its parsing lives in tmdb.go so this file stays clear of the TMDB block.
	cacheMax, err := ParseCacheMaxBytes(lookup)
	if err != nil {
		problems = append(problems, err)
	} else {
		cfg.CacheMaxBytes = cacheMax
	}

	baseURL, err := ParseTMDBBaseURL(lookup)
	if err != nil {
		problems = append(problems, err)
	} else {
		cfg.TMDBBaseURL = baseURL
	}

	if len(missing) > 0 {
		problems = append([]error{fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))}, problems...)
	}
	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}
	return cfg, nil
}

// LogValue lists the settings for the startup log line, with secrets redacted.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("listen_addr", c.ListenAddr),
		slog.String("data_dir", c.DataDir),
		slog.Any("tmdb_api_key", c.TMDBAPIKey),
		slog.Int64("tmdb_cache_max_bytes", c.CacheMaxBytes),
		slog.String("tmdb_base_url", c.TMDBBaseURL),
	)
}

func envValue(lookup LookupEnv, name string) string {
	v, _ := lookup(name)
	return strings.TrimSpace(v)
}

// secretValue reads a secret from NAME or from the file NAME_FILE points at.
// Errors never carry the value, only the variable name and the file path.
func secretValue(lookup LookupEnv, name string) (string, error) {
	direct := envValue(lookup, name)
	fileName := name + secretFileSuffix
	path := envValue(lookup, fileName)
	if path == "" {
		return direct, nil
	}
	if direct != "" {
		return "", fmt.Errorf("set %s or %s, not both", name, fileName)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: cannot read the secret file: %w", fileName, err)
	}
	return strings.TrimSpace(string(content)), nil
}
