package library

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/hydrate"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

var libraryTestEpoch = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// libraryTestClock is a settable clock shared by the service and the test.
type libraryTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *libraryTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *libraryTestClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// libraryTestPartners is the two-account partner relation auth derives.
type libraryTestPartners struct{ alice, bob auth.User }

func (p libraryTestPartners) PartnerOf(id int64) (auth.User, error) {
	switch id {
	case p.alice.ID:
		return p.bob, nil
	case p.bob.ID:
		return p.alice, nil
	}
	return auth.User{}, fmt.Errorf("user %d is not configured", id)
}

// libraryTestTMDB answers every movie and series details request, counting
// them, and fails the paths in failing.
type libraryTestTMDB struct {
	calls   atomic.Int32
	failing map[string]bool
}

func (f *libraryTestTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	if f.failing[r.URL.Path] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	kind, id, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	w.Header().Set("Content-Type", "application/json")
	if kind == "tv" {
		_, _ = fmt.Fprintf(w, `{"id":%s,"name":"Show %s","poster_path":"/tv%s.jpg"}`, id, id, id)
		return
	}
	_, _ = fmt.Fprintf(w, `{"id":%s,"title":"Movie %s","poster_path":"/m%s.jpg"}`, id, id, id)
}

// libraryKit is the library over a migrated temporary state.db with both
// accounts and their bootstrapped lists, and a fake TMDB behind hydration.
type libraryKit struct {
	db         *store.Database
	service    *Service
	clock      *libraryTestClock
	tmdb       *libraryTestTMDB
	alice, bob auth.User
	mux        *http.ServeMux
}

func newLibraryKit(t *testing.T) *libraryKit {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), store.StateFileName)
	db, err := store.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := store.LoadMigrations(store.StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateState(ctx, db, migrations, filepath.Join(filepath.Dir(path), store.RollbackDirName)); err != nil {
		t.Fatal(err)
	}
	alice := insertLibraryTestUser(t, db, "first", "alice", "Alice")
	bob := insertLibraryTestUser(t, db, "second", "bob", "Bob")
	if err := store.BootstrapLibraryLists(ctx, db, libraryTestEpoch); err != nil {
		t.Fatal(err)
	}
	fake := &libraryTestTMDB{failing: map[string]bool{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client := tmdb.NewClient(tmdb.Options{
		BaseURL:     server.URL,
		Limiter:     rate.NewLimiter(rate.Inf, 1),
		Sleep:       func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		MaxAttempts: 1,
	})
	clock := &libraryTestClock{now: libraryTestEpoch}
	service := NewService(db, libraryTestPartners{alice: alice, bob: bob}, hydrate.New(client), nil, clock.Now)
	mux := http.NewServeMux()
	service.Register(mux)
	return &libraryKit{db: db, service: service, clock: clock, tmdb: fake, alice: alice, bob: bob, mux: mux}
}

func insertLibraryTestUser(t *testing.T, db *store.Database, key, username, name string) auth.User {
	t.Helper()
	user := auth.User{ConfigKey: key, Username: username, Email: username + "@example.com", DisplayName: name}
	err := db.Write.QueryRowContext(context.Background(), `INSERT INTO users
		(config_key, username, email, display_name, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'unused', ?, ?) RETURNING id`,
		key, username, user.Email, name, libraryTestEpoch.Unix(), libraryTestEpoch.Unix()).Scan(&user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

type libraryResponse struct {
	status int
	body   []byte
}

// serve sends one request as user. The session gate and the origin check
// belong to httpapi and are exercised there; here the caller is placed in
// the context exactly as the gate places it.
func (k *libraryKit) serve(t *testing.T, user auth.User, method, target, body string) libraryResponse {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request = request.WithContext(auth.ContextWithUser(request.Context(), user))
	recorder := httptest.NewRecorder()
	k.mux.ServeHTTP(recorder, request)
	return libraryResponse{status: recorder.Code, body: recorder.Body.Bytes()}
}

func (k *libraryKit) mustServe(t *testing.T, user auth.User, method, target, body string, want int) []byte {
	t.Helper()
	resp := k.serve(t, user, method, target, body)
	if resp.status != want {
		t.Fatalf("%s %s %s = %d %s, want %d", method, target, body, resp.status, resp.body, want)
	}
	return resp.body
}

func decodeLibraryJSON[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %.300s: %v", raw, err)
	}
	return value
}

// The answers' card types hold an interface, so tests decode into these.
type testCard struct {
	MediaType string          `json:"mediaType"`
	TMDBID    int             `json:"tmdbId"`
	Available bool            `json:"available"`
	Result    json.RawMessage `json:"result"`
}

type testListItem struct {
	testCard
	AddedAt   string  `json:"addedAt"`
	AddedByMe bool    `json:"addedByMe"`
	Note      *string `json:"note"`
}

type testHistoryRow struct {
	testCard
	Scope        string  `json:"scope"`
	SeasonNumber *int    `json:"seasonNumber"`
	ActivityAt   string  `json:"activityAt"`
	Watched      bool    `json:"watched"`
	WatchedAt    *string `json:"watchedAt"`
	Rating       *rating `json:"rating"`
}

// listIDOf returns the id of the first list of kind that owner holds; owner
// 0 means the couple list.
func (k *libraryKit) listIDOf(t *testing.T, kind string, owner int64) int64 {
	t.Helper()
	var id int64
	err := k.db.Read.QueryRowContext(context.Background(),
		`SELECT id FROM lists WHERE kind = ? AND owner_user_id IS ? ORDER BY id LIMIT 1`, kind, nullableOwner(owner)).Scan(&id)
	if err != nil {
		t.Fatalf("list %s of %d: %v", kind, owner, err)
	}
	return id
}

func nullableOwner(owner int64) any {
	if owner == 0 {
		return nil
	}
	return owner
}

func (k *libraryKit) countRows(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	if err := k.db.Read.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
