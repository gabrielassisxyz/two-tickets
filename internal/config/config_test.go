package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Low entropy on purpose: a random-looking value trips the secret scanner.
const dummySecret = "sentinelsentinelsentinel"

func fakeEnv(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(fakeEnv(map[string]string{
		EnvListenAddr: "127.0.0.1:9000",
		EnvDataDir:    dir,
		EnvTMDBAPIKey: dummySecret,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9000" || cfg.DataDir != dir || cfg.TMDBAPIKey.Reveal() != dummySecret {
		t.Fatalf("unexpected config: listen=%q dir=%q key-matches=%v", cfg.ListenAddr, cfg.DataDir, cfg.TMDBAPIKey.Reveal() == dummySecret)
	}
}

func TestLoadDefaultsTheListenAddress(t *testing.T) {
	cfg, err := Load(fakeEnv(map[string]string{EnvDataDir: t.TempDir(), EnvTMDBAPIKey: dummySecret}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != defaultListenAddr {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, defaultListenAddr)
	}
}

func TestLoadNamesEveryMissingVariable(t *testing.T) {
	cases := map[string]struct {
		env     map[string]string
		missing []string
	}{
		"data dir":   {map[string]string{EnvTMDBAPIKey: dummySecret}, []string{EnvDataDir}},
		"tmdb key":   {map[string]string{EnvDataDir: "/srv/x"}, []string{EnvTMDBAPIKey}},
		"blank key":  {map[string]string{EnvDataDir: "/srv/x", EnvTMDBAPIKey: "  "}, []string{EnvTMDBAPIKey}},
		"everything": {map[string]string{}, []string{EnvDataDir, EnvTMDBAPIKey}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(fakeEnv(tc.env))
			if err == nil {
				t.Fatal("Load succeeded with a required variable missing")
			}
			for _, v := range tc.missing {
				if !strings.Contains(err.Error(), v) {
					t.Errorf("error %q does not name %s", err, v)
				}
			}
			if strings.Contains(err.Error(), dummySecret) {
				t.Errorf("error leaks the secret: %q", err)
			}
		})
	}
}

func TestLoadRejectsARelativeDataDir(t *testing.T) {
	_, err := Load(fakeEnv(map[string]string{EnvDataDir: "data", EnvTMDBAPIKey: dummySecret}))
	if err == nil || !strings.Contains(err.Error(), EnvDataDir) {
		t.Fatalf("Load error = %v, want one naming %s", err, EnvDataDir)
	}
}

func TestLoadReadsTheSecretFromAMountedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tmdb_api_key")
	if err := os.WriteFile(path, []byte(dummySecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(fakeEnv(map[string]string{EnvDataDir: t.TempDir(), EnvTMDBAPIKey + "_FILE": path}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TMDBAPIKey.Reveal() != dummySecret {
		t.Fatal("the secret read from the file does not match its content")
	}
}

func TestLoadRefusesBothTheSecretAndItsFile(t *testing.T) {
	_, err := Load(fakeEnv(map[string]string{
		EnvDataDir:              "/srv/x",
		EnvTMDBAPIKey:           dummySecret,
		EnvTMDBAPIKey + "_FILE": "/run/secrets/tmdb",
	}))
	if err == nil {
		t.Fatal("Load accepted both TMDB_API_KEY and TMDB_API_KEY_FILE")
	}
	if strings.Contains(err.Error(), dummySecret) {
		t.Errorf("error leaks the secret: %q", err)
	}
}

func TestLoadReportsAnUnreadableSecretFile(t *testing.T) {
	_, err := Load(fakeEnv(map[string]string{
		EnvDataDir:              "/srv/x",
		EnvTMDBAPIKey + "_FILE": filepath.Join(t.TempDir(), "absent"),
	}))
	if err == nil || !strings.Contains(err.Error(), EnvTMDBAPIKey+"_FILE") {
		t.Fatalf("Load error = %v, want one naming %s_FILE", err, EnvTMDBAPIKey)
	}
}

// Every path a Config or a Secret can take into text: fmt verbs, slog's two
// stock handlers and encoding/json.
func TestSecretIsRedactedEverywhere(t *testing.T) {
	cfg := Config{ListenAddr: ":8080", DataDir: "/srv/x", TMDBAPIKey: NewSecret(dummySecret)}
	var rendered []string
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		rendered = append(rendered, fmt.Sprintf(verb, cfg), fmt.Sprintf(verb, cfg.TMDBAPIKey))
	}
	rendered = append(rendered, fmt.Sprint(cfg.TMDBAPIKey), cfg.TMDBAPIKey.String())

	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("x", "config", cfg, "key", cfg.TMDBAPIKey)
	slog.New(slog.NewTextHandler(&logs, nil)).Info("x", "config", cfg, "key", cfg.TMDBAPIKey)
	rendered = append(rendered, logs.String())

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rendered = append(rendered, string(encoded))

	hexSecret := fmt.Sprintf("%x", dummySecret)
	for _, text := range rendered {
		if strings.Contains(text, dummySecret) || strings.Contains(strings.ToLower(text), hexSecret) {
			t.Errorf("secret leaked into %q", text)
		}
	}
	if !strings.Contains(logs.String(), redactedText) {
		t.Errorf("log output carries no redaction marker: %q", logs.String())
	}
}
