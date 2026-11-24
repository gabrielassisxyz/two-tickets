package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// testSchemaSteps is a small schema that grows one step at a time, with a
// foreign key so the rollback copy's foreign_key_check has something to check.
var testSchemaSteps = []Migration{
	{Version: 1, Name: "title", SQL: "CREATE TABLE title (id INTEGER PRIMARY KEY, name TEXT NOT NULL);"},
	{Version: 2, Name: "watch", SQL: "CREATE TABLE watch (id INTEGER PRIMARY KEY, title_id INTEGER NOT NULL REFERENCES title(id));"},
	{Version: 3, Name: "title_year", SQL: "ALTER TABLE title ADD COLUMN year INTEGER;"},
	{Version: 4, Name: "title_name_index", SQL: "CREATE INDEX title_name ON title(name);"},
}

type migrationFixture struct {
	dir         string
	dbPath      string
	rollbackDir string
	db          *Database
}

func newMigrationFixture(t *testing.T) *migrationFixture {
	t.Helper()
	dir := t.TempDir()
	f := &migrationFixture{dir: dir, dbPath: filepath.Join(dir, StateFileName), rollbackDir: filepath.Join(dir, RollbackDirName)}
	f.db = openTestDatabase(t, f.dbPath)
	return f
}

func (f *migrationFixture) migrateTo(t *testing.T, version int) {
	t.Helper()
	if err := MigrateState(context.Background(), f.db, testSchemaSteps[:version], f.rollbackDir); err != nil {
		t.Fatalf("MigrateState to %d: %v", version, err)
	}
}

func (f *migrationFixture) version(t *testing.T) int {
	t.Helper()
	v, err := schemaVersion(context.Background(), f.db.Write)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func (f *migrationFixture) rollbackCopies(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.rollbackDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMigrateStateFromAnEmptyFile(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 3)
	if got := f.version(t); got != 3 {
		t.Fatalf("schema version = %d, want 3", got)
	}
	if !tableExists(t, f.db.Write, "watch") {
		t.Fatal("migration 2 did not run")
	}
}

func TestMigrateStateUpgradeWritesAVerifiedRollbackCopy(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 1)
	if _, err := f.db.Write.Exec("INSERT INTO title (id, name) VALUES (1, 'Central do Brasil')"); err != nil {
		t.Fatal(err)
	}
	f.migrateTo(t, 2)

	copyPath := RollbackCopyPath(f.rollbackDir, 1)
	if copyPath != filepath.Join(f.dir, "pre-migration", "state-1.db") {
		t.Fatalf("copy path = %q, want <data dir>/pre-migration/state-1.db", copyPath)
	}
	uri := "file:" + copyPath + "?mode=ro"
	copyDB, err := sql.Open(sqliteDriverName, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = copyDB.Close() }()

	var integrity string
	if err := copyDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("integrity_check on the copy: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q, want ok", integrity)
	}
	if err := requireNoForeignKeyViolations(context.Background(), copyDB); err != nil {
		t.Fatalf("foreign_key_check on the copy: %v", err)
	}
	var name string
	if err := copyDB.QueryRow("SELECT name FROM title WHERE id = 1").Scan(&name); err != nil || name != "Central do Brasil" {
		t.Fatalf("copy lost the committed row: name=%q err=%v", name, err)
	}
}

func TestMigrateStateKeepsOnlyTheTwoNewestCopies(t *testing.T) {
	f := newMigrationFixture(t)
	for version := 1; version <= 3; version++ {
		f.migrateTo(t, version)
	}
	got := f.rollbackCopies(t)
	want := []string{"state-1.db", "state-2.db"}
	if !slices.Equal(got, want) {
		t.Fatalf("rollback copies = %v, want %v", got, want)
	}
}

func TestMigrateStateWithNothingPendingTakesNoCopy(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 2)
	before := f.rollbackCopies(t)
	f.migrateTo(t, 2)
	if after := f.rollbackCopies(t); !slices.Equal(before, after) {
		t.Fatalf("copies changed with nothing to migrate: %v -> %v", before, after)
	}
}

func TestMigrateStateFailedStepLeavesTheSchemaUntouched(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 1)
	broken := append(slices.Clone(testSchemaSteps[:1]), Migration{
		Version: 2,
		Name:    "broken",
		SQL:     "CREATE TABLE half_done (a INTEGER); INSERT INTO no_such_table VALUES (1);",
	})
	err := MigrateState(context.Background(), f.db, broken, f.rollbackDir)
	if err == nil {
		t.Fatal("MigrateState succeeded with a failing migration")
	}
	if !strings.Contains(err.Error(), "0002_broken") {
		t.Errorf("error %q does not name the failing migration", err)
	}
	if got := f.version(t); got != 1 {
		t.Fatalf("schema version = %d after a failed step, want 1", got)
	}
	if tableExists(t, f.db.Write, "half_done") {
		t.Fatal("the failed migration's first statement was committed")
	}
}

func TestMigrateStateRefusesASchemaNewerThanTheBinary(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 3)
	if err := MigrateState(context.Background(), f.db, testSchemaSteps[:2], f.rollbackDir); err == nil {
		t.Fatal("MigrateState accepted a database newer than its migrations")
	}
}

// A database that already holds a dangling reference produces a copy whose
// foreign_key_check is not empty; the migration must not run on top of it.
func TestMigrateStateStopsWhenTheCopyFailsVerification(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 2)
	raw, err := sql.Open(sqliteDriverName, f.dbPath) // foreign_keys is off by default
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO watch (id, title_id) VALUES (1, 999)"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	err = MigrateState(context.Background(), f.db, testSchemaSteps[:3], f.rollbackDir)
	if err == nil || !strings.Contains(err.Error(), "foreign_key_check") {
		t.Fatalf("MigrateState error = %v, want a foreign_key_check failure", err)
	}
	if got := f.version(t); got != 2 {
		t.Fatalf("schema version = %d, want 2: a migration ran without a verified copy", got)
	}
	if slices.Contains(f.rollbackCopies(t), "state-2.db") || slices.Contains(f.rollbackCopies(t), "state-2.db.partial") {
		t.Fatalf("an unverified copy was left behind: %v", f.rollbackCopies(t))
	}
}

// writeIndexedFile creates a database at schema 7 with an index and no foreign
// key, so foreign_key_check has nothing to read and only integrity_check can
// object to what the tests below do to it. The intact file must verify first.
func writeIndexedFile(t *testing.T) (path string, indexRootPage, pageSize int64) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "state-7.db")
	raw, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	setup := `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT NOT NULL, w TEXT NOT NULL);
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 2000)
		INSERT INTO t (id, v, w) SELECT i, printf('value-%08d', i), printf('other-%08d', 2001 - i) FROM n;
		CREATE INDEX t_v ON t(v);
		PRAGMA user_version = 7;`
	if _, err := raw.Exec(setup); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow("SELECT rootpage FROM sqlite_master WHERE name = 't_v'").Scan(&indexRootPage); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyRollbackCopy(context.Background(), path, 7); err != nil {
		t.Fatalf("the intact file failed verification: %v", err)
	}
	return path, indexRootPage, pageSize
}

func TestVerifyRollbackCopyRejectsTheWrongSchemaVersion(t *testing.T) {
	path, _, _ := writeIndexedFile(t)
	if err := verifyRollbackCopy(context.Background(), path, 8); err == nil {
		t.Fatal("a copy at schema 7 verified as schema 8")
	}
}

// A torn index page makes integrity_check fail as a query.
func TestVerifyRollbackCopyRejectsAMalformedPage(t *testing.T) {
	path, rootPage, pageSize := writeIndexedFile(t)
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	garbage := []byte(strings.Repeat("\xff", int(pageSize)))
	if _, err := file.WriteAt(garbage, (rootPage-1)*pageSize); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	if err := verifyRollbackCopy(context.Background(), path, 7); err == nil {
		t.Fatal("a file with a malformed index page passed verification")
	}
}

// An index whose definition no longer matches its entries leaves every page
// well formed, so integrity_check answers with report rows instead of failing:
// the check has to read what it says, not only whether it ran.
func TestVerifyRollbackCopyRejectsAnIndexThatDisagreesWithItsTable(t *testing.T) {
	path, _, _ := writeIndexedFile(t)
	raw, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	rewrite := `PRAGMA writable_schema = ON;
		UPDATE sqlite_master SET sql = 'CREATE INDEX t_v ON t(w)' WHERE name = 't_v';
		PRAGMA writable_schema = OFF;`
	if _, err := raw.Exec(rewrite); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	err = verifyRollbackCopy(context.Background(), path, 7)
	if err == nil {
		t.Fatal("a file whose index disagrees with its table passed verification")
	}
	if !strings.Contains(err.Error(), "integrity_check reports") {
		t.Fatalf("verify failed for another reason than the integrity report: %v", err)
	}
}

// The recovery path: move the copy over state.db, drop the WAL files, and the
// previous binary (here, the previous migration set) opens it at its schema.
func TestRollbackCopyRestoresThePreviousSchema(t *testing.T) {
	f := newMigrationFixture(t)
	f.migrateTo(t, 1)
	if _, err := f.db.Write.Exec("INSERT INTO title (id, name) VALUES (1, 'Bacurau')"); err != nil {
		t.Fatal(err)
	}
	f.migrateTo(t, 2)
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(f.dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if err := os.Rename(RollbackCopyPath(f.rollbackDir, 1), f.dbPath); err != nil {
		t.Fatal(err)
	}

	f.db = openTestDatabase(t, f.dbPath)
	if got := f.version(t); got != 1 {
		t.Fatalf("restored schema version = %d, want 1", got)
	}
	if tableExists(t, f.db.Write, "watch") {
		t.Fatal("the restored file carries the newer schema")
	}
	var name string
	if err := f.db.Read.QueryRow("SELECT name FROM title WHERE id = 1").Scan(&name); err != nil || name != "Bacurau" {
		t.Fatalf("restored row: name=%q err=%v", name, err)
	}
	f.migrateTo(t, 1)
}

func TestLoadMigrationsRequiresAnUnbrokenSequence(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"gap":      {"0001_a.sql": {Data: []byte("SELECT 1;")}, "0003_c.sql": {Data: []byte("SELECT 1;")}},
		"repeat":   {"0001_a.sql": {Data: []byte("SELECT 1;")}, "0001_b.sql": {Data: []byte("SELECT 1;")}},
		"no one":   {"0002_b.sql": {Data: []byte("SELECT 1;")}},
		"bad name": {"0001-a.sql": {Data: []byte("SELECT 1;")}},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadMigrations(fsys); err == nil {
				t.Fatal("LoadMigrations accepted a broken set")
			}
		})
	}
}

func TestEmbeddedStateMigrationsLoadAndApply(t *testing.T) {
	migrations, err := LoadMigrations(StateMigrations())
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(migrations) == 0 || migrations[0].Version != 1 {
		t.Fatalf("embedded migrations = %+v, want a set starting at 1", migrations)
	}
	f := newMigrationFixture(t)
	if err := MigrateState(context.Background(), f.db, migrations, f.rollbackDir); err != nil {
		t.Fatalf("MigrateState: %v", err)
	}
	if got := f.version(t); got != len(migrations) {
		t.Fatalf("schema version = %d, want %d", got, len(migrations))
	}
}
