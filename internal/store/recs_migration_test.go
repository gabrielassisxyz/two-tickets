package store

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const recsSchemaVersion = 4

var recsTables = []string{"rec_runs", "rec_results", "rec_exposures"}

// recsSchemaThreeFixture is a state.db as the previous release leaves it:
// schema 3, both accounts and a live session.
type recsSchemaThreeFixture struct {
	dir, path, rollbackDir string
	db                     *Database
}

func newRecsSchemaThreeFixture(t *testing.T) *recsSchemaThreeFixture {
	t.Helper()
	dir := t.TempDir()
	f := &recsSchemaThreeFixture{dir: dir, path: filepath.Join(dir, StateFileName), rollbackDir: filepath.Join(dir, RollbackDirName)}
	f.db = openTestDatabase(t, f.path)
	migrations := libraryEmbeddedMigrations(t)
	if len(migrations) < recsSchemaVersion-1 || migrations[recsSchemaVersion-2].Name != "library" {
		t.Fatalf("embedded migrations = %d, want 0003_library among them", len(migrations))
	}
	if err := MigrateState(context.Background(), f.db, migrations[:recsSchemaVersion-1], f.rollbackDir); err != nil {
		t.Fatalf("migrate to schema 3: %v", err)
	}
	libraryInsertUser(t, f.db.Write, libraryUserA)
	libraryInsertUser(t, f.db.Write, libraryUserB)
	return f
}

func recsTablesPresent(t *testing.T, db *Database) []string {
	t.Helper()
	var present []string
	for _, table := range recsTables {
		if tableExists(t, db.Write, table) {
			present = append(present, table)
		}
	}
	return present
}

func TestRecsMigrationUpgradesSchemaThreeAndTheCopyRestoresIt(t *testing.T) {
	f := newRecsSchemaThreeFixture(t)
	ctx := context.Background()
	before := libraryAccountRows(t, f.db.Write)
	if got := recsTablesPresent(t, f.db); len(got) != 0 {
		t.Fatalf("schema 3 already carries rec tables %v", got)
	}
	if err := MigrateState(ctx, f.db, libraryEmbeddedMigrations(t), f.rollbackDir); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got, err := schemaVersion(ctx, f.db.Write); err != nil || got != recsSchemaVersion {
		t.Fatalf("schema version = %d (%v), want %d", got, err, recsSchemaVersion)
	}
	if got := recsTablesPresent(t, f.db); !slices.Equal(got, recsTables) {
		t.Fatalf("rec tables = %v, want %v", got, recsTables)
	}
	if after := libraryAccountRows(t, f.db.Write); !slices.Equal(before, after) {
		t.Fatalf("account rows changed across the upgrade:\nbefore %v\nafter  %v", before, after)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}

	libraryRestoreRollbackCopy(t, f.path, RollbackCopyPath(f.rollbackDir, recsSchemaVersion-1))
	restored := openTestDatabase(t, f.path)
	if got, err := schemaVersion(ctx, restored.Write); err != nil || got != recsSchemaVersion-1 {
		t.Fatalf("restored schema version = %d (%v), want %d", got, err, recsSchemaVersion-1)
	}
	if got := recsTablesPresent(t, restored); len(got) != 0 {
		t.Fatalf("the restored file carries rec tables %v", got)
	}
	if got := libraryAccountRows(t, restored.Write); !slices.Equal(before, got) {
		t.Fatalf("restored account rows = %v, want %v", got, before)
	}
	// The previous binary knows three migrations; it must find nothing to do.
	if err := MigrateState(ctx, restored, libraryEmbeddedMigrations(t)[:recsSchemaVersion-1], f.rollbackDir); err != nil {
		t.Fatalf("the previous migration set refuses the restored file: %v", err)
	}
}

// The real 0004 with a failing statement appended: every rec table has been
// created by the time it fails, so a step that is not one transaction would
// leave them behind.
func TestRecsMigrationFailureLeavesTheFileUnchanged(t *testing.T) {
	f := newRecsSchemaThreeFixture(t)
	migrations := libraryEmbeddedMigrations(t)
	broken := slices.Clone(migrations[:recsSchemaVersion])
	broken[recsSchemaVersion-1].SQL += "\nINSERT INTO no_such_table VALUES (1);\n"
	accountsBefore := libraryAccountRows(t, f.db.Write)
	digestBefore := libraryCheckpointedDigest(t, f.db, f.path)

	err := MigrateState(context.Background(), f.db, broken, f.rollbackDir)
	if err == nil || !strings.Contains(err.Error(), "0004_recs") {
		t.Fatalf("MigrateState error = %v, want a failure naming 0004_recs", err)
	}
	if got := libraryCheckpointedDigest(t, f.db, f.path); got != digestBefore {
		t.Fatal("state.db changed on disk after a failed migration")
	}
	if got, _ := schemaVersion(context.Background(), f.db.Write); got != recsSchemaVersion-1 {
		t.Fatalf("schema version = %d after a failed migration, want %d", got, recsSchemaVersion-1)
	}
	if got := recsTablesPresent(t, f.db); len(got) != 0 {
		t.Fatalf("a failed migration left tables %v", got)
	}
	if got := libraryAccountRows(t, f.db.Write); !slices.Equal(accountsBefore, got) {
		t.Fatalf("account rows changed: %v -> %v", accountsBefore, got)
	}
}
