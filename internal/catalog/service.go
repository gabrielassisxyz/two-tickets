// Package catalog serves the TMDB-backed catalog routes: Discover and its
// sliders, title pages, search, and the filters and lookups. It answers
// through internal/respond and never imports httpapi, which imports this
// package to register the routes.
package catalog

import (
	"net/http"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// Service is what every catalog route shares: the process-wide TMDB client
// and the clock.
type Service struct {
	tmdb *tmdb.Client
	now  func() time.Time
	// watchlist is set at wiring time by UseWatchlist; nil means nothing is
	// on a watchlist.
	watchlist WatchlistMembership
}

// NewService builds the catalog over client. A nil now means time.Now.
func NewService(client *tmdb.Client, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{tmdb: client, now: now}
}

// Register adds the catalog routes to mux. Each route family adds exactly one
// line here, a call to the register<Family> function in that family's own
// file: s.registerDiscover(mux), s.registerTitles(mux), s.registerSearch(mux),
// s.registerLookups(mux).
//
// The families register through watchlistMarkingMux, so every result page
// they answer gains mediaInfo.watchlists in one place.
func (s *Service) Register(mux *http.ServeMux) {
	marking := watchlistMarkingMux{mux: mux, service: s}
	s.registerDiscover(marking)
	s.registerTitles(marking)
	s.registerSearch(marking)
	s.registerLookups(marking)
}
