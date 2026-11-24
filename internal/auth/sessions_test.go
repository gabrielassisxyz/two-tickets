package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func sessionRowCount(t *testing.T, svc *Service) int {
	t.Helper()
	var n int
	if err := svc.db.Read.QueryRow("SELECT count(*) FROM sessions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The cookie carries 32 random bytes and the table holds only their SHA-256:
// nothing stored is a working token.
func TestSessionTokenIsRandomAndOnlyItsHashIsStored(t *testing.T) {
	svc, db := newTestService(t, newAuthTestClock())
	first := mustLogin(t, svc, "alice", alicePassword)
	second := mustLogin(t, svc, "alice", alicePassword)
	if first.Token == second.Token {
		t.Fatal("two logins produced the same token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(first.Token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %q is not 32 bytes of unpadded base64url (err %v)", first.Token, err)
	}
	if bytes.Count(raw, []byte{0}) > 8 {
		t.Fatalf("token bytes look zeroed: %x", raw)
	}

	rows, err := db.Read.Query("SELECT token_hash, user_id, created_at, last_seen_at, expires_at FROM sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	want := sha256.Sum256(raw)
	found := false
	for rows.Next() {
		var stored []byte
		var userID, created, lastSeen, expires int64
		if err := rows.Scan(&stored, &userID, &created, &lastSeen, &expires); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(stored, raw) || bytes.Contains(stored, []byte(first.Token)) {
			t.Fatal("the token itself is stored")
		}
		if bytes.Equal(stored, want[:]) {
			found = true
		}
	}
	if !found {
		t.Fatal("no row holds the SHA-256 of the token")
	}
}

func TestMalformedOrUnknownTokensAreRefused(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	session := mustLogin(t, svc, "alice", alicePassword)
	raw, _ := base64.RawURLEncoding.DecodeString(session.Token)
	stdEncoded := base64.StdEncoding.EncodeToString(raw)
	flipped := append([]byte(nil), raw...)
	flipped[0] ^= 1
	for name, token := range map[string]string{
		"empty":                  "",
		"garbage":                "not a token",
		"short":                  base64.RawURLEncoding.EncodeToString(raw[:31]),
		"long":                   base64.RawURLEncoding.EncodeToString(append(raw, 0)),
		"padded standard base64": stdEncoded,
		"one bit off":            base64.RawURLEncoding.EncodeToString(flipped),
		"the stored hash":        base64.RawURLEncoding.EncodeToString(func() []byte { h := sha256.Sum256(raw); return h[:] }()),
	} {
		if _, _, err := svc.Authenticate(context.Background(), token); !errors.Is(err, ErrNoSession) {
			t.Errorf("%s: error = %v, want ErrNoSession", name, err)
		}
	}
	if _, _, err := svc.Authenticate(context.Background(), session.Token); err != nil {
		t.Fatalf("the real token was refused: %v", err)
	}
}

// Security: a login never reuses the session the browser already had.
func TestLoginReplacesTheSessionTheRequestCarried(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	before := mustLogin(t, svc, "alice", alicePassword)
	after, err := svc.Login(context.Background(), LoginAttempt{
		Identifier: "alice", Password: alicePassword, Client: authTestClientA, PreviousToken: before.Token,
	})
	if err != nil {
		t.Fatal(err)
	}
	if after.Token == before.Token {
		t.Fatal("the token is the same before and after login")
	}
	if _, _, err := svc.Authenticate(context.Background(), before.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("the pre-login token still works: %v", err)
	}
	if _, _, err := svc.Authenticate(context.Background(), after.Token); err != nil {
		t.Fatalf("the new token does not work: %v", err)
	}
}

// CRITERION 6: extended at most once per 24 hours.
func TestSessionIsExtendedAtMostOncePer24Hours(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	session := mustLogin(t, svc, "alice", alicePassword)
	wantInitial := authTestEpoch.Add(SessionLifetime)
	if !session.ExpiresAt.Equal(wantInitial) {
		t.Fatalf("expires_at at login = %s, want %s", session.ExpiresAt, wantInitial)
	}
	for _, step := range []time.Duration{10 * time.Minute, 20 * time.Minute, 29 * time.Minute} {
		clock.Advance(step)
		got, renewed, err := svc.Authenticate(context.Background(), session.Token)
		if err != nil {
			t.Fatal(err)
		}
		if renewed || !got.ExpiresAt.Equal(wantInitial) || storedExpiry(t, svc, session.Token) != wantInitial.Unix() {
			t.Fatalf("at %s: renewed=%v expires=%s, want unchanged %s", clock.Now().Sub(authTestEpoch), renewed, got.ExpiresAt, wantInitial)
		}
	}
	clock.Advance(25*time.Hour - time.Hour + time.Minute)
	got, renewed, err := svc.Authenticate(context.Background(), session.Token)
	if err != nil {
		t.Fatal(err)
	}
	wantExtended := clock.Now().Add(SessionLifetime)
	if !renewed || !got.ExpiresAt.Equal(wantExtended) || storedExpiry(t, svc, session.Token) != wantExtended.Unix() {
		t.Fatalf("after 25h: renewed=%v expires=%s, want %s", renewed, got.ExpiresAt, wantExtended)
	}
	// The next request inside the new 24 hours does not extend again.
	clock.Advance(time.Hour)
	if _, renewed, _ := svc.Authenticate(context.Background(), session.Token); renewed {
		t.Fatal("extended twice within 24 hours")
	}
}

func storedExpiry(t *testing.T, svc *Service, token string) int64 {
	t.Helper()
	hash, _ := sessionTokenHash(token)
	var expires int64
	if err := svc.db.Read.QueryRow("SELECT expires_at FROM sessions WHERE token_hash = ?", hash[:]).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	return expires
}

func TestExpiredSessionIsRefusedThenPruned(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	old := mustLogin(t, svc, "alice", alicePassword)
	clock.Advance(SessionLifetime - time.Hour)
	fresh := mustLogin(t, svc, "bob", bobPassword)
	clock.Advance(time.Hour)
	if _, _, err := svc.Authenticate(context.Background(), old.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("a session at its expiry still authenticates: %v", err)
	}
	pruned, err := svc.PruneExpiredSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 || sessionRowCount(t, svc) != 1 {
		t.Fatalf("pruned %d, %d left; want 1 pruned and 1 left", pruned, sessionRowCount(t, svc))
	}
	if _, _, err := svc.Authenticate(context.Background(), fresh.Token); err != nil {
		t.Fatalf("the live session was pruned: %v", err)
	}
}

func TestSessionPruningRunsUntilCancelled(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	mustLogin(t, svc, "alice", alicePassword)
	clock.Advance(SessionLifetime)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunSessionPruning(ctx, time.Hour, discardLogger())
		close(done)
	}()
	deadline := time.Now().Add(10 * time.Second)
	for sessionRowCount(t, svc) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the pruner never removed the expired session")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the pruner did not stop when its context ended")
	}
}

// CRITERION 7, service side: logout revokes, and a replayed token is refused.
func TestLogoutRevokesOnlyThatSession(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	alice := mustLogin(t, svc, "alice", alicePassword)
	bob := mustLogin(t, svc, "bob", bobPassword)
	if err := svc.Logout(context.Background(), alice.Token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Authenticate(context.Background(), alice.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("the logged-out token still works: %v", err)
	}
	// Partner isolation: one account's logout cannot end the other's session.
	if _, _, err := svc.Authenticate(context.Background(), bob.Token); err != nil {
		t.Fatalf("the partner's session ended: %v", err)
	}
	for _, token := range []string{alice.Token, "", "garbage"} {
		if err := svc.Logout(context.Background(), token); err != nil {
			t.Errorf("logout of %q: %v", token, err)
		}
	}
}

func TestSessionOfAnAccountNoLongerConfiguredIsRefused(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	session := mustLogin(t, svc, "alice", alicePassword)
	svc.accounts[0].user.ID += 1000
	if _, _, err := svc.Authenticate(context.Background(), session.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("a session of an unconfigured user id authenticated: %v", err)
	}
}
