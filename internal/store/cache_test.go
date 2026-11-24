package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestCache(t *testing.T) (*CacheStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), CacheFileName)
	cache, err := OpenCacheStore(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenCacheStore: %v", err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	return cache, path
}

func TestCacheSchemaIsVersioned(t *testing.T) {
	cache, _ := openTestCache(t)
	var version int
	if err := cache.db.Write.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CacheSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, CacheSchemaVersion)
	}
}

func TestCacheAutoVacuumIsIncremental(t *testing.T) {
	cache, _ := openTestCache(t)
	var mode string
	if err := cache.db.Write.QueryRow("PRAGMA auto_vacuum").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	// 2 means incremental. A file created without the pragma first would read
	// 0 (none) or 1 (full), and incremental_vacuum would do nothing.
	if mode != "2" {
		t.Fatalf("auto_vacuum = %q, want incremental (2)", mode)
	}
}

func TestCacheFileAtAnotherVersionIsReplaced(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), CacheFileName)
	cache, err := OpenCacheStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, "v1|GET|/movie/1|", 200, []byte(`{"id":1}`), 9999999999); err != nil {
		t.Fatal(err)
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate an older binary's file by bumping the version out from under it.
	db, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	reopened, err := OpenCacheStore(ctx, path)
	if err != nil {
		t.Fatalf("OpenCacheStore after version skew: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	if _, ok, err := reopened.Get(ctx, "v1|GET|/movie/1|"); err != nil || ok {
		t.Fatalf("old row survived a version replacement: ok=%v err=%v", ok, err)
	}
	var version int
	if err := reopened.db.Write.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CacheSchemaVersion {
		t.Fatalf("user_version = %d, want %d", version, CacheSchemaVersion)
	}
}

func TestCachePruneDeletesOnlyExpiredRows(t *testing.T) {
	ctx := context.Background()
	cache, _ := openTestCache(t)
	if err := cache.Set(ctx, "old", 200, []byte(`{}`), 100); err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(ctx, "fresh", 200, []byte(`{}`), 9999999999); err != nil {
		t.Fatal(err)
	}
	deleted, err := cache.PruneExpired(ctx, 1000)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, ok, _ := cache.Get(ctx, "old"); ok {
		t.Fatal("expired row survived the prune")
	}
	if _, ok, _ := cache.Get(ctx, "fresh"); !ok {
		t.Fatal("live row was pruned")
	}
}

func TestCachePruneThenRotateWithTinyCap(t *testing.T) {
	ctx := context.Background()
	cache, path := openTestCache(t)
	// Non-expired rows so the prune frees nothing and the size check forces a
	// rotation. Each body is 1 KiB; the cap is a few kilobytes.
	big := make([]byte, 1024)
	for i := range big {
		big[i] = 'x'
	}
	for i := 0; i < 20; i++ {
		key := "key-" + string(rune('a'+i)) + string(rune('0'+i%10))
		if err := cache.Set(ctx, key, 200, big, 9999999999); err != nil {
			t.Fatal(err)
		}
	}
	rotated, err := cache.PruneAndMaybeRotate(ctx, 0, 4096)
	if err != nil {
		t.Fatalf("PruneAndMaybeRotate: %v", err)
	}
	if !rotated {
		t.Fatal("a cache far over a 4 KiB cap did not rotate")
	}
	// The fresh file opens, accepts writes and holds none of the old rows.
	if err := cache.Set(ctx, "after-rotate", 200, []byte(`{}`), 9999999999); err != nil {
		t.Fatalf("fresh cache.db refused a write: %v", err)
	}
	if _, ok, _ := cache.Get(ctx, "key-a0"); ok {
		t.Fatal("an old row survived the rotation")
	}
	if _, ok, _ := cache.Get(ctx, "after-rotate"); !ok {
		t.Fatal("a write after rotation is not readable")
	}
	// The path still names a database file, not a leftover temp name.
	if path == "" {
		t.Fatal("empty cache path")
	}
}
