package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// restartableState is a state.db file that a test can close and reopen, the
// way a process restart does.
type restartableState struct {
	t    *testing.T
	path string
	db   *store.Database
}

func newRestartableState(t *testing.T) *restartableState {
	s := &restartableState{t: t, path: filepath.Join(t.TempDir(), store.StateFileName)}
	t.Cleanup(func() {
		if s.db != nil {
			_ = s.db.Close()
		}
	})
	return s
}

// start closes the previous process's database, if any, and starts a new
// service over the same file with accounts.
func (s *restartableState) start(accounts [2]config.Account, clock *authTestClock) (*Service, error) {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.t.Fatal(err)
		}
	}
	s.db = openMigratedState(s.t, s.path)
	return NewService(context.Background(), s.db, accounts, clock.Now)
}

// CRITERION 9: each field changes across a restart with the id and the
// sessions intact.
func TestAccountChangesTakeEffectAfterRestartWithTheSameID(t *testing.T) {
	cases := map[string]struct {
		change         func(*config.Account)
		loginAs        string
		loginPassword  string
		refusedAs      string
		refusedWithOld string
	}{
		"password": {
			change:         func(a *config.Account) { a.PasswordHash = config.NewSecret(hashFor(t, "newnewnewnewnewnew")) },
			loginAs:        "alice",
			loginPassword:  "newnewnewnewnewnew",
			refusedAs:      "alice",
			refusedWithOld: alicePassword,
		},
		"username": {
			change:         func(a *config.Account) { a.Username = "alicia" },
			loginAs:        "alicia",
			loginPassword:  alicePassword,
			refusedAs:      "alice",
			refusedWithOld: alicePassword,
		},
		"email": {
			change:         func(a *config.Account) { a.Email = "alicia@example.org" },
			loginAs:        "alicia@example.org",
			loginPassword:  alicePassword,
			refusedAs:      "alice@example.com",
			refusedWithOld: alicePassword,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			clock := newAuthTestClock()
			state := newRestartableState(t)
			accounts := testAccounts(t)
			before, err := state.start(accounts, clock)
			if err != nil {
				t.Fatal(err)
			}
			oldSession := mustLogin(t, before, "alice", alicePassword)
			idsBefore := before.Users()

			tc.change(&accounts[0])
			after, err := state.start(accounts, clock)
			if err != nil {
				t.Fatal(err)
			}
			idsAfter := after.Users()
			if idsAfter[0].ID != idsBefore[0].ID || idsAfter[1].ID != idsBefore[1].ID {
				t.Fatalf("ids changed across the restart: %d,%d -> %d,%d", idsBefore[0].ID, idsBefore[1].ID, idsAfter[0].ID, idsAfter[1].ID)
			}
			if _, err := after.Login(context.Background(), LoginAttempt{Identifier: tc.refusedAs, Password: tc.refusedWithOld, Client: authTestClientA}); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("the old %s still logs in: %v", name, err)
			}
			session := mustLogin(t, after, tc.loginAs, tc.loginPassword)
			if session.User.ID != idsBefore[0].ID {
				t.Fatalf("the new %s logs in user %d, want %d", name, session.User.ID, idsBefore[0].ID)
			}
			old, _, err := after.Authenticate(context.Background(), oldSession.Token)
			if err != nil {
				t.Fatalf("the session from before the restart was dropped: %v", err)
			}
			if old.User.ID != idsBefore[0].ID {
				t.Fatalf("the old session now belongs to user %d", old.User.ID)
			}
			assertStoredAccount(t, state.db, accounts[0])
		})
	}
}

func assertStoredAccount(t *testing.T, db *store.Database, want config.Account) {
	t.Helper()
	var username, email, display, hash string
	err := db.Read.QueryRow("SELECT username, email, display_name, password_hash FROM users WHERE config_key = ?", want.ConfigKey).
		Scan(&username, &email, &display, &hash)
	if err != nil {
		t.Fatal(err)
	}
	if username != want.Username || email != want.Email || display != want.DisplayName || hash != want.PasswordHash.Reveal() {
		t.Fatalf("users row = %q %q %q, want %q %q %q (hash equal: %v)", username, email, display,
			want.Username, want.Email, want.DisplayName, hash == want.PasswordHash.Reveal())
	}
}

func TestAccountsCanTradeUsernamesAndEmailsInOneRestart(t *testing.T) {
	clock := newAuthTestClock()
	state := newRestartableState(t)
	accounts := testAccounts(t)
	before, err := state.start(accounts, clock)
	if err != nil {
		t.Fatal(err)
	}
	ids := before.Users()
	accounts[0].Username, accounts[1].Username = accounts[1].Username, accounts[0].Username
	accounts[0].Email, accounts[1].Email = accounts[1].Email, accounts[0].Email
	after, err := state.start(accounts, clock)
	if err != nil {
		t.Fatalf("swapping identifiers failed startup: %v", err)
	}
	session := mustLogin(t, after, "bob", alicePassword)
	if session.User.ID != ids[0].ID {
		t.Fatalf("bob with alice's password is user %d, want %d", session.User.ID, ids[0].ID)
	}
}

func TestUpdatedAtMovesOnlyWhenAnAccountChanges(t *testing.T) {
	clock := newAuthTestClock()
	state := newRestartableState(t)
	accounts := testAccounts(t)
	first, err := state.start(accounts, clock)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(48 * time.Hour)
	accounts[1].DisplayName = "Robert"
	second, err := state.start(accounts, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := second.Users()[0].UpdatedAt, first.Users()[0].UpdatedAt; !got.Equal(want) {
		t.Errorf("unchanged account updated_at moved: %s -> %s", want, got)
	}
	if got := second.Users()[1].UpdatedAt; !got.Equal(clock.Now().Truncate(time.Second)) {
		t.Errorf("changed account updated_at = %s, want %s", got, clock.Now())
	}
	if second.Users()[1].CreatedAt != first.Users()[1].CreatedAt {
		t.Error("created_at moved")
	}
}

func TestStartupRefusesAWeakOrMalformedHash(t *testing.T) {
	weak, err := HashPassword(alicePassword, Argon2idParams{MemoryKiB: 8 * 1024, Passes: 1, Lanes: 1})
	if err != nil {
		t.Fatal(err)
	}
	for name, hash := range map[string]string{"weak": weak, "malformed": "not-a-hash"} {
		t.Run(name, func(t *testing.T) {
			accounts := testAccounts(t)
			accounts[1].PasswordHash = config.NewSecret(hash)
			state := newRestartableState(t)
			_, err := state.start(accounts, newAuthTestClock())
			if err == nil || !strings.Contains(err.Error(), `"second"`) {
				t.Fatalf("startup error = %v, want one naming the account", err)
			}
			if strings.Contains(err.Error(), hash) {
				t.Fatal("the error repeats the hash")
			}
			var n int
			if err := state.db.Read.QueryRow("SELECT count(*) FROM users").Scan(&n); err != nil || n != 0 {
				t.Fatalf("%d users written before the refusal (err %v)", n, err)
			}
		})
	}
}

// The configuration layer refuses a shared username or email; the table's
// UNIQUE constraints are the second line, for a row left by an account that
// is no longer configured.
func TestStartupFailsWhenAStaleRowHoldsAConfiguredUsername(t *testing.T) {
	clock := newAuthTestClock()
	state := newRestartableState(t)
	accounts := testAccounts(t)
	if _, err := state.start(accounts, clock); err != nil {
		t.Fatal(err)
	}
	accounts[1].ConfigKey = "third"
	if _, err := state.start(accounts, clock); err == nil {
		t.Fatal("a new account took the username a stale row holds")
	}
}
