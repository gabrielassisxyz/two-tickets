package library

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
)

// History is a current-state index: one row per title, scope and season the
// caller has a watch or a rating for, showing both when both exist. Removing
// the state removes the row; nothing about past edits is kept. activity_at is
// the later of the row's watch and its rating's last change.
const historyPageQuery = `WITH keyed AS (
		SELECT media_type, tmdb_id, scope, season_number FROM watches WHERE user_id = ?1
		UNION
		SELECT media_type, tmdb_id, scope, season_number FROM ratings WHERE user_id = ?1
	), state AS (
		SELECT k.media_type, k.tmdb_id, k.scope, k.season_number, COALESCE(k.season_number, -1) AS season_key,
			w.watched_at, r.level, r.review, r.updated_at AS rated_at,
			MAX(COALESCE(w.watched_at, 0), COALESCE(r.updated_at, 0)) AS activity_at
		FROM keyed AS k
		LEFT JOIN watches AS w ON w.user_id = ?1 AND w.media_type = k.media_type AND w.tmdb_id = k.tmdb_id
			AND w.scope = k.scope AND w.season_number IS k.season_number
		LEFT JOIN ratings AS r ON r.user_id = ?1 AND r.media_type = k.media_type AND r.tmdb_id = k.tmdb_id
			AND r.scope = k.scope AND r.season_number IS k.season_number
	)
	SELECT media_type, tmdb_id, scope, season_number, season_key, watched_at, level, review, rated_at, activity_at
	FROM state
	WHERE (?2 = '' OR level = ?2)
		AND (?3 = 0 OR activity_at < ?4
			OR (activity_at = ?4 AND (media_type, tmdb_id, scope, season_key) > (?5, ?6, ?7, ?8)))
	ORDER BY activity_at DESC, media_type ASC, tmdb_id ASC, scope ASC, season_key ASC
	LIMIT ?9`

// historyRowBody is one history row: the hydrated card plus the state.
type historyRowBody struct {
	hydrate.Card
	Scope        string  `json:"scope"`
	SeasonNumber *int    `json:"seasonNumber"`
	ActivityAt   string  `json:"activityAt"`
	Watched      bool    `json:"watched"`
	WatchedAt    *string `json:"watchedAt"`
	Rating       *rating `json:"rating"`
}

type storedHistoryRow struct {
	libraryTitle
	Scope      string
	Season     sql.NullInt64
	SeasonKey  int
	WatchedAt  sql.NullInt64
	Level      sql.NullString
	Review     sql.NullString
	RatedAt    sql.NullInt64
	ActivityAt int64
}

func (s *Service) handleHistoryGet(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	level := r.URL.Query().Get("level")
	if _, known := LevelValue(level); level != "" && !known {
		libraryBadRequest(w, "level must be one of hated, disliked, meh, fine, good, liked, loved")
		return
	}
	var after historyCursor
	given, ok := decodeLibraryCursor(w, r, &after)
	if !ok {
		return
	}
	rows, err := loadHistoryPage(r.Context(), s.db.Read, people.me.ID, level, given, after)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	rows, next := libraryPageOf(rows, func(row storedHistoryRow) any {
		return historyCursor{ActivityAt: row.ActivityAt, MediaType: row.MediaType, TMDBID: row.TMDBID, Scope: row.Scope, Season: row.SeasonKey}
	})
	libraryWriteJSON(w, http.StatusOK, cursorPage[historyRowBody]{Results: s.hydrateHistory(r.Context(), rows), NextCursor: next})
}

func loadHistoryPage(ctx context.Context, q libraryQueryer, userID int64, level string, given bool, after historyCursor) ([]storedHistoryRow, error) {
	rows, err := q.QueryContext(ctx, historyPageQuery, userID, level, given,
		after.ActivityAt, after.MediaType, after.TMDBID, after.Scope, after.Season, libraryPageSize+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	history := []storedHistoryRow{}
	for rows.Next() {
		var row storedHistoryRow
		if err := rows.Scan(&row.MediaType, &row.TMDBID, &row.Scope, &row.Season, &row.SeasonKey,
			&row.WatchedAt, &row.Level, &row.Review, &row.RatedAt, &row.ActivityAt); err != nil {
			return nil, err
		}
		history = append(history, row)
	}
	return history, rows.Err()
}

func (s *Service) hydrateHistory(ctx context.Context, rows []storedHistoryRow) []historyRowBody {
	titles := make([]hydrate.Title, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, hydrate.Title{MediaType: row.MediaType, TMDBID: row.TMDBID})
	}
	cards := s.cards.Cards(ctx, titles)
	bodies := make([]historyRowBody, 0, len(rows))
	for i, row := range rows {
		body := historyRowBody{
			Card:       cards[i],
			Scope:      row.Scope,
			ActivityAt: libraryTimestamp(row.ActivityAt),
			Watched:    row.WatchedAt.Valid,
			WatchedAt:  libraryOptionalTimestamp(row.WatchedAt),
		}
		if row.Season.Valid {
			season := int(row.Season.Int64)
			body.SeasonNumber = &season
		}
		if row.RatedAt.Valid {
			body.Rating = &rating{Level: libraryOptionalString(row.Level), Review: libraryOptionalString(row.Review), UpdatedAt: libraryTimestamp(row.RatedAt.Int64)}
		}
		bodies = append(bodies, body)
	}
	return bodies
}
