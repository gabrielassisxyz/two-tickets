package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// The tests start this test binary again as the real process: TestMain sees
// helperModeEnv and runs main (or runServe with a broken migration set)
// instead of the tests, so exit codes, stderr and the listener are the
// process's own.
const (
	helperModeEnv      = "TWO_TICKETS_TEST_HELPER"
	helperMain         = "main"
	helperBrokenSchema = "broken-migration"
	listenMarkerEnv    = "TWO_TICKETS_TEST_LISTEN_MARKER"

	// Low entropy on purpose: a random-looking value trips the secret scanner.
	dummySecret = "sentinelsentinelsentinel"
	processWait = 30 * time.Second
)

func TestMain(m *testing.M) {
	switch os.Getenv(helperModeEnv) {
	case "":
		os.Exit(m.Run())
	case helperMain:
		main()
	case helperBrokenSchema:
		os.Exit(runServe(context.Background(), brokenMigrationDeps()))
	}
	os.Exit(99)
}

// brokenMigrationDeps is the real startup with a migration that fails halfway
// and a listen function that leaves a marker, so the parent can tell whether
// the process ever got as far as binding the port.
func brokenMigrationDeps() serveDeps {
	deps := defaultServeDeps()
	deps.stateMigrations = fstest.MapFS{
		"0001_broken.sql": {Data: []byte("CREATE TABLE half_done (a INTEGER); INSERT INTO no_such_table VALUES (1);")},
	}
	deps.listen = func(string, string) (net.Listener, error) {
		_ = os.WriteFile(os.Getenv(listenMarkerEnv), []byte("listened\n"), 0o600)
		return nil, errors.New("test helper: the process reached listen")
	}
	return deps
}

// helperCommand builds the child with an explicit environment, so a variable
// set in the developer's shell never decides a test, and with stdin closed.
func helperCommand(ctx context.Context, mode string, env map[string]string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = []string{helperModeEnv + "=" + mode}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("process did not run: %v", err)
	}
	if !exitErr.Exited() {
		t.Fatalf("process was killed instead of exiting: %v", err)
	}
	return exitErr.ExitCode()
}

func TestBinaryRefusesToStartWithoutARequiredVariable(t *testing.T) {
	for _, missing := range []string{config.EnvDataDir, config.EnvTMDBAPIKey} {
		t.Run(missing, func(t *testing.T) {
			dataDir := t.TempDir()
			env := map[string]string{
				config.EnvDataDir:    dataDir,
				config.EnvTMDBAPIKey: dummySecret,
				config.EnvListenAddr: "127.0.0.1:0",
			}
			delete(env, missing)
			ctx, cancel := context.WithTimeout(context.Background(), processWait)
			defer cancel()

			out, err := helperCommand(ctx, helperMain, env).CombinedOutput()
			if code := exitCodeOf(t, err); code == 0 {
				t.Fatalf("exit code 0 with %s unset; output:\n%s", missing, out)
			}
			if !strings.Contains(string(out), missing) {
				t.Errorf("output does not name %s:\n%s", missing, out)
			}
			if strings.Contains(string(out), dummySecret) {
				t.Errorf("output prints the secret value:\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(dataDir, store.StateFileName)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("state.db was touched before the configuration was valid (stat: %v)", err)
			}
		})
	}
}

func TestBinaryExitsBeforeListeningWhenAMigrationFails(t *testing.T) {
	dataDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "listened")
	ctx, cancel := context.WithTimeout(context.Background(), processWait)
	defer cancel()

	out, err := helperCommand(ctx, helperBrokenSchema, map[string]string{
		config.EnvDataDir:    dataDir,
		config.EnvTMDBAPIKey: dummySecret,
		config.EnvListenAddr: "127.0.0.1:0",
		listenMarkerEnv:      marker,
	}).CombinedOutput()
	if code := exitCodeOf(t, err); code == 0 {
		t.Fatalf("exit code 0 after a failed migration; output:\n%s", out)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the process reached listen after a failed migration, so /readyz could have answered; output:\n%s", out)
	}
	if !strings.Contains(string(out), "0001_broken") {
		t.Errorf("output does not name the failing migration:\n%s", out)
	}
}

// syncBuffer collects the child's stderr while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// watchForListenAddr copies the child's log into logs and sends the address
// from the "listening" line once it appears.
func watchForListenAddr(stderr io.Reader, logs *syncBuffer) <-chan string {
	addr := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Bytes()
			_, _ = logs.Write(line)
			_, _ = logs.Write([]byte{'\n'})
			var entry struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if json.Unmarshal(line, &entry) == nil && entry.Msg == "listening" {
				addr <- entry.Addr
			}
		}
	}()
	return addr
}

func journalModeOf(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	return mode
}

func TestBinaryStartsMigratesAndServesHealthRoutes(t *testing.T) {
	dataDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), processWait)
	defer cancel()
	cmd := helperCommand(ctx, helperMain, map[string]string{
		config.EnvDataDir:    dataDir,
		config.EnvTMDBAPIKey: dummySecret,
		config.EnvListenAddr: "127.0.0.1:0",
	})
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	var logs syncBuffer
	addrCh := watchForListenAddr(stderr, &logs)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	var addr string
	select {
	case addr = <-addrCh:
	case <-ctx.Done():
		t.Fatalf("the process never logged its listen address:\n%s", logs.String())
	}

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || len(body) != 0 {
			t.Errorf("GET %s = %d %q, want 200 with an empty body", path, resp.StatusCode, body)
		}
	}
	for _, name := range []string{store.StateFileName, store.CacheFileName} {
		if mode := journalModeOf(t, filepath.Join(dataDir, name)); mode != "wal" {
			t.Errorf("%s journal_mode = %q, want wal", name, mode)
		}
	}
	if _, err := os.Stat(store.RollbackCopyPath(filepath.Join(dataDir, store.RollbackDirName), 0)); err != nil {
		t.Errorf("first migration left no rollback copy: %v", err)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := exitCodeOf(t, cmd.Wait()); code != 0 {
		t.Errorf("exit code after SIGTERM = %d, want 0:\n%s", code, logs.String())
	}
	if strings.Contains(logs.String(), dummySecret) {
		t.Errorf("the log prints the secret value:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), `"tmdb_api_key":"[REDACTED]"`) {
		t.Errorf("the startup log does not show the key as redacted:\n%s", logs.String())
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	var stderr bytes.Buffer
	if code := dispatchCommand([]string{"frobnicate"}, &stderr); code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "frobnicate") {
		t.Errorf("usage error does not name the command: %q", stderr.String())
	}
}
