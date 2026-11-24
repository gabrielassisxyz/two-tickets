package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backupSeedDatabase creates a live database with two related tables and one
// watched title, the smallest shape the backup has to carry across intact.
func backupSeedDatabase(t *testing.T, path string) *Database {
	t.Helper()
	db := openTestDatabase(t, path)
	setup := `CREATE TABLE backup_title (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE backup_watch (id INTEGER PRIMARY KEY, title_id INTEGER NOT NULL REFERENCES backup_title(id));
		INSERT INTO backup_title (id, name) VALUES (1, 'Central do Brasil');
		INSERT INTO backup_watch (id, title_id) VALUES (1, 1);
		PRAGMA user_version = 3;`
	if _, err := db.Write.Exec(setup); err != nil {
		t.Fatalf("seed backup source: %v", err)
	}
	return db
}

// backupTargetRows reads the published artifact read-only and returns the
// title names it holds, failing when the artifact is missing or unreadable.
func backupTargetRows(t *testing.T, target string) []string {
	t.Helper()
	uri := "file:" + target + "?mode=ro"
	copyDB, err := sql.Open(sqliteDriverName, uri)
	if err != nil {
		t.Fatalf("open backup artifact: %v", err)
	}
	defer func() { _ = copyDB.Close() }()
	rows, err := copyDB.Query("SELECT name FROM backup_title ORDER BY id")
	if err != nil {
		t.Fatalf("read backup artifact: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan backup row: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read backup rows: %v", err)
	}
	return names
}

// backupRequireAbsent fails when a failed run leaves anything behind at path,
// whether a published artifact or a half-written copy.
func backupRequireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s exists after a failed backup (stat: %v)", path, err)
	}
}

func backupRequireChecks(t *testing.T, target string) {
	t.Helper()
	uri := "file:" + target + "?mode=ro"
	copyDB, err := sql.Open(sqliteDriverName, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = copyDB.Close() }()
	if err := requireIntegrityOK(context.Background(), copyDB); err != nil {
		t.Fatalf("integrity_check on the backup: %v", err)
	}
	if err := requireNoForeignKeyViolations(context.Background(), copyDB); err != nil {
		t.Fatalf("foreign_key_check on the backup: %v", err)
	}
}

// A torn page in the copy must fail verification, even though the file opens:
// the check has to read the integrity report, not only whether the copy was
// produced. writeIndexedFile is the rollback suite's corruptible fixture.
func TestBackupVerifyRejectsAMalformedCopy(t *testing.T) {
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

	if err := verifyBackupCopy(context.Background(), path); err == nil {
		t.Fatal("a copy with a malformed page passed verification")
	}
}

func TestBackupStateDBCopiesTheLiveDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	backupSeedDatabase(t, source)
	target := filepath.Join(dir, BackupDirName, StateFileName)

	start := time.Now()
	if err := BackupStateDB(ctx, source, target); err != nil {
		t.Fatalf("BackupStateDB: %v", err)
	}
	if got := backupTargetRows(t, target); len(got) != 1 || got[0] != "Central do Brasil" {
		t.Fatalf("backup rows = %q, want the seeded title", got)
	}
	backupRequireChecks(t, target)
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(start) {
		t.Fatalf("backup mtime = %v, want it newer than the run's start %v", info.ModTime(), start)
	}
	if _, err := os.Stat(target + ".partial"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary copy left behind (stat: %v)", err)
	}
}

func TestBackupStateDBReplacesThePreviousArtifact(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	db := backupSeedDatabase(t, source)
	target := filepath.Join(dir, BackupDirName, StateFileName)

	if err := BackupStateDB(ctx, source, target); err != nil {
		t.Fatalf("first BackupStateDB: %v", err)
	}
	// Push the first artifact an hour into the past, so the second run's
	// newer mtime proves replacement without waiting on the clock.
	stale := time.Now().Add(-time.Hour)
	if err := os.Chtimes(target, stale, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec("INSERT INTO backup_title (id, name) VALUES (2, 'Bacurau')"); err != nil {
		t.Fatal(err)
	}
	if err := BackupStateDB(ctx, source, target); err != nil {
		t.Fatalf("second BackupStateDB: %v", err)
	}
	if got := backupTargetRows(t, target); len(got) != 2 || got[1] != "Bacurau" {
		t.Fatalf("backup rows after replacement = %q, want both titles", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(stale) {
		t.Fatalf("backup mtime = %v, want it newer than the previous artifact %v", info.ModTime(), stale)
	}
}

// A source that is not a database at all must fail before anything is
// published, and leave neither an artifact nor a half-written copy.
func TestBackupStateDBRefusesACorruptSource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	if err := os.WriteFile(source, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, BackupDirName, StateFileName)

	if err := BackupStateDB(ctx, source, target); err == nil {
		t.Fatal("BackupStateDB succeeded on a corrupt source")
	}
	backupRequireAbsent(t, target)
	backupRequireAbsent(t, target+".partial")
}

// A dangling reference survives VACUUM INTO (it is logical, not a torn page),
// so only the verification step can refuse it: without that step this source
// would publish a backup that fails foreign_key_check.
func TestBackupStateDBRefusesAForeignKeyViolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	backupSeedDatabase(t, source)
	raw, err := sql.Open(sqliteDriverName, source) // foreign_keys is off by default
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("INSERT INTO backup_watch (id, title_id) VALUES (2, 999)"); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	target := filepath.Join(dir, BackupDirName, StateFileName)

	err = BackupStateDB(ctx, source, target)
	if err == nil {
		t.Fatal("BackupStateDB published a backup with a foreign key violation")
	}
	backupRequireAbsent(t, target)
	backupRequireAbsent(t, target+".partial")
}

func TestBackupStateDBRefusesAnUnwritableTarget(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories stay writable for root")
	}
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	backupSeedDatabase(t, source)
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	target := filepath.Join(locked, StateFileName)

	if err := BackupStateDB(ctx, source, target); err == nil {
		t.Fatal("BackupStateDB succeeded with an unwritable target")
	}
	backupRequireAbsent(t, target)
}

// A rerun that fails must not leave a half-written file where the previous
// complete artifact was: readers of the final path see a whole backup or
// nothing, never a corrupt middle state. A backup written straight to the
// final path instead of renamed into place would leave that middle state here.
func TestBackupStateDBFailedRerunLeavesNothingBehind(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	source := filepath.Join(dir, StateFileName)
	db := backupSeedDatabase(t, source)
	target := filepath.Join(dir, BackupDirName, StateFileName)

	if err := BackupStateDB(ctx, source, target); err != nil {
		t.Fatalf("first BackupStateDB: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := BackupStateDB(ctx, source, target); err == nil {
		t.Fatal("BackupStateDB succeeded on a corrupt source")
	}
	backupRequireAbsent(t, target)
	backupRequireAbsent(t, target+".partial")
}
