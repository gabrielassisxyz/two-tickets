package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// startDiscoverServer is the router with the real catalog registered, over a
// stand-in TMDB that answers every request with an empty trending page.
func startDiscoverServer(t *testing.T) *authServer {
	t.Helper()
	tmdbStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"results":[],"total_pages":1,"total_results":0}`))
	}))
	t.Cleanup(tmdbStub.Close)
	service := catalog.NewService(tmdb.NewClient(tmdb.Options{BaseURL: tmdbStub.URL}), nil)
	return startAuthServerWith(t, nil, func(deps RouterDeps) http.Handler {
		deps.Catalog = service
		return NewRouter(deps)
	})
}

func TestDiscoverSliderManifestNeedsASession(t *testing.T) {
	s := startDiscoverServer(t)
	const manifest = "/api/v1/settings/discover"
	resp := s.do(t, authRequest{method: http.MethodGet, path: manifest})
	if resp.status != http.StatusUnauthorized {
		t.Fatalf("%s without a session = %d %s, want 401", manifest, resp.status, resp.body)
	}
	assertErrorShape(t, resp)
	token := s.mustLogin(t, "alice", routerAlicePassword)
	if resp := s.do(t, authRequest{method: http.MethodGet, path: manifest, token: token}); resp.status != http.StatusOK {
		t.Fatalf("%s with a session = %d %s, want 200", manifest, resp.status, resp.body)
	}
}

// The login page draws /backdrops before anyone has signed in.
func TestDiscoverBackdropsAnswerWithoutASession(t *testing.T) {
	s := startDiscoverServer(t)
	resp := s.do(t, authRequest{method: http.MethodGet, path: "/api/v1/backdrops"})
	if resp.status != http.StatusOK || string(resp.body) != "[]\n" {
		t.Fatalf("/api/v1/backdrops without a session = %d %s, want 200 []", resp.status, resp.body)
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: "/api/v1/discover/trending?page=1"}); resp.status != http.StatusUnauthorized {
		t.Errorf("/api/v1/discover/trending without a session = %d, want 401", resp.status)
	}
}
