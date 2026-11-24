package library

import (
	"context"
	"database/sql"
	"net/http"
	"slices"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
)

// Series status values. A show row means the whole series was watched and
// takes precedence; season rows alone mean it is in progress.
const (
	seriesStatusWatched    = "watched"
	seriesStatusInProgress = "in_progress"
	seriesStatusUnwatched  = "unwatched"
)

// titleStateBody answers GET /library/title and every watch and rating
// mutation: both people's state for the title and its seasons, and the
// caller's list memberships.
type titleStateBody struct {
	MediaType string           `json:"mediaType"`
	TMDBID    int              `json:"tmdbId"`
	Me        personTitleState `json:"me"`
	Partner   personTitleState `json:"partner"`
	Lists     []listMembership `json:"lists"`
}

// personTitleState is one person's state. Watched, WatchedAt and Rating are
// the title-level row (the movie, or the whole show); Seasons holds every
// season with a watch or a rating, in season order.
type personTitleState struct {
	DisplayName string  `json:"displayName"`
	Status      string  `json:"status"`
	Watched     bool    `json:"watched"`
	WatchedAt   *string `json:"watchedAt"`
	Rating      *rating `json:"rating"`
	// RegularSeasonsWatched counts watched seasons other than specials
	// (season 0), which never count towards completing a series.
	RegularSeasonsWatched int           `json:"regularSeasonsWatched"`
	Seasons               []seasonState `json:"seasons"`
}

type seasonState struct {
	SeasonNumber int     `json:"seasonNumber"`
	Watched      bool    `json:"watched"`
	WatchedAt    *string `json:"watchedAt"`
	Rating       *rating `json:"rating"`
}

// rating is a stored rating: a level, a review, or both.
type rating struct {
	Level     *string `json:"level"`
	Review    *string `json:"review"`
	UpdatedAt string  `json:"updatedAt"`
}

type listMembership struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// seriesStatus is the series state machine. Show and season rows coexist
// independently: marking every regular season never creates a show row, and
// removing the show row leaves the season rows, so the status falls back to
// in progress.
func seriesStatus(showWatched bool, watchedSeasons []int) (status string, regularSeasons int) {
	for _, season := range watchedSeasons {
		if season > 0 {
			regularSeasons++
		}
	}
	switch {
	case showWatched:
		return seriesStatusWatched, regularSeasons
	case len(watchedSeasons) > 0:
		return seriesStatusInProgress, regularSeasons
	default:
		return seriesStatusUnwatched, regularSeasons
	}
}

func (s *Service) handleTitleState(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	title, ok := libraryPathTitle(w, r)
	if !ok {
		return
	}
	state, err := loadTitleState(r.Context(), s.db.Read, people, title)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	libraryWriteJSON(w, http.StatusOK, state)
}

// loadTitleState reads through q, so a mutation answers with the state its
// own transaction just wrote.
func loadTitleState(ctx context.Context, q libraryQueryer, people libraryPeople, title libraryTitle) (titleStateBody, error) {
	me, err := loadPersonTitleState(ctx, q, people.me, title)
	if err != nil {
		return titleStateBody{}, err
	}
	partner, err := loadPersonTitleState(ctx, q, people.partner, title)
	if err != nil {
		return titleStateBody{}, err
	}
	lists, err := loadListMemberships(ctx, q, people.me.ID, title)
	if err != nil {
		return titleStateBody{}, err
	}
	return titleStateBody{MediaType: title.MediaType, TMDBID: title.TMDBID, Me: me, Partner: partner, Lists: lists}, nil
}

func loadPersonTitleState(ctx context.Context, q libraryQueryer, person auth.User, title libraryTitle) (personTitleState, error) {
	state := personTitleState{DisplayName: person.DisplayName, Seasons: []seasonState{}}
	seasons := map[int]*seasonState{}
	seasonOf := func(number int) *seasonState {
		if seasons[number] == nil {
			seasons[number] = &seasonState{SeasonNumber: number}
		}
		return seasons[number]
	}
	var watchedSeasons []int
	err := scanTitleWatches(ctx, q, person.ID, title, func(season sql.NullInt64, watchedAt int64) {
		at := libraryTimestamp(watchedAt)
		if !season.Valid {
			state.Watched, state.WatchedAt = true, &at
			return
		}
		entry := seasonOf(int(season.Int64))
		entry.Watched, entry.WatchedAt = true, &at
		watchedSeasons = append(watchedSeasons, int(season.Int64))
	})
	if err != nil {
		return personTitleState{}, err
	}
	err = scanTitleRatings(ctx, q, person.ID, title, func(season sql.NullInt64, stored rating) {
		if !season.Valid {
			state.Rating = &stored
			return
		}
		seasonOf(int(season.Int64)).Rating = &stored
	})
	if err != nil {
		return personTitleState{}, err
	}
	state.Status, state.RegularSeasonsWatched = seriesStatus(state.Watched, watchedSeasons)
	numbers := make([]int, 0, len(seasons))
	for number := range seasons {
		numbers = append(numbers, number)
	}
	slices.Sort(numbers)
	for _, number := range numbers {
		state.Seasons = append(state.Seasons, *seasons[number])
	}
	return state, nil
}

func scanTitleWatches(ctx context.Context, q libraryQueryer, userID int64, title libraryTitle, each func(season sql.NullInt64, watchedAt int64)) error {
	rows, err := q.QueryContext(ctx, `SELECT season_number, watched_at FROM watches
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ?`, userID, title.MediaType, title.TMDBID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var season sql.NullInt64
		var watchedAt int64
		if err := rows.Scan(&season, &watchedAt); err != nil {
			return err
		}
		each(season, watchedAt)
	}
	return rows.Err()
}

func scanTitleRatings(ctx context.Context, q libraryQueryer, userID int64, title libraryTitle, each func(season sql.NullInt64, stored rating)) error {
	rows, err := q.QueryContext(ctx, `SELECT season_number, level, review, updated_at FROM ratings
		WHERE user_id = ? AND media_type = ? AND tmdb_id = ?`, userID, title.MediaType, title.TMDBID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var season sql.NullInt64
		var level, review sql.NullString
		var updatedAt int64
		if err := rows.Scan(&season, &level, &review, &updatedAt); err != nil {
			return err
		}
		each(season, rating{Level: libraryOptionalString(level), Review: libraryOptionalString(review), UpdatedAt: libraryTimestamp(updatedAt)})
	}
	return rows.Err()
}

// loadListMemberships lists the caller's lists and the couple list that hold
// the title; the partner's own lists are not the caller's memberships.
func loadListMemberships(ctx context.Context, q libraryQueryer, userID int64, title libraryTitle) ([]listMembership, error) {
	rows, err := q.QueryContext(ctx, `SELECT l.id, l.kind, l.name FROM lists AS l
		JOIN list_items AS li ON li.list_id = l.id
		WHERE li.media_type = ? AND li.tmdb_id = ? AND (l.owner_user_id = ? OR l.kind = 'couple')
		ORDER BY CASE l.kind WHEN 'watchlist' THEN 0 WHEN 'couple' THEN 1 ELSE 2 END, l.id`,
		title.MediaType, title.TMDBID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	memberships := []listMembership{}
	for rows.Next() {
		var m listMembership
		if err := rows.Scan(&m.ID, &m.Kind, &m.Name); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func libraryOptionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
