package library

import (
	"context"
	"database/sql"
	"net/http"
	"unicode/utf8"

	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
)

const listItemNoteMaxChars = 1000

// listItemBody is one list item: the hydrated card plus who added it, when,
// and the note.
type listItemBody struct {
	hydrate.Card
	AddedAt   string  `json:"addedAt"`
	AddedByMe bool    `json:"addedByMe"`
	Note      *string `json:"note"`
}

// storedListItem is one list_items row.
type storedListItem struct {
	libraryTitle
	AddedBy int64
	AddedAt int64
	Note    sql.NullString
}

// listItemPutBody adds a title; a note given replaces the stored one, and an
// absent note keeps it.
type listItemPutBody struct {
	MediaType string  `json:"mediaType"`
	TMDBID    int     `json:"tmdbId"`
	Note      *string `json:"note"`
}

// listItemPutAnswer is what adding an item answers: the stored row, without
// hydration, which the list page does on its own read.
type listItemPutAnswer struct {
	ListID    int64   `json:"listId"`
	MediaType string  `json:"mediaType"`
	TMDBID    int     `json:"tmdbId"`
	AddedAt   string  `json:"addedAt"`
	AddedByMe bool    `json:"addedByMe"`
	Note      *string `json:"note"`
}

func (s *Service) handleListItemsGet(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	listID, ok := libraryPathListID(w, r)
	if !ok {
		return
	}
	var after listItemCursor
	given, ok := decodeLibraryCursor(w, r, &after)
	if !ok {
		return
	}
	// Every list is readable: each belongs to one of the two people or to
	// both, and each sees the other's lists read-only.
	if _, err := loadList(r.Context(), s.db.Read, listID); err != nil {
		s.answerListChange(w, r, err, nil)
		return
	}
	items, err := loadListItemsPage(r.Context(), s.db.Read, listID, given, after)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	items, next := libraryPageOf(items, func(item storedListItem) any {
		return listItemCursor{AddedAt: item.AddedAt, MediaType: item.MediaType, TMDBID: item.TMDBID}
	})
	libraryWriteJSON(w, http.StatusOK, cursorPage[listItemBody]{Results: s.hydrateListItems(r.Context(), items, people.me.ID), NextCursor: next})
}

func (s *Service) hydrateListItems(ctx context.Context, items []storedListItem, callerID int64) []listItemBody {
	titles := make([]hydrate.Title, 0, len(items))
	for _, item := range items {
		titles = append(titles, hydrate.Title{MediaType: item.MediaType, TMDBID: item.TMDBID})
	}
	cards := s.cards.Cards(ctx, titles)
	bodies := make([]listItemBody, 0, len(items))
	for i, item := range items {
		bodies = append(bodies, listItemBody{
			Card:      cards[i],
			AddedAt:   libraryTimestamp(item.AddedAt),
			AddedByMe: item.AddedBy == callerID,
			Note:      libraryOptionalString(item.Note),
		})
	}
	return bodies
}

// loadListItemsPage fetches one row past the page, so the caller knows
// whether another page follows.
func loadListItemsPage(ctx context.Context, q libraryQueryer, listID int64, given bool, after listItemCursor) ([]storedListItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT media_type, tmdb_id, added_by_user_id, added_at, note FROM list_items
		WHERE list_id = ?1 AND (?2 = 0 OR added_at < ?3 OR (added_at = ?3 AND (media_type, tmdb_id) > (?4, ?5)))
		ORDER BY added_at DESC, media_type ASC, tmdb_id ASC
		LIMIT ?6`, listID, given, after.AddedAt, after.MediaType, after.TMDBID, libraryPageSize+1)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []storedListItem{}
	for rows.Next() {
		var item storedListItem
		if err := rows.Scan(&item.MediaType, &item.TMDBID, &item.AddedBy, &item.AddedAt, &item.Note); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) handleListItemPut(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	listID, ok := libraryPathListID(w, r)
	if !ok {
		return
	}
	var body listItemPutBody
	if !decodeLibraryBody(w, r, &body, true) {
		return
	}
	title, problem := libraryTitleOf(body.MediaType, body.TMDBID)
	if problem == "" && body.Note != nil && utf8.RuneCountInString(*body.Note) > listItemNoteMaxChars {
		problem = "note must be at most 1000 characters"
	}
	if problem != "" {
		libraryBadRequest(w, problem)
		return
	}
	var stored storedListItem
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		if _, err := lockedList(r.Context(), tx, listID, people.me.ID, storedList.canChangeItems); err != nil {
			return err
		}
		var err error
		stored, err = s.addListItem(r.Context(), tx, listID, people.me.ID, title, body.Note)
		return err
	})
	s.answerListChange(w, r, err, func() {
		libraryWriteJSON(w, http.StatusOK, listItemPutAnswer{
			ListID: listID, MediaType: stored.MediaType, TMDBID: stored.TMDBID,
			AddedAt: libraryTimestamp(stored.AddedAt), AddedByMe: stored.AddedBy == people.me.ID, Note: libraryOptionalString(stored.Note),
		})
	})
}

// addListItem keeps the first added_at and adder when the title is already
// on the list, so adding twice leaves one row.
func (s *Service) addListItem(ctx context.Context, tx *sql.Tx, listID, callerID int64, title libraryTitle, note *string) (storedListItem, error) {
	var noteValue sql.NullString
	if note != nil {
		noteValue = sql.NullString{String: *note, Valid: *note != ""}
	}
	stored := storedListItem{libraryTitle: title}
	err := tx.QueryRowContext(ctx, `INSERT INTO list_items (list_id, media_type, tmdb_id, added_by_user_id, added_at, note)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6)
		ON CONFLICT (list_id, media_type, tmdb_id) DO UPDATE SET note = CASE WHEN ?7 THEN excluded.note ELSE list_items.note END
		RETURNING added_by_user_id, added_at, note`,
		listID, title.MediaType, title.TMDBID, callerID, s.now().Unix(), noteValue, note != nil).
		Scan(&stored.AddedBy, &stored.AddedAt, &stored.Note)
	return stored, err
}

// handleListItemDelete answers 204 whether or not the title was on the list.
func (s *Service) handleListItemDelete(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	listID, ok := libraryPathListID(w, r)
	if !ok {
		return
	}
	title, ok := libraryPathTitle(w, r)
	if !ok {
		return
	}
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		if _, err := lockedList(r.Context(), tx, listID, people.me.ID, storedList.canChangeItems); err != nil {
			return err
		}
		return removeListItem(r.Context(), tx, listID, title)
	})
	s.answerListChange(w, r, err, func() { libraryNoContent(w) })
}

func removeListItem(ctx context.Context, tx *sql.Tx, listID int64, title libraryTitle) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM list_items WHERE list_id = ? AND media_type = ? AND tmdb_id = ?`,
		listID, title.MediaType, title.TMDBID)
	return err
}

func libraryTitleOf(mediaType string, tmdbID int) (libraryTitle, string) {
	if !libraryMediaTypeKnown(mediaType) {
		return libraryTitle{}, "mediaType must be movie or tv"
	}
	if tmdbID <= 0 {
		return libraryTitle{}, "tmdbId must be a positive integer"
	}
	return libraryTitle{MediaType: mediaType, TMDBID: tmdbID}, ""
}
