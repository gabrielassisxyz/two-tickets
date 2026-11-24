package auth

import (
	"context"
	"net/netip"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// Low entropy on purpose: a random-looking value trips the secret scanner.
const (
	alicePassword = "alicealicealicealice"
	bobPassword   = "bobbobbobbobbobbob"
)

var (
	authTestEpoch     = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	authTestClientA   = netip.MustParseAddr("192.0.2.10")
	authTestClientB   = netip.MustParseAddr("192.0.2.20")
	authTestHashCache sync.Map
)

// authTestClock is a settable clock shared by the service and the test.
type authTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newAuthTestClock() *authTestClock { return &authTestClock{now: authTestEpoch} }

func (c *authTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *authTestClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// hashFor derives each password once per test binary: argon2id at the real
// cost is the slow part of every test here.
func hashFor(t *testing.T, password string) string {
	t.Helper()
	if cached, ok := authTestHashCache.Load(password); ok {
		return cached.(string)
	}
	encoded, err := HashPassword(password, MinimumArgon2idParams)
	if err != nil {
		t.Fatal(err)
	}
	authTestHashCache.Store(password, encoded)
	return encoded
}

func testAccounts(t *testing.T) [2]config.Account {
	t.Helper()
	return [2]config.Account{
		{ConfigKey: "first", Username: "alice", Email: "alice@example.com", DisplayName: "Alice", PasswordHash: config.NewSecret(hashFor(t, alicePassword))},
		{ConfigKey: "second", Username: "bob", Email: "bob@example.com", DisplayName: "Bob", PasswordHash: config.NewSecret(hashFor(t, bobPassword))},
	}
}

// openMigratedState opens state.db at path and runs the real migrations.
func openMigratedState(t *testing.T, path string) *store.Database {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := store.LoadMigrations(store.StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateState(ctx, db, migrations, filepath.Join(filepath.Dir(path), store.RollbackDirName)); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestService(t *testing.T, clock *authTestClock) (*Service, *store.Database) {
	t.Helper()
	db := openMigratedState(t, filepath.Join(t.TempDir(), store.StateFileName))
	t.Cleanup(func() { _ = db.Close() })
	svc, err := NewService(context.Background(), db, testAccounts(t), clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db
}

type recordedVerification struct {
	hash PasswordHash
}

// countVerifications replaces the verifier with one that records every call
// and then runs the real one, so behaviour is unchanged.
func countVerifications(svc *Service) *[]recordedVerification {
	var calls []recordedVerification
	svc.verifyPassword = func(h PasswordHash, password string) bool {
		calls = append(calls, recordedVerification{hash: h})
		return verifyArgon2idPassword(h, password)
	}
	return &calls
}

// refuseEveryPassword makes login fail without paying for argon2id, for the
// tests that only count attempts.
func refuseEveryPassword(svc *Service) {
	svc.verifyPassword = func(PasswordHash, string) bool { return false }
}

func mustLogin(t *testing.T, svc *Service, identifier, password string) Session {
	t.Helper()
	session, err := svc.Login(context.Background(), LoginAttempt{Identifier: identifier, Password: password, Client: authTestClientA})
	if err != nil {
		t.Fatalf("login %q: %v", identifier, err)
	}
	return session
}
