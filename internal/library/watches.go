package library

import (
	"context"
	"database/sql"
	"net/http"
)

// The unique indexes on watches and ratings are partial, split by whether
// season_number is set, and SQLite only matches an ON CONFLICT target to a
// partial index that repeats its WHERE clause. So every upsert comes in two
// forms, and a bare target would fail at run time, not at compile time.
const (
	watchInsertWithoutSeason = `INSERT INTO watches (user_id, media_type, tmdb_id, season_number, scope, watched_at, created_at)
		VALUES (?1, ?2, ?3, NULL, ?4, ?5, ?5)
		ON CONFLICT (user_id, media_type, tmdb_id, scope) WHERE season_number IS NULL DO NOTHING`
	watchInsertWithSeason = `INSERT INTO watches (user_id, media_type, tmdb_id, season_number, scope, watched_at, created_at)
		VALUES (?1, ?2, ?3, ?6, ?4, ?5, ?5)
		ON CONFLICT (user_id, media_type, tmdb_id, scope, season_number) WHERE season_number IS NOT NULL DO NOTHING`
)

// libraryMutation is one watch or rating change on the caller's own rows.
type libraryMutation func(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget) error

// serveTargetMutation runs mutate and answers the title's state, both inside
// one transaction. The user id is always the session's.
func (s *Service) serveTargetMutation(w http.ResponseWriter, r *http.Request, target libraryTarget, mutate libraryMutation) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	var state titleStateBody
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		if err := mutate(r.Context(), tx, people.me.ID, target); err != nil {
			return err
		}
		var err error
		state, err = loadTitleState(r.Context(), tx, people, target.libraryTitle)
		return err
	})
	if err != nil {
		s.failed(w, r, err)
		return
	}
	libraryWriteJSON(w, http.StatusOK, state)
}

func (s *Service) handleWatchPut(w http.ResponseWriter, r *http.Request) {
	target, ok := decodeLibraryTarget(w, r)
	if !ok {
		return
	}
	s.serveTargetMutation(w, r, target, s.insertWatch)
}

func (s *Service) handleWatchDelete(w http.ResponseWriter, r *http.Request) {
	target, ok := decodeLibraryTarget(w, r)
	if !ok {
		return
	}
	s.serveTargetMutation(w, r, target, deleteWatch)
}

// insertWatch keeps the first watched_at when the row already exists, so
// marking twice is a no-op.
func (s *Service) insertWatch(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget) error {
	query := watchInsertWithoutSeason
	args := []any{userID, target.MediaType, target.TMDBID, target.Scope, s.now().Unix()}
	if target.Season != nil {
		query = watchInsertWithSeason
		args = append(args, *target.Season)
	}
	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func deleteWatch(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM watches
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ? AND scope = ? AND season_number IS ?`,
		userID, target.MediaType, target.TMDBID, target.Scope, libraryNullableSeason(target))
	return err
}

func libraryNullableSeason(target libraryTarget) sql.NullInt64 {
	if target.Season == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*target.Season), Valid: true}
}
