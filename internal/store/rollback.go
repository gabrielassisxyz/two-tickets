package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// RollbackDirName is the directory, inside the data directory, that holds the
// pre-migration copies of state.db.
const RollbackDirName = "pre-migration"

const keptRollbackCopies = 2

var rollbackCopyName = regexp.MustCompile(`^state-(\d+)\.db$`)

// RollbackCopyPath is where the copy of state.db at schema version lives.
func RollbackCopyPath(dir string, version int) string {
	return filepath.Join(dir, fmt.Sprintf("state-%d.db", version))
}

// writeRollbackCopy snapshots the database with VACUUM INTO, which goes
// through SQLite and so includes pages still in the WAL; copying state.db on
// its own could miss committed transactions. The snapshot is written under a
// temporary name and only renamed into place once it verifies, so a crash or a
// bad copy never leaves something that looks like a usable rollback point.
func writeRollbackCopy(ctx context.Context, writer *sql.DB, dir string, version int) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	final := RollbackCopyPath(dir, version)
	partial := final + ".partial"
	if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := writer.ExecContext(ctx, "VACUUM INTO ?", partial); err != nil {
		return fmt.Errorf("VACUUM INTO: %w", err)
	}
	if err := verifyRollbackCopy(ctx, partial, version); err != nil {
		return errors.Join(fmt.Errorf("verify: %w", err), os.Remove(partial))
	}
	if err := os.Rename(partial, final); err != nil {
		return err
	}
	return pruneRollbackCopies(dir)
}

// verifyRollbackCopy opens the copy read-only and requires a clean
// integrity_check, an empty foreign_key_check and the expected schema version.
func verifyRollbackCopy(ctx context.Context, path string, wantVersion int) (err error) {
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open(sqliteDriverName, uri.String())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	db.SetMaxOpenConns(1)

	if err := requireIntegrityOK(ctx, db); err != nil {
		return err
	}
	if err := requireNoForeignKeyViolations(ctx, db); err != nil {
		return err
	}
	got, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if got != wantVersion {
		return fmt.Errorf("copy is at schema %d, want %d", got, wantVersion)
	}
	return nil
}

func requireIntegrityOK(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var report []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return fmt.Errorf("integrity_check: %w", err)
		}
		report = append(report, line)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if len(report) != 1 || report[0] != "ok" {
		return fmt.Errorf("integrity_check reports %q", report)
	}
	return nil
}

// pruneRollbackCopies keeps the copies of the newest schema versions. Age is
// read from the version in the name, not the file time, which a restore or a
// backup tool can change.
func pruneRollbackCopies(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var versions []int
	for _, entry := range entries {
		match := rollbackCopyName.FindStringSubmatch(entry.Name())
		if match == nil || entry.IsDir() {
			continue
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		versions = append(versions, version)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))
	if len(versions) <= keptRollbackCopies {
		return nil
	}
	var errs []error
	for _, version := range versions[keptRollbackCopies:] {
		errs = append(errs, os.Remove(RollbackCopyPath(dir, version)))
	}
	return errors.Join(errs...)
}
