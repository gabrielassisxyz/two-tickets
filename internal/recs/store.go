// Package recs persists recommender runs: one personalised computation is a
// rec_runs row plus its rec_results, published atomically by setting
// completed_at, and rec_exposures records which ranks entered the viewport.
// The algorithm itself lives elsewhere; this package owns only the storage
// rules named in 0004_recs.sql.
package recs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Result is one row of rec_results. Reasons holds the seed titles that
// produced it.
type Result struct {
	Rank      int64
	MediaType string
	TmdbID    int64
	Score     float64
	Reasons   []string
}

// Page is one read of the newest completed run: the total before pagination
// plus the requested slice.
type Page struct {
	RunID   int64
	Total   int
	Results []Result
}

// CreateRun inserts a rec_runs row with completed_at NULL. It returns the new
// run id. Callers add results and then PublishRun; a run that never publishes
// stays invisible to ReadLatest.
func CreateRun(ctx context.Context, db *sql.DB, userID int64, algorithm, watermark string, startedAt int64) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `INSERT INTO rec_runs (user_id, algorithm, watermark, started_at, completed_at)
		VALUES (?, ?, ?, ?, NULL) RETURNING id`, userID, algorithm, watermark, startedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert rec_run: %w", err)
	}
	return id, nil
}

// AddResults inserts every result of one run. Reasons that are nil encode as
// an empty JSON array, so the rec_results_reasons_is_json_array check holds.
func AddResults(ctx context.Context, db *sql.DB, runID int64, results []Result) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	for _, r := range results {
		reasons := r.Reasons
		if reasons == nil {
			reasons = []string{}
		}
		encoded, err := json.Marshal(reasons)
		if err != nil {
			return fmt.Errorf("encode reasons: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO rec_results (run_id, rank, media_type, tmdb_id, score, reasons)
			VALUES (?, ?, ?, ?, ?, ?)`, runID, r.Rank, r.MediaType, r.TmdbID, r.Score, string(encoded)); err != nil {
			return fmt.Errorf("insert rec_result rank %d: %w", r.Rank, err)
		}
	}
	return tx.Commit()
}

// PublishRun makes a run visible by setting completed_at. It runs after every
// result is in, in its own statement, and refuses a run that is already
// published or missing, so a failed computation can never replace the newest
// completed run.
func PublishRun(ctx context.Context, db *sql.DB, runID int64, completedAt int64) error {
	res, err := db.ExecContext(ctx, `UPDATE rec_runs SET completed_at = ? WHERE id = ? AND completed_at IS NULL`, completedAt, runID)
	if err != nil {
		return fmt.Errorf("publish rec_run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("publish rec_run %d: no unpublished run", runID)
	}
	return nil
}

// ReadLatest returns the newest completed run for a user. A run with
// completed_at NULL is invisible. mediaType filters on the server before
// pagination, so Total counts the run's matching results: "" means all,
// "movie" or "tv" a single type. limit <= 0 means no limit. An unknown user,
// or a user with no completed run, yields an empty page and no error.
func ReadLatest(ctx context.Context, db *sql.DB, userID int64, mediaType string, limit, offset int) (Page, error) {
	if mediaType != "" && mediaType != "movie" && mediaType != "tv" {
		return Page{}, fmt.Errorf("unknown media type %q", mediaType)
	}
	if offset < 0 {
		offset = 0
	}
	var runID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM rec_runs
		WHERE user_id = ? AND completed_at IS NOT NULL
		ORDER BY completed_at DESC, id DESC LIMIT 1`, userID).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, nil
	}
	if err != nil {
		return Page{}, fmt.Errorf("newest completed run: %w", err)
	}
	filter := ""
	args := []any{runID}
	if mediaType != "" {
		filter = " AND media_type = ?"
		args = append(args, mediaType)
	}
	var total int
	countQuery := "SELECT count(*) FROM rec_results WHERE run_id = ?" + filter
	countArgs := append([]any{}, args...)
	if err := db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count rec_results: %w", err)
	}
	query := "SELECT rank, media_type, tmdb_id, score, reasons FROM rec_results WHERE run_id = ?" + filter + " ORDER BY rank ASC"
	queryArgs := append([]any{}, args...)
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		queryArgs = append(queryArgs, limit, offset)
	} else if offset > 0 {
		query += " LIMIT -1 OFFSET ?"
		queryArgs = append(queryArgs, offset)
	}
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return Page{}, fmt.Errorf("read rec_results: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := Page{RunID: runID, Total: total}
	for rows.Next() {
		var r Result
		var encoded string
		if err := rows.Scan(&r.Rank, &r.MediaType, &r.TmdbID, &r.Score, &encoded); err != nil {
			return Page{}, err
		}
		if err := json.Unmarshal([]byte(encoded), &r.Reasons); err != nil {
			return Page{}, fmt.Errorf("decode reasons: %w", err)
		}
		page.Results = append(page.Results, r)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return page, nil
}

// RecordExposure stores that a card entered the viewport. The primary key on
// (run_id, rank) makes the write idempotent: a repeat resolves the conflict
// by doing nothing and returns no error.
func RecordExposure(ctx context.Context, db *sql.DB, runID, rank, userID int64, exposedAt int64) error {
	_, err := db.ExecContext(ctx, `INSERT INTO rec_exposures (run_id, rank, user_id, exposed_at)
		VALUES (?, ?, ?, ?) ON CONFLICT (run_id, rank) DO NOTHING`, runID, rank, userID, exposedAt)
	if err != nil {
		return fmt.Errorf("record rec_exposure: %w", err)
	}
	return nil
}

// Watermark builds the value a run stores: the user's rating count, newest
// ratings.updated_at, watch count and newest watches.created_at, each zero
// when the table holds no row for the user. Counts are part of it so that a
// deleted row also moves the watermark.
func Watermark(ctx context.Context, db *sql.DB, userID int64) (string, error) {
	var ratingsCount, ratingsMax, watchesCount, watchesMax int64
	err := db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM ratings WHERE user_id = ?),
		COALESCE((SELECT max(updated_at) FROM ratings WHERE user_id = ?), 0),
		(SELECT count(*) FROM watches WHERE user_id = ?),
		COALESCE((SELECT max(created_at) FROM watches WHERE user_id = ?), 0)`,
		userID, userID, userID, userID).Scan(&ratingsCount, &ratingsMax, &watchesCount, &watchesMax)
	if err != nil {
		return "", fmt.Errorf("watermark: %w", err)
	}
	return fmt.Sprintf("%d:%d:%d:%d", ratingsCount, ratingsMax, watchesCount, watchesMax), nil
}

// WatermarkMoved reports whether the user's ratings or watches changed since
// the newest completed run: true when the current watermark differs from the
// run's, and true when the user has no completed run yet. Another user's
// change never moves it, because both watermarks read that user only.
func WatermarkMoved(ctx context.Context, db *sql.DB, userID int64) (bool, error) {
	current, err := Watermark(ctx, db, userID)
	if err != nil {
		return false, err
	}
	var stored string
	err = db.QueryRowContext(ctx, `SELECT watermark FROM rec_runs
		WHERE user_id = ? AND completed_at IS NOT NULL
		ORDER BY completed_at DESC, id DESC LIMIT 1`, userID).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("newest run watermark: %w", err)
	}
	return current != stored, nil
}
