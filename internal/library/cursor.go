package library

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
)

// libraryPageSize is how many items a list or history page holds: the most
// one hydration call fetches.
const libraryPageSize = hydrate.PageLimit

// cursorPage is every cursor-paged answer. NextCursor is null on the last
// page; otherwise it is opaque to the client and carries the full ordering
// key of the page's last row, so rows sharing a timestamp are never skipped
// or repeated.
type cursorPage[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"nextCursor"`
}

// listItemCursor is the key lists order by: added_at DESC, media_type ASC,
// tmdb_id ASC.
type listItemCursor struct {
	AddedAt   int64  `json:"a"`
	MediaType string `json:"m"`
	TMDBID    int    `json:"t"`
}

// historyCursor is the key history orders by: activity_at DESC, media_type,
// tmdb_id, scope, season_number ASC. Season is -1 for a row without one,
// which sorts where SQLite puts NULL in ascending order.
type historyCursor struct {
	ActivityAt int64  `json:"a"`
	MediaType  string `json:"m"`
	TMDBID     int    `json:"t"`
	Scope      string `json:"s"`
	Season     int    `json:"n"`
}

func encodeLibraryCursor(key any) *string {
	raw, err := json.Marshal(key)
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}

// decodeLibraryCursor reads ?cursor into dest and reports whether one was
// given. A cursor this server did not issue answers 400.
func decodeLibraryCursor(w http.ResponseWriter, r *http.Request, dest any) (given, ok bool) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return false, true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(decoded))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(dest)
	}
	if err != nil {
		libraryBadRequest(w, "cursor is not one this server issued")
		return false, false
	}
	return true, true
}

// libraryPageOf trims the one extra row a page query fetches to learn whether
// another page follows, and builds the cursor from the last row kept.
func libraryPageOf[T any](rows []T, keyOf func(T) any) ([]T, *string) {
	if len(rows) <= libraryPageSize {
		return rows, nil
	}
	rows = rows[:libraryPageSize]
	return rows, encodeLibraryCursor(keyOf(rows[len(rows)-1]))
}
