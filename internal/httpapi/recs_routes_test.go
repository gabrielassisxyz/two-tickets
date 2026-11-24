package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/recs"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

// recsTestTMDB answers cold-start trending and details for any title, so the
// recommendations routes serve without ratings.
type recsTestTMDB struct{}

func (recsTestTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/trending/all/week" {
		_, _ = w.Write([]byte(`{"page":1,"total_pages":1,"total_results":1,"results":[{"id":9001,"media_type":"movie","title":"Trend"}]}`))
		return
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if kind == "tv" {
		_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Show %s"}`, id, id)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Movie %s"}`, id, id)
}

type recsServer struct {
	*authServer
	alice, bob string
}

// startRecsServer is the router as the binary assembles it, with the
// recommender over a TMDB stand-in.
func startRecsServer(t *testing.T) *recsServer {
	t.Helper()
	substitute := httptest.NewServer(recsTestTMDB{})
	t.Cleanup(substitute.Close)
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:     substitute.URL,
		Limiter:     rate.NewLimiter(rate.Inf, 1),
		Sleep:       func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		MaxAttempts: 1,
	})
	s := startAuthServerWith(t, nil, func(deps RouterDeps) http.Handler {
		db := deps.Readiness[0].(*store.Database)
		deps.Recs = recs.NewService(db, client, hydrate.New(client), nil, nil, nil)
		return NewRouter(deps)
	})
	return &recsServer{
		authServer: s,
		alice:      s.mustLogin(t, "alice", routerAlicePassword),
		bob:        s.mustLogin(t, "bob", routerBobPassword),
	}
}

// The recommendations routes sit behind the session gate and the origin
// check: no session answers 401, a foreign origin on a mutation answers
// 403, and a signed-in cold start answers a popular page.
func TestRecsRoutesAreBehindTheSessionGateAndTheOriginCheck(t *testing.T) {
	s := startRecsServer(t)
	if resp := s.do(t, authRequest{method: http.MethodGet, path: "/api/v1/recommendations"}); resp.status != http.StatusUnauthorized {
		t.Errorf("GET without a session = %d, want 401", resp.status)
	}
	if resp := s.do(t, authRequest{method: http.MethodPost, path: "/api/v1/recommendations/exposures", body: `{}`, headers: withOrigin(nil)}); resp.status != http.StatusUnauthorized {
		t.Errorf("POST without a session = %d, want 401", resp.status)
	}
	foreign := map[string]string{"Origin": "https://elsewhere.example", "Content-Type": "application/json"}
	resp := s.do(t, authRequest{method: http.MethodPost, path: "/api/v1/recommendations/exposures", body: `{"runId":1,"ranks":[1]}`, token: s.alice, headers: foreign})
	if resp.status != http.StatusForbidden {
		t.Errorf("POST from a foreign origin = %d, want 403", resp.status)
	}
	resp = s.do(t, authRequest{method: http.MethodGet, path: "/api/v1/recommendations", token: s.alice, headers: withOrigin(nil)})
	if resp.status != http.StatusOK {
		t.Fatalf("cold start = %d %.300s, want 200", resp.status, resp.body)
	}
	assertErrorShape(t, s.do(t, authRequest{method: http.MethodPost, path: "/api/v1/recommendations/exposures", body: `{"runId":9999,"ranks":[1]}`, token: s.alice, headers: withOrigin(nil)}))
}
