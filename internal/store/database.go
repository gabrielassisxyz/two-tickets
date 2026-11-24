// Package store opens the service's two SQLite files and migrates state.db.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	// The pure-Go driver keeps the binary static; cgo would rule out a
	// distroless static image.
	_ "modernc.org/sqlite"
)

// File names inside the data directory. state.db holds everything that
// matters; cache.db holds TMDB responses and may be deleted at any time.
const (
	StateFileName = "state.db"
	CacheFileName = "cache.db"
)

const (
	sqliteDriverName = "sqlite"
	busyTimeoutMS    = 5000
	readerPoolSize   = 4
)

// Database is one SQLite file behind two pools. Write has a single connection,
// so writes are serialised in Go instead of contending for SQLite's lock; Read
// is a small pool that WAL lets run alongside the writer.
type Database struct {
	Write *sql.DB
	Read  *sql.DB
}

// OpenDatabase opens path, creating the file when it is missing. Every
// connection either pool opens runs the same pragmas, because SQLite applies
// them per connection and database/sql opens new ones whenever it likes.
func OpenDatabase(ctx context.Context, path string) (*Database, error) {
	// The writer opens first: it creates the file and switches it to WAL, which
	// a reader restricted to query_only could not do.
	writer, err := openPool(ctx, path, writerQuery(), 1)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	reader, err := openPool(ctx, path, readerQuery(), readerPoolSize)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open readers: %w", err), writer.Close())
	}
	return &Database{Write: writer, Read: reader}, nil
}

// Ping runs a trivial query on the read pool. It is the readiness check, so it
// must stay cheap: never an integrity scan.
func (d *Database) Ping(ctx context.Context) error {
	var one int
	return d.Read.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

// Close closes both pools.
func (d *Database) Close() error {
	return errors.Join(d.Read.Close(), d.Write.Close())
}

func connectionPragmas() url.Values {
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	return q
}

func writerQuery() url.Values {
	q := connectionPragmas()
	// BEGIN IMMEDIATE takes the write lock up front. A deferred transaction
	// that reads and then writes can fail with SQLITE_BUSY at the upgrade,
	// which busy_timeout does not retry.
	q.Set("_txlock", "immediate")
	return q
}

func readerQuery() url.Values {
	q := connectionPragmas()
	// Keeps every write on the single writer connection.
	q.Add("_pragma", "query_only(1)")
	return q
}

func openPool(ctx context.Context, path string, query url.Values, size int) (*sql.DB, error) {
	// A plain file name, not a file: URI: the driver strips its own parameters
	// off the query and SQLite never parses the path as URI syntax.
	db, err := sql.Open(sqliteDriverName, path+"?"+query.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(size)
	db.SetMaxIdleConns(size)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return db, nil
}
