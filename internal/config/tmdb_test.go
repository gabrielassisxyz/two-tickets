package config

import (
	"testing"
)

func TestParseCacheMaxBytesDefaultsWhenBlank(t *testing.T) {
	for _, env := range []map[string]string{{}, {EnvCacheMaxBytes: "  "}} {
		got, err := ParseCacheMaxBytes(fakeEnv(env))
		if err != nil {
			t.Fatalf("ParseCacheMaxBytes: %v", err)
		}
		if got != DefaultCacheMaxBytes {
			t.Fatalf("got %d, want default %d", got, DefaultCacheMaxBytes)
		}
	}
}

func TestParseCacheMaxBytesReadsAByteCount(t *testing.T) {
	got, err := ParseCacheMaxBytes(fakeEnv(map[string]string{EnvCacheMaxBytes: "4096"}))
	if err != nil {
		t.Fatalf("ParseCacheMaxBytes: %v", err)
	}
	if got != 4096 {
		t.Fatalf("got %d, want 4096", got)
	}
}

func TestParseCacheMaxBytesRejectsBadValues(t *testing.T) {
	for _, raw := range []string{"zero", "0", "-10", "12.5", "10MB"} {
		if _, err := ParseCacheMaxBytes(fakeEnv(map[string]string{EnvCacheMaxBytes: raw})); err == nil {
			t.Fatalf("ParseCacheMaxBytes accepted %q", raw)
		}
	}
}

func TestLoadDefaultsTheCacheCap(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{EnvDataDir: t.TempDir(), EnvTMDBAPIKey: dummySecret}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CacheMaxBytes != DefaultCacheMaxBytes {
		t.Fatalf("CacheMaxBytes = %d, want default %d", cfg.CacheMaxBytes, DefaultCacheMaxBytes)
	}
}

func TestLoadReadsTheCacheCap(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{
		EnvDataDir:       t.TempDir(),
		EnvTMDBAPIKey:    dummySecret,
		EnvCacheMaxBytes: "12345",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CacheMaxBytes != 12345 {
		t.Fatalf("CacheMaxBytes = %d, want 12345", cfg.CacheMaxBytes)
	}
}

func TestLoadLeavesTheTMDBBaseURLEmptyByDefault(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{EnvDataDir: t.TempDir(), EnvTMDBAPIKey: dummySecret}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TMDBBaseURL != "" {
		t.Fatalf("TMDBBaseURL = %q, want empty (TMDB itself)", cfg.TMDBBaseURL)
	}
}

func TestLoadReadsTheTMDBBaseURLOverride(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{
		EnvDataDir:     t.TempDir(),
		EnvTMDBAPIKey:  dummySecret,
		EnvTMDBBaseURL: "http://tmdb:8080/3/",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TMDBBaseURL != "http://tmdb:8080/3" {
		t.Fatalf("TMDBBaseURL = %q, want http://tmdb:8080/3", cfg.TMDBBaseURL)
	}
}

func TestLoadRejectsATMDBBaseURLThatIsNotAnAbsoluteHTTPURL(t *testing.T) {
	for _, raw := range []string{"tmdb:8080", "/3", "ftp://tmdb/3", "http://"} {
		_, err := Load(fakeEnv(map[string]string{
			EnvDataDir:     t.TempDir(),
			EnvTMDBAPIKey:  dummySecret,
			EnvTMDBBaseURL: raw,
		}))
		if err == nil {
			t.Fatalf("Load accepted %s=%q", EnvTMDBBaseURL, raw)
		}
	}
}
