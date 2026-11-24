// Package library serves each person's watched state, ratings, reviews,
// lists and history, the partner's read-only view of them, and the Seerr
// watchlist routes the kept frontend calls.
//
// Every route runs behind httpapi's session gate and origin check. Every
// mutation acts on the caller, whom the session names; the partner is derived
// on the server through PartnerResolver. No route reads a user id from the
// client, so nothing a request carries can name whose rows it changes.
// Mutations are idempotent and each is one transaction on the writer.
package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// PartnerResolver derives the other account. *auth.Service satisfies it.
type PartnerResolver interface {
	PartnerOf(userID int64) (auth.User, error)
}

// Service is the library over state.db.
type Service struct {
	db       *store.Database
	partners PartnerResolver
	cards    *hydrate.Hydrator
	logger   *slog.Logger
	now      func() time.Time
}

// NewService builds the library. A nil logger discards; a nil now means
// time.Now.
func NewService(db *store.Database, partners PartnerResolver, cards *hydrate.Hydrator, logger *slog.Logger, now func() time.Time) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, partners: partners, cards: cards, logger: logger, now: now}
}

// Register adds the library routes and the Seerr watchlist routes to mux.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/library/title/{mediaType}/{tmdbId}", s.handleTitleState)
	mux.HandleFunc("PUT /api/v1/library/watch", s.handleWatchPut)
	mux.HandleFunc("DELETE /api/v1/library/watch", s.handleWatchDelete)
	mux.HandleFunc("PUT /api/v1/library/rating", s.handleRatingPut)
	mux.HandleFunc("DELETE /api/v1/library/rating", s.handleRatingDelete)
	mux.HandleFunc("GET /api/v1/library/lists", s.handleListsGet)
	mux.HandleFunc("POST /api/v1/library/lists", s.handleListCreate)
	mux.HandleFunc("PATCH /api/v1/library/lists/{id}", s.handleListRename)
	mux.HandleFunc("DELETE /api/v1/library/lists/{id}", s.handleListDelete)
	mux.HandleFunc("GET /api/v1/library/lists/{id}/items", s.handleListItemsGet)
	mux.HandleFunc("PUT /api/v1/library/lists/{id}/items", s.handleListItemPut)
	mux.HandleFunc("DELETE /api/v1/library/lists/{id}/items/{mediaType}/{tmdbId}", s.handleListItemDelete)
	mux.HandleFunc("GET /api/v1/library/history", s.handleHistoryGet)
	mux.HandleFunc("POST /api/v1/watchlist", s.handleSeerrWatchlistAdd)
	mux.HandleFunc("DELETE /api/v1/watchlist/{tmdbId}", s.handleSeerrWatchlistRemove)
	mux.HandleFunc("GET /api/v1/discover/watchlist", s.handleSeerrWatchlistPage)
}

// libraryPeople is the caller and the partner of one request.
type libraryPeople struct {
	me, partner auth.User
}

// people resolves both accounts or answers the request itself. The session
// gate guarantees a caller; its absence is a wiring fault, not a client one.
func (s *Service) people(w http.ResponseWriter, r *http.Request) (libraryPeople, bool) {
	me, ok := auth.UserFromContext(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return libraryPeople{}, false
	}
	partner, err := s.partners.PartnerOf(me.ID)
	if err != nil {
		s.failed(w, r, err)
		return libraryPeople{}, false
	}
	return libraryPeople{me: me, partner: partner}, true
}

// inWriteTx runs fn in one transaction on the writer, which begins
// IMMEDIATE, so a read inside fn sees every committed write and no other
// writer can interleave with it.
func (s *Service) inWriteTx(ctx context.Context, fn func(tx *sql.Tx) error) (err error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// libraryQueryer is what a read needs, inside a transaction or on the read
// pool.
type libraryQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// libraryTimestampLayout is how a JavaScript Date serialises, which is what
// Seerr's own server sends and what the auth routes answer.
const libraryTimestampLayout = "2006-01-02T15:04:05.000Z07:00"

func libraryTimestamp(unix int64) string {
	return time.Unix(unix, 0).UTC().Format(libraryTimestampLayout)
}

func libraryOptionalTimestamp(unix sql.NullInt64) *string {
	if !unix.Valid {
		return nil
	}
	formatted := libraryTimestamp(unix.Int64)
	return &formatted
}

// libraryWriteJSON answers status with body, with the headers respond.JSON
// sets; respond.JSON itself only answers 200.
func libraryWriteJSON(w http.ResponseWriter, status int, body any) {
	if status == http.StatusOK {
		respond.JSON(w, body)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func libraryNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func libraryBadRequest(w http.ResponseWriter, problem string) {
	respond.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), problem)
}

func libraryForbidden(w http.ResponseWriter, problem string) {
	respond.Error(w, http.StatusForbidden, http.StatusText(http.StatusForbidden), problem)
}

func libraryNotFound(w http.ResponseWriter, problem string) {
	respond.Error(w, http.StatusNotFound, http.StatusText(http.StatusNotFound), problem)
}

// failed answers 500. The cause goes to the log, never to the client.
func (s *Service) failed(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.ErrorContext(r.Context(), "library request failed", "path", r.URL.Path, "error", err)
	respond.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}
