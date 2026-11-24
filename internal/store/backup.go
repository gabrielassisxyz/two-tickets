package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// BackupDirName is the directory, inside the data directory, that holds the
// backup artifact the home server's backup role reads.
const BackupDirName = "backup"

// DefaultBackupTarget is where the backup subcommand writes when no --target
// flag is given: the backup directory of the /data volume the api runs with.
const DefaultBackupTarget = "/data/" + BackupDirName + "/" + StateFileName

// BackupStateDB copies the live database at sourcePath to targetPath. It
// deletes any previous artifact, writes an online copy to a temporary file in
// the target directory with VACUUM INTO (which goes through SQLite and so
// includes pages still in the WAL), verifies the copy with integrity_check
// and foreign_key_check, fsyncs it, and renames it into place atomically. Any
// failure is returned with nothing left at the final path.
func BackupStateDB(ctx context.Context, sourcePath, targetPath string) error {
	if sourcePath == "" {
		return errors.New("backup source path is empty")
	}
	if targetPath == "" {
		return errors.New("backup target path is empty")
	}
	if sourcePath == targetPath {
		return errors.New("backup source and target are the same file")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("backup source: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("backup target directory: %w", err)
	}
	partial := targetPath + ".partial"
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous backup: %w", err)
	}
	if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale backup copy: %w", err)
	}
	if err := vacuumInto(ctx, sourcePath, partial); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := verifyBackupCopy(ctx, partial); err != nil {
		_ = os.Remove(partial)
		return err
	}
	if err := fsyncFile(partial); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("fsync backup copy: %w", err)
	}
	// os.Rename within one directory is atomic: readers of the final path see
	// either the previous complete artifact or the new one, never a half file.
	if err := os.Rename(partial, targetPath); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("publish backup: %w", err)
	}
	return nil
}

// vacuumInto writes an online copy of the database at sourcePath to destPath.
// The source is opened read-write (a query_only connection could not take the
// lock VACUUM needs) with a busy timeout, so a backup taken while the api
// serves waits its turn instead of failing.
func vacuumInto(ctx context.Context, sourcePath, destPath string) error {
	db, err := sql.Open(sqliteDriverName, sourcePath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open backup source: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("VACUUM INTO: %w", err)
	}
	return db.Close()
}

// verifyBackupCopy opens the copy read-only and requires a clean
// integrity_check and an empty foreign_key_check before it may be published.
func verifyBackupCopy(ctx context.Context, path string) (err error) {
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open(sqliteDriverName, uri.String())
	if err != nil {
		return fmt.Errorf("open backup copy: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	db.SetMaxOpenConns(1)

	if err := requireIntegrityOK(ctx, db); err != nil {
		return err
	}
	return requireNoForeignKeyViolations(ctx, db)
}

// fsyncFile flushes the copy to stable storage before the rename publishes
// it, so a crash cannot publish a file whose pages never reached the disk.
func fsyncFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
