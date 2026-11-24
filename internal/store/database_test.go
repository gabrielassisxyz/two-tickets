package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func openTestDatabase(t *testing.T, path string) *Database {
	t.Helper()
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type connectionPragmaValues struct {
	journalMode string
	foreignKeys int
	busyTimeout int
}

func readConnectionPragmas(t *testing.T, conn *sql.Conn) connectionPragmaValues {
	t.Helper()
	ctx := context.Background()
	var got connectionPragmaValues
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&got.journalMode); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&got.foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&got.busyTimeout); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertConnectionPragmas(t *testing.T, label string, conn *sql.Conn) {
	t.Helper()
	want := connectionPragmaValues{journalMode: "wal", foreignKeys: 1, busyTimeout: 5000}
	if got := readConnectionPragmas(t, conn); got != want {
		t.Errorf("%s: pragmas = %+v, want %+v", label, got, want)
	}
}

// Two reader connections are held at once, so the pool has to open a second
// one: a pragma applied only to the first connection would pass a test that
// borrowed the same connection twice.
func TestOpenDatabaseSetsPragmasOnWriterAndPooledReaders(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{StateFileName, CacheFileName} {
		t.Run(name, func(t *testing.T) {
			db := openTestDatabase(t, filepath.Join(t.TempDir(), name))

			writer, err := db.Write.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = writer.Close() }()
			assertConnectionPragmas(t, "writer", writer)

			first, err := db.Read.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = first.Close() }()
			second, err := db.Read.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = second.Close() }()
			assertConnectionPragmas(t, "first reader", first)
			assertConnectionPragmas(t, "second reader", second)
		})
	}
}

func TestOpenDatabaseReadersCannotWrite(t *testing.T) {
	db := openTestDatabase(t, filepath.Join(t.TempDir(), StateFileName))
	if _, err := db.Read.Exec("CREATE TABLE readers_must_not_write (a INTEGER)"); err == nil {
		t.Fatal("a reader connection accepted a write")
	}
	if _, err := db.Write.Exec("CREATE TABLE writer_may_write (a INTEGER)"); err != nil {
		t.Fatalf("the writer refused a write: %v", err)
	}
}

func TestOpenDatabaseWriterIsASingleConnection(t *testing.T) {
	db := openTestDatabase(t, filepath.Join(t.TempDir(), StateFileName))
	if got := db.Write.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("writer MaxOpenConnections = %d, want 1", got)
	}
}

func TestPingFailsOnceTheDatabaseIsClosed(t *testing.T) {
	db, err := OpenDatabase(context.Background(), filepath.Join(t.TempDir(), StateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("Ping on an open database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(context.Background()); err == nil {
		t.Fatal("Ping succeeded on a closed database")
	}
}
