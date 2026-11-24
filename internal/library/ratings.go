package library

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"unicode/utf8"
)

// ratingReviewMaxChars is the schema's own bound on a review.
const ratingReviewMaxChars = 10000

// The rating upsert in its two partial-index forms. updated_at only moves when
// the level or the review actually changes, so repeating a PUT is a no-op
// and does not reorder history.
const (
	ratingUpsertWithoutSeason = `INSERT INTO ratings (user_id, media_type, tmdb_id, season_number, scope, level, scale, review, created_at, updated_at)
		VALUES (?1, ?2, ?3, NULL, ?4, ?5, ?6, ?7, ?8, ?8)
		ON CONFLICT (user_id, media_type, tmdb_id, scope) WHERE season_number IS NULL DO UPDATE SET ` + ratingUpsertSet
	ratingUpsertWithSeason = `INSERT INTO ratings (user_id, media_type, tmdb_id, season_number, scope, level, scale, review, created_at, updated_at)
		VALUES (?1, ?2, ?3, ?9, ?4, ?5, ?6, ?7, ?8, ?8)
		ON CONFLICT (user_id, media_type, tmdb_id, scope, season_number) WHERE season_number IS NOT NULL DO UPDATE SET ` + ratingUpsertSet
	ratingUpsertSet = `updated_at = CASE WHEN ratings.level IS excluded.level AND ratings.review IS excluded.review
			THEN ratings.updated_at ELSE excluded.updated_at END,
		level = excluded.level, scale = excluded.scale, review = excluded.review`
)

// ratingPutBody is a target plus the parts to set. An absent part is left as
// it is, so the level picker never clears a review; an empty string clears
// that part, and a rating left with neither part is removed.
type ratingPutBody struct {
	libraryTargetBody
	Level  *string `json:"level"`
	Review *string `json:"review"`
}

func (b ratingPutBody) validate() (libraryTarget, string) {
	target, problem := b.target()
	if problem != "" {
		return libraryTarget{}, problem
	}
	if b.Level == nil && b.Review == nil {
		return libraryTarget{}, "level or review is required"
	}
	if b.Level != nil && *b.Level != "" {
		if _, ok := LevelValue(*b.Level); !ok {
			return libraryTarget{}, "level must be one of hated, disliked, meh, fine, good, liked, loved"
		}
	}
	if b.Review != nil && utf8.RuneCountInString(*b.Review) > ratingReviewMaxChars {
		return libraryTarget{}, "review must be at most 10000 characters"
	}
	return target, ""
}

func (s *Service) handleRatingPut(w http.ResponseWriter, r *http.Request) {
	var body ratingPutBody
	if !decodeLibraryBody(w, r, &body, true) {
		return
	}
	target, problem := body.validate()
	if problem != "" {
		libraryBadRequest(w, problem)
		return
	}
	s.serveTargetMutation(w, r, target, func(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget) error {
		return s.upsertRating(ctx, tx, userID, target, body.Level, body.Review)
	})
}

func (s *Service) handleRatingDelete(w http.ResponseWriter, r *http.Request) {
	target, ok := decodeLibraryTarget(w, r)
	if !ok {
		return
	}
	s.serveTargetMutation(w, r, target, deleteRating)
}

// upsertRating merges the parts the body set into the stored row. The read and
// the write share the caller's IMMEDIATE transaction, so no concurrent write
// can land between them.
func (s *Service) upsertRating(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget, level, review *string) error {
	var stored struct{ level, review sql.NullString }
	err := tx.QueryRowContext(ctx, `SELECT level, review FROM ratings
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ? AND scope = ? AND season_number IS ?`,
		userID, target.MediaType, target.TMDBID, target.Scope, libraryNullableSeason(target)).Scan(&stored.level, &stored.review)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	merged := struct{ level, review sql.NullString }{ratingMergePart(stored.level, level), ratingMergePart(stored.review, review)}
	if !merged.level.Valid && !merged.review.Valid {
		return deleteRating(ctx, tx, userID, target)
	}
	var scale sql.NullString
	if merged.level.Valid {
		scale = sql.NullString{String: RatingScale, Valid: true}
	}
	query := ratingUpsertWithoutSeason
	args := []any{userID, target.MediaType, target.TMDBID, target.Scope, merged.level, scale, merged.review, s.now().Unix()}
	if target.Season != nil {
		query = ratingUpsertWithSeason
		args = append(args, *target.Season)
	}
	_, err = tx.ExecContext(ctx, query, args...)
	return err
}

func ratingMergePart(stored sql.NullString, sent *string) sql.NullString {
	switch {
	case sent == nil:
		return stored
	case *sent == "":
		return sql.NullString{}
	default:
		return sql.NullString{String: *sent, Valid: true}
	}
}

func deleteRating(ctx context.Context, tx *sql.Tx, userID int64, target libraryTarget) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM ratings
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ? AND scope = ? AND season_number IS ?`,
		userID, target.MediaType, target.TMDBID, target.Scope, libraryNullableSeason(target))
	return err
}
