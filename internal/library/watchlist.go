package library

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/respond"
)

// The Seerr watchlist routes the kept stars call, backed by the caller's
// watchlist list. They follow upstream's server/routes/watchlist.ts and the
// watchlist branch of server/routes/discover.ts at the pinned commit, with
// one deliberate difference: adding a title already there answers 201 with
// the same item where upstream answers 409, and removing a title that is not
// there answers 204 where upstream answers 404, so both are idempotent.

// seerrWatchlistItem is upstream's Watchlist entity as POST answers it. id is
// the TMDB id, the same id /discover/watchlist answers.
type seerrWatchlistItem struct {
	ID        int    `json:"id"`
	RatingKey string `json:"ratingKey"`
	TMDBID    int    `json:"tmdbId"`
	MediaType string `json:"mediaType"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// seerrWatchlistEntry is one /discover/watchlist result, upstream's
// WatchlistItem.
type seerrWatchlistEntry struct {
	ID        int    `json:"id"`
	RatingKey string `json:"ratingKey"`
	TMDBID    int    `json:"tmdbId"`
	MediaType string `json:"mediaType"`
	Title     string `json:"title"`
}

type seerrWatchlistAddBody struct {
	TMDBID    int    `json:"tmdbId"`
	MediaType string `json:"mediaType"`
	Title     string `json:"title"`
}

// seerrRatingKey is the stable local form of upstream's Plex rating key: the
// media type and the TMDB id, which together name one title.
func seerrRatingKey(title libraryTitle) string {
	return title.MediaType + "-" + strconv.Itoa(title.TMDBID)
}

// callerWatchlistID returns the caller's watchlist. The bootstrap creates it
// before the process serves, so a missing one is a fault, never a reason to
// create it here.
func callerWatchlistID(ctx context.Context, q libraryQueryer, userID int64) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM lists WHERE kind = 'watchlist' AND owner_user_id = ?`, userID).Scan(&id)
	return id, err
}

func (s *Service) handleSeerrWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	var body seerrWatchlistAddBody
	if !decodeLibraryBody(w, r, &body, false) {
		return
	}
	title, problem := libraryTitleOf(body.MediaType, body.TMDBID)
	if problem != "" {
		libraryBadRequest(w, problem)
		return
	}
	var stored storedListItem
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		listID, err := callerWatchlistID(r.Context(), tx, me.ID)
		if err != nil {
			return err
		}
		stored, err = s.addListItem(r.Context(), tx, listID, me.ID, title, nil)
		return err
	})
	if err != nil {
		s.failed(w, r, err)
		return
	}
	added := libraryTimestamp(stored.AddedAt)
	libraryWriteJSON(w, http.StatusCreated, seerrWatchlistItem{
		ID: title.TMDBID, RatingKey: seerrRatingKey(title), TMDBID: title.TMDBID, MediaType: title.MediaType,
		Title: body.Title, CreatedAt: added, UpdatedAt: added,
	})
}

func (s *Service) handleSeerrWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	mediaType := r.URL.Query().Get("mediaType")
	if !libraryMediaTypeKnown(mediaType) {
		respond.Error(w, http.StatusBadRequest, "Invalid mediaType query parameter.")
		return
	}
	id, ok := libraryPositiveInt(r.PathValue("tmdbId"))
	if !ok {
		libraryBadRequest(w, "tmdbId must be a positive integer")
		return
	}
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		listID, err := callerWatchlistID(r.Context(), tx, me.ID)
		if err != nil {
			return err
		}
		return removeListItem(r.Context(), tx, listID, libraryTitle{MediaType: mediaType, TMDBID: int(id)})
	})
	if err != nil {
		s.failed(w, r, err)
		return
	}
	libraryNoContent(w)
}

// handleSeerrWatchlistPage answers upstream's page envelope, 20 items a page
// in the list order, each title through hydration.
func (s *Service) handleSeerrWatchlistPage(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		value, ok := libraryPositiveInt(raw)
		if !ok {
			libraryBadRequest(w, "page must be a positive integer")
			return
		}
		page = int(value)
	}
	items, total, err := loadSeerrWatchlistPage(r.Context(), s.db.Read, me.ID, page)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	titles := make([]hydrate.Title, 0, len(items))
	for _, item := range items {
		titles = append(titles, hydrate.Title{MediaType: item.MediaType, TMDBID: item.TMDBID})
	}
	cards := s.cards.Cards(r.Context(), titles)
	entries := make([]seerrWatchlistEntry, 0, len(items))
	for i, item := range items {
		entries = append(entries, seerrWatchlistEntry{
			ID: item.TMDBID, RatingKey: seerrRatingKey(item), TMDBID: item.TMDBID, MediaType: item.MediaType, Title: cards[i].DisplayTitle(),
		})
	}
	totalPages := max(1, (total+libraryPageSize-1)/libraryPageSize)
	respond.JSON(w, catalog.NewResultsPage(page, totalPages, total, entries))
}

func loadSeerrWatchlistPage(ctx context.Context, q libraryQueryer, userID int64, page int) ([]libraryTitle, int, error) {
	listID, err := callerWatchlistID(ctx, q, userID)
	if err != nil {
		return nil, 0, err
	}
	total, err := listItemCount(ctx, q, listID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.QueryContext(ctx, `SELECT media_type, tmdb_id FROM list_items WHERE list_id = ?
		ORDER BY added_at DESC, media_type ASC, tmdb_id ASC LIMIT ? OFFSET ?`,
		listID, libraryPageSize, int64(page-1)*libraryPageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	titles := []libraryTitle{}
	for rows.Next() {
		var title libraryTitle
		if err := rows.Scan(&title.MediaType, &title.TMDBID); err != nil {
			return nil, 0, err
		}
		titles = append(titles, title)
	}
	return titles, total, rows.Err()
}

// OnCallerWatchlist answers the catalog's membership question for the account
// ctx carries, in one query however many titles a page holds. Without an
// account nothing is on a watchlist.
func (s *Service) OnCallerWatchlist(ctx context.Context, titles []catalog.WatchlistTitle) (map[catalog.WatchlistTitle]bool, error) {
	found := map[catalog.WatchlistTitle]bool{}
	me, ok := auth.UserFromContext(ctx)
	if !ok || len(titles) == 0 {
		return found, nil
	}
	values := make([]string, 0, len(titles))
	args := []any{me.ID}
	for _, title := range titles {
		values = append(values, "(?, ?)")
		args = append(args, title.MediaType, title.TMDBID)
	}
	rows, err := s.db.Read.QueryContext(ctx, `SELECT li.media_type, li.tmdb_id FROM list_items AS li
		JOIN lists AS l ON l.id = li.list_id
		WHERE l.kind = 'watchlist' AND l.owner_user_id = ?
			AND (li.media_type, li.tmdb_id) IN (VALUES `+strings.Join(values, ", ")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var title catalog.WatchlistTitle
		if err := rows.Scan(&title.MediaType, &title.TMDBID); err != nil {
			return nil, err
		}
		found[title] = true
	}
	return found, rows.Err()
}
