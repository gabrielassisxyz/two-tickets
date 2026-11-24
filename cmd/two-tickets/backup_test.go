package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// backupSeedStateFile creates a DATA_DIR holding a live state.db with one
// title, and returns the directory.
func backupSeedStateFile(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	db, err := store.OpenDatabase(context.Background(), filepath.Join(dataDir, store.StateFileName))
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	if _, err := db.Write.Exec("CREATE TABLE backup_title (id INTEGER PRIMARY KEY, name TEXT NOT NULL);" +
		"INSERT INTO backup_title (id, name) VALUES (1, 'Central do Brasil');"); err != nil {
		t.Fatalf("seed state.db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close state.db: %v", err)
	}
	return dataDir
}

// backupArtifactTitle reads the title back out of the published artifact.
func backupArtifactTitle(t *testing.T, target string) string {
	t.Helper()
	copyDB, err := sql.Open("sqlite", "file:"+target+"?mode=ro")
	if err != nil {
		t.Fatalf("open backup artifact: %v", err)
	}
	defer func() { _ = copyDB.Close() }()
	var name string
	if err := copyDB.QueryRow("SELECT name FROM backup_title WHERE id = 1").Scan(&name); err != nil {
		t.Fatalf("read backup artifact: %v", err)
	}
	var integrity string
	if err := copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, err = %v", integrity, err)
	}
	return name
}

func TestBackupSubcommandPublishesTheArtifact(t *testing.T) {
	dataDir := backupSeedStateFile(t)
	t.Setenv("DATA_DIR", dataDir)
	target := filepath.Join(t.TempDir(), "state.db")

	var stderr bytes.Buffer
	if code := dispatchCommand([]string{"backup", "--target", target}, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0: %s", code, stderr.String())
	}
	if got := backupArtifactTitle(t, target); got != "Central do Brasil" {
		t.Fatalf("backup title = %q, want the seeded title", got)
	}
}

func TestBackupSubcommandNeedsDataDir(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	var stderr bytes.Buffer
	if code := dispatchCommand([]string{"backup", "--target", filepath.Join(t.TempDir(), "state.db")}, &stderr); code == 0 {
		t.Fatal("backup succeeded without DATA_DIR")
	}
	if !strings.Contains(stderr.String(), "DATA_DIR") {
		t.Errorf("output does not name DATA_DIR: %q", stderr.String())
	}
}

func TestBackupSubcommandRefusesACorruptSource(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, store.StateFileName), []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	target := filepath.Join(t.TempDir(), "state.db")

	var stderr bytes.Buffer
	if code := dispatchCommand([]string{"backup", "--target", target}, &stderr); code != exitFailure {
		t.Fatalf("exit code = %d, want %d: %s", code, exitFailure, stderr.String())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target exists after a failed backup (stat: %v)", err)
	}
}

func TestBackupSubcommandRejectsAPositionalArgument(t *testing.T) {
	dataDir := backupSeedStateFile(t)
	t.Setenv("DATA_DIR", dataDir)

	var stderr bytes.Buffer
	if code := dispatchCommand([]string{"backup", "extra"}, &stderr); code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}
