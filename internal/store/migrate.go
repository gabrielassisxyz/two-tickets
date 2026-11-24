package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

// Migration is one forward-only schema step: Version N takes state.db from
// schema N-1 to schema N. The schema version lives in PRAGMA user_version,
// which sits in the database header and so commits or rolls back with the
// migration's own transaction.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

//go:embed migrations/state/*.sql
var stateMigrationFiles embed.FS

var migrationFileName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// StateMigrations returns the migrations compiled into the binary.
func StateMigrations() fs.FS {
	sub, err := fs.Sub(stateMigrationFiles, "migrations/state")
	if err != nil {
		// fs.Sub only fails on an invalid path, and this one is a constant.
		panic(err)
	}
	return sub
}

// LoadMigrations reads NNNN_name.sql files from the root of fsys. The versions
// must run 1, 2, 3... without a gap or a repeat: a hole would mean a binary
// that silently skips a step.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m, err := readMigrationFile(fsys, entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, m)
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for i, m := range migrations {
		if m.Version != i+1 {
			return nil, fmt.Errorf("migration %04d_%s: expected version %d, the sequence must start at 1 with no gap or repeat", m.Version, m.Name, i+1)
		}
	}
	return migrations, nil
}

func readMigrationFile(fsys fs.FS, name string) (Migration, error) {
	match := migrationFileName.FindStringSubmatch(name)
	if match == nil {
		return Migration{}, fmt.Errorf("migration file %q is not named NNNN_name.sql", name)
	}
	version, err := strconv.Atoi(match[1])
	if err != nil {
		return Migration{}, fmt.Errorf("migration file %q: %w", name, err)
	}
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Migration{}, fmt.Errorf("read migration %q: %w", name, err)
	}
	return Migration{Version: version, Name: match[2], SQL: string(body)}, nil
}

// MigrateState brings the database up to the newest migration. Before the
// first pending step it writes a verified rollback copy of the current schema
// into rollbackDir; no step runs without one. Any failure is returned, and the
// caller must stop: a half-known schema is not something to serve from.
func MigrateState(ctx context.Context, db *Database, migrations []Migration, rollbackDir string) error {
	current, err := schemaVersion(ctx, db.Write)
	if err != nil {
		return err
	}
	latest := len(migrations)
	if current > latest {
		return fmt.Errorf("database schema is version %d, newer than the %d this binary knows; run a newer binary or restore a rollback copy", current, latest)
	}
	if current == latest {
		return nil
	}
	if err := writeRollbackCopy(ctx, db.Write, rollbackDir, current); err != nil {
		return fmt.Errorf("rollback copy of schema %d: %w", current, err)
	}
	for _, m := range migrations[current:] {
		if err := applyMigration(ctx, db.Write, m); err != nil {
			return fmt.Errorf("migration %04d_%s: %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func schemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m Migration) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if _, err = tx.ExecContext(ctx, m.SQL); err != nil {
		return err
	}
	// foreign_keys cannot be switched off inside a transaction, so a step
	// that rebuilds a table could still leave dangling references behind.
	if err = requireNoForeignKeyViolations(ctx, tx); err != nil {
		return err
	}
	// PRAGMA takes no bound parameters; Version is an int, never user input.
	if _, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
		return err
	}
	return tx.Commit()
}

type sqlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func requireNoForeignKeyViolations(ctx context.Context, q sqlQuerier) error {
	rows, err := q.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("foreign_key_check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return errors.New("foreign_key_check reports violations")
	}
	return rows.Err()
}
