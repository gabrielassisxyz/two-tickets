package auth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"
)

// CRITERION 1: an unknown identifier still pays for one argon2id verification,
// against the dummy hash, and fails exactly like a wrong password.
func TestUnknownIdentifierIsVerifiedAgainstTheDummyHash(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	calls := countVerifications(svc)
	ctx := context.Background()

	_, unknownErr := svc.Login(ctx, LoginAttempt{Identifier: "mallory", Password: alicePassword, Client: authTestClientA})
	if len(*calls) != 1 {
		t.Fatalf("unknown identifier: %d verifications, want 1", len(*calls))
	}
	if (*calls)[0].hash.Params != svc.dummyHash.Params || string((*calls)[0].hash.key) != string(svc.dummyHash.key) {
		t.Fatal("unknown identifier was not verified against the dummy hash")
	}

	_, wrongErr := svc.Login(ctx, LoginAttempt{Identifier: "alice", Password: bobPassword, Client: authTestClientA})
	if len(*calls) != 2 || string((*calls)[1].hash.key) != string(svc.accounts[0].hash.key) {
		t.Fatalf("wrong password: verifications = %d, or not against the account's hash", len(*calls))
	}

	_, emptyErr := svc.Login(ctx, LoginAttempt{Client: authTestClientA})
	if len(*calls) != 3 || string((*calls)[2].hash.key) != string(svc.dummyHash.key) {
		t.Fatalf("empty attempt: verifications = %d, or not against the dummy hash", len(*calls))
	}
	for name, err := range map[string]error{"unknown": unknownErr, "wrong password": wrongErr, "empty": emptyErr} {
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: error = %v, want ErrInvalidCredentials", name, err)
		}
	}
}

// An account's own password, verified against the dummy by mistake, must not
// log anyone in: the found flag is checked after verification.
func TestCorrectPasswordForAnUnknownIdentifierFails(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	svc.verifyPassword = func(PasswordHash, string) bool { return true }
	_, err := svc.Login(context.Background(), LoginAttempt{Identifier: "mallory", Password: "anything", Client: authTestClientA})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("an unknown identifier logged in when verification passed: %v", err)
	}
}

// CRITERION 2, first half.
func TestIdentifierCaseAndSurroundingSpacesDoNotMatter(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	for _, identifier := range []string{"alice", "  ALICE ", "Alice", "alice@example.com", "\tAlice@Example.COM\n"} {
		session := mustLogin(t, svc, identifier, alicePassword)
		if session.User.ConfigKey != "first" {
			t.Errorf("%q logged in %q, want first", identifier, session.User.ConfigKey)
		}
	}
	// The password is not normalised.
	if _, err := svc.Login(context.Background(), LoginAttempt{Identifier: "alice", Password: " " + alicePassword, Client: authTestClientA}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("a password with a leading space was accepted: %v", err)
	}
}

func assertRateLimited(t *testing.T, err error, context string) {
	t.Helper()
	var limited *RateLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("%s: error = %v, want a rate limit", context, err)
	}
	if limited.RetryAfter <= 0 || limited.RetryAfter > LoginRateWindow {
		t.Fatalf("%s: RetryAfter = %s", context, limited.RetryAfter)
	}
}

func attemptLogin(svc *Service, identifier string, client netip.Addr) error {
	_, err := svc.Login(context.Background(), LoginAttempt{Identifier: identifier, Password: "wrong", Client: client})
	return err
}

// CRITERION 3: ten failures per address and identifier.
func TestEleventhFailureForOneAddressAndIdentifierIsRateLimited(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	refuseEveryPassword(svc)
	for i := 1; i <= LoginAttemptsPerIdentifier; i++ {
		if err := attemptLogin(svc, "alice", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v, want invalid credentials", i, err)
		}
		clock.Advance(time.Minute)
	}
	assertRateLimited(t, attemptLogin(svc, "alice", authTestClientA), "11th attempt")
	// The limit is on the normalised identifier, so case does not reset it.
	assertRateLimited(t, attemptLogin(svc, " ALICE ", authTestClientA), "11th attempt in upper case")
	// Another identifier from the same address still has its own budget.
	if err := attemptLogin(svc, "bob", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("another identifier: %v", err)
	}
	// The window slides: 15 minutes after the first attempt, one slot frees.
	clock.Advance(LoginRateWindow - LoginAttemptsPerIdentifier*time.Minute + time.Second)
	if err := attemptLogin(svc, "alice", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("after the window slid: %v", err)
	}
	assertRateLimited(t, attemptLogin(svc, "alice", authTestClientA), "the slot was used again")
}

// CRITERION 3 counts failed logins: a correct password ends the run of failures
// against that identifier, so legitimate logins never lock the account.
func TestSuccessfulLoginClearsTheIdentifiersFailures(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	for i := 1; i < LoginAttemptsPerIdentifier; i++ {
		if err := attemptLogin(svc, "alice", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d: %v, want invalid credentials", i, err)
		}
	}
	mustLogin(t, svc, "alice", alicePassword)
	// Without the reset this is the 11th attempt inside the window.
	if err := attemptLogin(svc, "alice", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("first failure after a successful login: %v, want invalid credentials, not a rate limit", err)
	}
}

// The per-address ceiling counts attempts, successes included: a login that
// succeeds clears its own identifier's failures and never the address's budget,
// which is what bounds guessing across many identifiers.
func TestSuccessfulLoginKeepsTheAddressBudget(t *testing.T) {
	clock := newAuthTestClock()
	svc, _ := newTestService(t, clock)
	for i := 1; i < LoginAttemptsPerAddress; i++ {
		if err := attemptLogin(svc, fmt.Sprintf("guess-%d", i), authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v, want invalid credentials", i, err)
		}
	}
	mustLogin(t, svc, "alice", alicePassword)
	assertRateLimited(t, attemptLogin(svc, "guess-31", authTestClientA), "31st attempt after a successful 30th")
}

// CRITERION 3: thirty attempts per address, whatever the identifiers.
func TestThirtyFirstAttemptFromOneAddressIsRateLimited(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	refuseEveryPassword(svc)
	for i := 1; i <= LoginAttemptsPerAddress; i++ {
		if err := attemptLogin(svc, fmt.Sprintf("user%d", i), authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v, want invalid credentials", i, err)
		}
	}
	assertRateLimited(t, attemptLogin(svc, "never-tried", authTestClientA), "31st attempt")
	// The ceiling is per address.
	if err := attemptLogin(svc, "never-tried", authTestClientB); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("another address: %v", err)
	}
}

// CRITERION 3: addresses do not share counters.
func TestDifferentAddressesDoNotShareACounter(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	refuseEveryPassword(svc)
	for i := 1; i <= LoginAttemptsPerIdentifier; i++ {
		if err := attemptLogin(svc, "alice", authTestClientA); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	assertRateLimited(t, attemptLogin(svc, "alice", authTestClientA), "11th from A")
	for i := 1; i <= LoginAttemptsPerIdentifier; i++ {
		if err := attemptLogin(svc, "alice", authTestClientB); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d from B: %v", i, err)
		}
	}
}

// A refused attempt is not verified: the limit also caps argon2id work.
func TestRateLimitedAttemptIsNotVerified(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	verified := 0
	svc.verifyPassword = func(PasswordHash, string) bool { verified++; return false }
	for range LoginAttemptsPerIdentifier + 5 {
		_ = attemptLogin(svc, "alice", authTestClientA)
	}
	if verified != LoginAttemptsPerIdentifier {
		t.Fatalf("%d verifications, want %d", verified, LoginAttemptsPerIdentifier)
	}
}

func TestRateLimiterForgetsIdleKeys(t *testing.T) {
	limiter := newLoginRateLimiter()
	for i := range 50 {
		limiter.admit(authTestClientA, fmt.Sprintf("user%d", i), authTestEpoch)
	}
	limiter.admit(authTestClientB, "x", authTestEpoch.Add(LoginRateWindow+time.Second))
	if len(limiter.byPair) != 1 || len(limiter.byAddress) != 1 {
		t.Fatalf("after a window: %d pairs, %d addresses, want 1 and 1", len(limiter.byPair), len(limiter.byAddress))
	}
}

func TestPartnerIsTheOtherAccount(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	users := svc.Users()
	for i, user := range users {
		partner, err := svc.PartnerOf(user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if partner != users[1-i] {
			t.Errorf("PartnerOf(%d) = %+v, want %+v", user.ID, partner, users[1-i])
		}
	}
	if _, err := svc.PartnerOf(users[0].ID + users[1].ID + 100); err == nil {
		t.Fatal("PartnerOf an unknown id answered a user")
	}
}
