package store

import (
	"context"
	"errors"
	"time"
)

// Names the bootstrap gives the lists it creates. The UI may label lists by
// kind instead; these are what a row carries when nothing else names it.
const (
	LibraryWatchlistName  = "Watchlist"
	LibraryCoupleListName = "Couple"
)

// BootstrapLibraryLists creates every account's watchlist and the single
// couple list where they are missing, in one transaction. It runs at startup
// once users holds the configured accounts, and is safe to run any number of
// times: the existence checks run inside the writer's IMMEDIATE transaction,
// so two bootstraps never both see a list as missing, and the partial unique
// indexes on lists refuse a duplicate even if they did.
func BootstrapLibraryLists(ctx context.Context, db *Database, now time.Time) (err error) {
	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	createdAt := now.Unix()
	if _, err = tx.ExecContext(ctx, `INSERT INTO lists (kind, owner_user_id, name, created_at)
		SELECT 'watchlist', u.id, ?, ? FROM users AS u
		WHERE NOT EXISTS (SELECT 1 FROM lists AS l WHERE l.kind = 'watchlist' AND l.owner_user_id = u.id)
		ORDER BY u.id`, LibraryWatchlistName, createdAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO lists (kind, owner_user_id, name, created_at)
		SELECT 'couple', NULL, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM lists WHERE kind = 'couple')`, LibraryCoupleListName, createdAt); err != nil {
		return err
	}
	return tx.Commit()
}
