package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const librarySchemaVersion = 3

var libraryTables = []string{"watches", "ratings", "lists", "list_items"}

// librarySchemaTwoFixture is a state.db as the previous release leaves it:
// schema 2, both accounts and a live session.
type librarySchemaTwoFixture struct {
	dir, path, rollbackDir string
	db                     *Database
}

func newLibrarySchemaTwoFixture(t *testing.T) *librarySchemaTwoFixture {
	t.Helper()
	dir := t.TempDir()
	f := &librarySchemaTwoFixture{dir: dir, path: filepath.Join(dir, StateFileName), rollbackDir: filepath.Join(dir, RollbackDirName)}
	f.db = openTestDatabase(t, f.path)
	migrations := libraryEmbeddedMigrations(t)
	if len(migrations) < librarySchemaVersion || migrations[librarySchemaVersion-1].Name != "library" {
		t.Fatalf("embedded migrations = %d, want 0003_library among them", len(migrations))
	}
	if err := MigrateState(context.Background(), f.db, migrations[:2], f.rollbackDir); err != nil {
		t.Fatalf("migrate to schema 2: %v", err)
	}
	libraryInsertUser(t, f.db.Write, libraryUserA)
	libraryInsertUser(t, f.db.Write, libraryUserB)
	if _, err := f.db.Write.Exec("INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		bytes.Repeat([]byte{7}, 32), libraryUserA, libraryFixtureTime, libraryFixtureTime, libraryFixtureTime+86400); err != nil {
		t.Fatal(err)
	}
	return f
}

// libraryAccountRows is every users and sessions row, rendered for comparison.
func libraryAccountRows(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var out []string
	for _, query := range []string{
		"SELECT printf('user %d %s %s %s %s %s %d %d', id, config_key, username, email, display_name, password_hash, created_at, updated_at) FROM users ORDER BY id",
		"SELECT printf('session %s %d %d %d %d', hex(token_hash), user_id, created_at, last_seen_at, expires_at) FROM sessions ORDER BY token_hash",
	} {
		rows, err := db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			out = append(out, line)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
	}
	return out
}

func libraryTablesPresent(t *testing.T, db *sql.DB) []string {
	t.Helper()
	var present []string
	for _, table := range libraryTables {
		if tableExists(t, db, table) {
			present = append(present, table)
		}
	}
	return present
}

func TestLibraryMigrationFromAnEmptyDatabase(t *testing.T) {
	db := newLibraryDatabase(t)
	// Newer migrations (0004_recs and later) raise the newest version past
	// the library's own; the library's tables must still be there.
	if got, err := schemaVersion(context.Background(), db.Write); err != nil || got < librarySchemaVersion {
		t.Fatalf("schema version = %d (%v), want at least %d", got, err, librarySchemaVersion)
	}
	if got := libraryTablesPresent(t, db.Write); !slices.Equal(got, libraryTables) {
		t.Fatalf("library tables = %v, want %v", got, libraryTables)
	}
}

// The upgrade keeps every account row, adds the library on top of them, and
// leaves a rollback copy that restores schema 2 for the previous binary.
func TestLibraryMigrationUpgradesSchemaTwoAndTheCopyRestoresIt(t *testing.T) {
	f := newLibrarySchemaTwoFixture(t)
	ctx := context.Background()
	before := libraryAccountRows(t, f.db.Write)
	if len(before) != 3 {
		t.Fatalf("fixture rows = %v, want two users and a session", before)
	}
	if err := MigrateState(ctx, f.db, libraryEmbeddedMigrations(t), f.rollbackDir); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if after := libraryAccountRows(t, f.db.Write); !slices.Equal(before, after) {
		t.Fatalf("account rows changed across the upgrade:\nbefore %v\nafter  %v", before, after)
	}
	if err := BootstrapLibraryLists(ctx, f.db, time.Unix(libraryFixtureTime, 0)); err != nil {
		t.Fatalf("bootstrap on the upgraded file: %v", err)
	}
	libraryRequireBootstrappedLists(t, f.db.Write, libraryUserA, libraryUserB)
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}

	libraryRestoreRollbackCopy(t, f.path, RollbackCopyPath(f.rollbackDir, 2))
	restored := openTestDatabase(t, f.path)
	if got, err := schemaVersion(ctx, restored.Write); err != nil || got != 2 {
		t.Fatalf("restored schema version = %d (%v), want 2", got, err)
	}
	if got := libraryTablesPresent(t, restored.Write); len(got) != 0 {
		t.Fatalf("the restored file carries library tables %v", got)
	}
	if got := libraryAccountRows(t, restored.Write); !slices.Equal(before, got) {
		t.Fatalf("restored account rows = %v, want %v", got, before)
	}
	// The previous binary knows two migrations; it must find nothing to do.
	if err := MigrateState(ctx, restored, libraryEmbeddedMigrations(t)[:2], f.rollbackDir); err != nil {
		t.Fatalf("the previous migration set refuses the restored file: %v", err)
	}
}

func libraryRestoreRollbackCopy(t *testing.T, statePath, copyPath string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(statePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if err := os.Rename(copyPath, statePath); err != nil {
		t.Fatalf("restore %s: %v", copyPath, err)
	}
}

// libraryCheckpointedDigest folds the WAL into state.db and hashes the file,
// so two digests differ only if committed content did.
func libraryCheckpointedDigest(t *testing.T, db *Database, path string) [32]byte {
	t.Helper()
	if _, err := db.Write.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(body)
}

// The real 0003 with a failing statement appended: every library table has
// been created by the time it fails, so a step that is not one transaction
// would leave them behind.
func TestLibraryMigrationFailureLeavesTheFileUnchanged(t *testing.T) {
	f := newLibrarySchemaTwoFixture(t)
	migrations := libraryEmbeddedMigrations(t)
	broken := slices.Clone(migrations[:librarySchemaVersion])
	broken[librarySchemaVersion-1].SQL += "\nINSERT INTO no_such_table VALUES (1);\n"
	accountsBefore := libraryAccountRows(t, f.db.Write)
	digestBefore := libraryCheckpointedDigest(t, f.db, f.path)

	err := MigrateState(context.Background(), f.db, broken, f.rollbackDir)
	if err == nil || !strings.Contains(err.Error(), "0003_library") {
		t.Fatalf("MigrateState error = %v, want a failure naming 0003_library", err)
	}
	if got := libraryCheckpointedDigest(t, f.db, f.path); got != digestBefore {
		t.Fatal("state.db changed on disk after a failed migration")
	}
	if got, _ := schemaVersion(context.Background(), f.db.Write); got != 2 {
		t.Fatalf("schema version = %d after a failed migration, want 2", got)
	}
	if got := libraryTablesPresent(t, f.db.Write); len(got) != 0 {
		t.Fatalf("a failed migration left tables %v", got)
	}
	if got := libraryAccountRows(t, f.db.Write); !slices.Equal(accountsBefore, got) {
		t.Fatalf("account rows changed: %v -> %v", accountsBefore, got)
	}
}
