package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Cache schema. cache.db is disposable: the schema version lives in
// PRAGMA user_version, and a file at any other version is replaced by a fresh
// one rather than migrated. state.db is never touched here.
//
// PRAGMA auto_vacuum = INCREMENTAL must be set on the empty file before its
// first table is created, or incremental_vacuum does nothing at all.
const (
	// CacheSchemaVersion is the only user_version cache.db may hold.
	CacheSchemaVersion = 1

	cacheTableDDL = `CREATE TABLE IF NOT EXISTS tmdb_cache (
		key TEXT PRIMARY KEY,
		status INTEGER NOT NULL,
		body BLOB NOT NULL,
		expires_at INTEGER NOT NULL
	)`
	cacheExpiryIndexDDL = `CREATE INDEX IF NOT EXISTS tmdb_cache_expires ON tmdb_cache (expires_at)`
)

// CacheStore is cache.db behind a read/write mutex. Normal use (Get, Set,
// PruneExpired) holds the read lock so requests run together; Rotate takes
// the write lock so no request holds a cache connection while the file is
// swapped. The write lock is only for the swap.
type CacheStore struct {
	mu   sync.RWMutex
	db   *Database
	path string
}

// OpenCacheStore opens path, creating it when missing, and ensures the cache
// schema. A file at any other user_version is replaced by a fresh one.
func OpenCacheStore(ctx context.Context, path string) (*CacheStore, error) {
	db, err := OpenDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	version, err := schemaVersion(ctx, db.Write)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if version != CacheSchemaVersion {
		_ = db.Close()
		if err := replaceWithFreshCache(ctx, path); err != nil {
			return nil, err
		}
		db, err = OpenDatabase(ctx, path)
		if err != nil {
			return nil, err
		}
		// The replacement already carries the schema, but verify it so a
		// failed init cannot serve silently.
		if version, err = schemaVersion(ctx, db.Write); err != nil {
			_ = db.Close()
			return nil, err
		}
		if version != CacheSchemaVersion {
			_ = db.Close()
			return nil, fmt.Errorf("cache schema is version %d, want %d", version, CacheSchemaVersion)
		}
		return &CacheStore{db: db, path: path}, nil
	}
	if err := ensureCacheTables(ctx, db.Write); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &CacheStore{db: db, path: path}, nil
}

// Close closes both pools.
func (s *CacheStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Path returns the cache file path.
func (s *CacheStore) Path() string {
	return s.path
}

// CacheEntry is one cached TMDB response.
type CacheEntry struct {
	Status    int
	Body      []byte
	ExpiresAt int64
}

// Get returns the row for key, or ok=false when absent.
func (s *CacheStore) Get(ctx context.Context, key string) (entry CacheEntry, ok bool, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return CacheEntry{}, false, errors.New("cache is closed")
	}
	var status int
	var body []byte
	var expiresAt int64
	rowErr := s.db.Read.QueryRowContext(ctx, "SELECT status, body, expires_at FROM tmdb_cache WHERE key = ?", key).Scan(&status, &body, &expiresAt)
	if errors.Is(rowErr, sql.ErrNoRows) {
		return CacheEntry{}, false, nil
	}
	if rowErr != nil {
		return CacheEntry{}, false, rowErr
	}
	return CacheEntry{Status: status, Body: body, ExpiresAt: expiresAt}, true, nil
}

// Set inserts or replaces the row for key.
func (s *CacheStore) Set(ctx context.Context, key string, status int, body []byte, expiresAt int64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return errors.New("cache is closed")
	}
	_, err := s.db.Write.ExecContext(ctx, "INSERT OR REPLACE INTO tmdb_cache (key, status, body, expires_at) VALUES (?, ?, ?, ?)", key, status, body, expiresAt)
	return err
}

// PruneExpired deletes rows whose expiry has passed and runs an incremental
// vacuum so the freed pages return to the file. It returns the deleted count.
func (s *CacheStore) PruneExpired(ctx context.Context, nowUnix int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0, errors.New("cache is closed")
	}
	result, err := s.db.Write.ExecContext(ctx, "DELETE FROM tmdb_cache WHERE expires_at <= ?", nowUnix)
	if err != nil {
		return 0, err
	}
	deleted, _ := result.RowsAffected()
	// incremental_vacuum without a page count frees every free page. It must
	// run on the writer: readers are query_only.
	if _, err := s.db.Write.ExecContext(ctx, "PRAGMA incremental_vacuum"); err != nil {
		return deleted, err
	}
	return deleted, nil
}

// Size returns the on-disk size of the cache file plus its WAL sidecars when
// present. Missing sidecars count as zero rather than an error.
func (s *CacheStore) Size() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cacheFileSize(s.path)
}

func cacheFileSize(path string) (int64, error) {
	var total int64
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		total += info.Size()
	}
	return total, nil
}

// PruneAndMaybeRotate prunes expired rows, vacuums, and rotates when the file
// is still over maxBytes. It returns true when a rotation happened.
func (s *CacheStore) PruneAndMaybeRotate(ctx context.Context, nowUnix int64, maxBytes int64) (bool, error) {
	if _, err := s.PruneExpired(ctx, nowUnix); err != nil {
		return false, err
	}
	size, err := s.Size()
	if err != nil {
		return false, err
	}
	if size <= maxBytes {
		return false, nil
	}
	if err := s.Rotate(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Rotate builds and initialises a fresh file under a temporary name, then
// swaps it in while no request holds a cache connection. The old -wal and
// -shm files are removed.
func (s *CacheStore) Rotate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return errors.New("cache is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.db.Close(); err != nil {
		return err
	}
	s.db = nil

	dir := filepath.Dir(s.path)
	fresh, err := os.CreateTemp(dir, "cache-*.fresh.db")
	if err != nil {
		return err
	}
	freshPath := fresh.Name()
	_ = fresh.Close()
	// The temp file is empty, so init sets auto_vacuum before the first table.
	if err := initFreshCacheFile(ctx, freshPath); err != nil {
		_ = os.Remove(freshPath)
		return s.reopenAfterFailedRotate(ctx)
	}
	if err := os.Rename(freshPath, s.path); err != nil {
		_ = os.Remove(freshPath)
		return s.reopenAfterFailedRotate(ctx)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		// A stale sidecar next to a fresh file is harmless: the new database
		// opens without it and recreates what it needs. Failures are ignored
		// so a read-only sidecar cannot fail an otherwise good rotation.
		_ = os.Remove(s.path + suffix)
	}
	db, err := OpenDatabase(ctx, s.path)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *CacheStore) reopenAfterFailedRotate(ctx context.Context) error {
	db, err := OpenDatabase(ctx, s.path)
	if err != nil {
		return err
	}
	s.db = db
	return errors.New("cache rotation failed, old file kept")
}

func replaceWithFreshCache(ctx context.Context, path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return initFreshCacheFile(ctx, path)
}

// initFreshCacheFile creates the cache schema on an empty file. auto_vacuum
// comes first: once the first table exists the pragma is locked in.
func initFreshCacheFile(ctx context.Context, path string) error {
	// A single connection: the file is empty and nothing else uses it yet.
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		return fmt.Errorf("set auto_vacuum: %w", err)
	}
	if err := ensureCacheTables(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", CacheSchemaVersion)); err != nil {
		return fmt.Errorf("set cache schema version: %w", err)
	}
	return nil
}

func ensureCacheTables(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, cacheTableDDL); err != nil {
		return fmt.Errorf("create tmdb_cache: %w", err)
	}
	if _, err := db.ExecContext(ctx, cacheExpiryIndexDDL); err != nil {
		return fmt.Errorf("create tmdb_cache_expires: %w", err)
	}
	return nil
}
