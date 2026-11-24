package library

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

// List kinds, as the schema names them.
const (
	listKindWatchlist = "watchlist"
	listKindCouple    = "couple"
	listKindCustom    = "custom"
)

// Who a list belongs to, from the caller's point of view.
const (
	listOwnerMe      = "me"
	listOwnerPartner = "partner"
	listOwnerShared  = "shared"
)

const listNameMaxChars = 100

// storedList is one lists row.
type storedList struct {
	ID        int64
	Kind      string
	Owner     sql.NullInt64
	Name      string
	CreatedAt int64
}

// canRename is the rule for renaming and deleting: only the owner, and only
// a custom list. Watchlists and the couple list are fixed.
func (l storedList) canRename(callerID int64) bool {
	return l.Kind == listKindCustom && l.Owner.Valid && l.Owner.Int64 == callerID
}

// canChangeItems is the rule for adding and removing items: the owner, or
// either person on the couple list. The partner's lists are read-only.
func (l storedList) canChangeItems(callerID int64) bool {
	return l.Kind == listKindCouple || (l.Owner.Valid && l.Owner.Int64 == callerID)
}

// listBody is a list as the lists page reads it.
type listBody struct {
	ID             int64  `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Owner          string `json:"owner"`
	ItemCount      int    `json:"itemCount"`
	CreatedAt      string `json:"createdAt"`
	CanRename      bool   `json:"canRename"`
	CanChangeItems bool   `json:"canChangeItems"`
}

func newListBody(list storedList, itemCount int, callerID int64) listBody {
	owner := listOwnerPartner
	switch {
	case list.Kind == listKindCouple:
		owner = listOwnerShared
	case list.Owner.Valid && list.Owner.Int64 == callerID:
		owner = listOwnerMe
	}
	return listBody{
		ID:             list.ID,
		Kind:           list.Kind,
		Name:           list.Name,
		Owner:          owner,
		ItemCount:      itemCount,
		CreatedAt:      libraryTimestamp(list.CreatedAt),
		CanRename:      list.canRename(callerID),
		CanChangeItems: list.canChangeItems(callerID),
	}
}

type listsBody struct {
	Results []listBody `json:"results"`
}

type listNameBody struct {
	Name string `json:"name"`
}

func (b listNameBody) validName() (string, string) {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		return "", "name must not be empty"
	}
	if utf8.RuneCountInString(name) > listNameMaxChars {
		return "", "name must be at most 100 characters"
	}
	return name, ""
}

// handleListsGet answers the caller's lists, the couple list and the
// partner's lists, in that order, each with its item count.
func (s *Service) handleListsGet(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Read.QueryContext(r.Context(), `SELECT l.id, l.kind, l.owner_user_id, l.name, l.created_at,
			(SELECT COUNT(*) FROM list_items AS li WHERE li.list_id = l.id)
		FROM lists AS l
		WHERE l.owner_user_id IN (?1, ?2) OR l.kind = 'couple'
		ORDER BY CASE WHEN l.owner_user_id = ?1 THEN 0 WHEN l.kind = 'couple' THEN 1 ELSE 2 END,
			CASE l.kind WHEN 'watchlist' THEN 0 ELSE 1 END, l.created_at, l.id`, people.me.ID, people.partner.ID)
	if err != nil {
		s.failed(w, r, err)
		return
	}
	defer func() { _ = rows.Close() }()
	body := listsBody{Results: []listBody{}}
	for rows.Next() {
		var list storedList
		var count int
		if err := rows.Scan(&list.ID, &list.Kind, &list.Owner, &list.Name, &list.CreatedAt, &count); err != nil {
			s.failed(w, r, err)
			return
		}
		body.Results = append(body.Results, newListBody(list, count, people.me.ID))
	}
	if err := rows.Err(); err != nil {
		s.failed(w, r, err)
		return
	}
	libraryWriteJSON(w, http.StatusOK, body)
}

func (s *Service) handleListCreate(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	var body listNameBody
	if !decodeLibraryBody(w, r, &body, true) {
		return
	}
	name, problem := body.validName()
	if problem != "" {
		libraryBadRequest(w, problem)
		return
	}
	list := storedList{Kind: listKindCustom, Owner: sql.NullInt64{Int64: people.me.ID, Valid: true}, Name: name, CreatedAt: s.now().Unix()}
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		return tx.QueryRowContext(r.Context(), `INSERT INTO lists (kind, owner_user_id, name, created_at)
			VALUES ('custom', ?, ?, ?) RETURNING id`, people.me.ID, name, list.CreatedAt).Scan(&list.ID)
	})
	if err != nil {
		s.failed(w, r, err)
		return
	}
	libraryWriteJSON(w, http.StatusCreated, newListBody(list, 0, people.me.ID))
}

// errListNotFound and errListForbidden end a list transaction with the
// answer the handler gives.
var (
	errListNotFound  = errors.New("list not found")
	errListForbidden = errors.New("list not the caller's to change")
)

func (s *Service) handleListRename(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	listID, ok := libraryPathListID(w, r)
	if !ok {
		return
	}
	var body listNameBody
	if !decodeLibraryBody(w, r, &body, true) {
		return
	}
	name, problem := body.validName()
	if problem != "" {
		libraryBadRequest(w, problem)
		return
	}
	var renamed listBody
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		list, err := lockedList(r.Context(), tx, listID, people.me.ID, storedList.canRename)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE lists SET name = ? WHERE id = ? AND owner_user_id = ? AND kind = 'custom'`,
			name, list.ID, people.me.ID); err != nil {
			return err
		}
		list.Name = name
		count, err := listItemCount(r.Context(), tx, list.ID)
		renamed = newListBody(list, count, people.me.ID)
		return err
	})
	s.answerListChange(w, r, err, func() { libraryWriteJSON(w, http.StatusOK, renamed) })
}

// handleListDelete answers 204 for a list that is already gone, so a
// repeated delete is a no-op like every other mutation.
func (s *Service) handleListDelete(w http.ResponseWriter, r *http.Request) {
	people, ok := s.people(w, r)
	if !ok {
		return
	}
	listID, ok := libraryPathListID(w, r)
	if !ok {
		return
	}
	err := s.inWriteTx(r.Context(), func(tx *sql.Tx) error {
		list, err := lockedList(r.Context(), tx, listID, people.me.ID, storedList.canRename)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(), `DELETE FROM lists WHERE id = ? AND owner_user_id = ? AND kind = 'custom'`, list.ID, people.me.ID)
		return err
	})
	if errors.Is(err, errListNotFound) {
		libraryNoContent(w)
		return
	}
	s.answerListChange(w, r, err, func() { libraryNoContent(w) })
}

// lockedList reads a list inside the writer's transaction and checks the
// caller may change it.
func lockedList(ctx context.Context, q libraryQueryer, listID, callerID int64, allowed func(storedList, int64) bool) (storedList, error) {
	list, err := loadList(ctx, q, listID)
	if err != nil {
		return storedList{}, err
	}
	if !allowed(list, callerID) {
		return storedList{}, errListForbidden
	}
	return list, nil
}

func loadList(ctx context.Context, q libraryQueryer, listID int64) (storedList, error) {
	var list storedList
	err := q.QueryRowContext(ctx, `SELECT id, kind, owner_user_id, name, created_at FROM lists WHERE id = ?`, listID).
		Scan(&list.ID, &list.Kind, &list.Owner, &list.Name, &list.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storedList{}, errListNotFound
	}
	return list, err
}

func listItemCount(ctx context.Context, q libraryQueryer, listID int64) (int, error) {
	var count int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM list_items WHERE list_id = ?`, listID).Scan(&count)
	return count, err
}

// answerListChange maps a list transaction's outcome to its answer.
func (s *Service) answerListChange(w http.ResponseWriter, r *http.Request, err error, success func()) {
	switch {
	case err == nil:
		success()
	case errors.Is(err, errListNotFound):
		libraryNotFound(w, "no such list")
	case errors.Is(err, errListForbidden):
		libraryForbidden(w, "only the owner changes a custom list; watchlists and the couple list are fixed, and the partner's lists are read-only")
	default:
		s.failed(w, r, err)
	}
}

func libraryPathListID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := libraryPositiveInt(r.PathValue("id"))
	if !ok {
		libraryBadRequest(w, "list id must be a positive integer")
	}
	return id, ok
}
