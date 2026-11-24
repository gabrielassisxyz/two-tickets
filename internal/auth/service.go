package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// ErrInvalidCredentials is the one answer to every failed login: unknown
// identifier, wrong password and unreadable request alike.
var ErrInvalidCredentials = errors.New("invalid credentials")

// RateLimitedError refuses a login attempt over one of the limits.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("too many login attempts, retry in %s", e.RetryAfter)
}

// Service is login, sessions and the partner relation over state.db.
type Service struct {
	db       *store.Database
	accounts [2]knownAccount
	// dummyHash is what an unknown identifier is verified against.
	dummyHash PasswordHash
	limiter   *loginRateLimiter
	now       func() time.Time
	// verifyPassword is swapped by tests to count verifications; production
	// always uses verifyArgon2idPassword.
	verifyPassword func(PasswordHash, string) bool
}

// NewService writes the configured accounts into users and returns the
// service. now may be nil, meaning time.Now; tests pass a fake clock.
func NewService(ctx context.Context, db *store.Database, accounts [2]config.Account, now func() time.Time) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	known, err := syncAccounts(ctx, db, accounts, now())
	if err != nil {
		return nil, err
	}
	// The dummy costs what the first account costs. Two accounts with
	// different parameters would each be distinguishable from the dummy
	// anyway; one shared setting is what the documentation asks for.
	dummy, err := newDummyPasswordHash(known[0].hash.Params)
	if err != nil {
		return nil, err
	}
	return &Service{
		db:             db,
		accounts:       known,
		dummyHash:      dummy,
		limiter:        newLoginRateLimiter(),
		now:            now,
		verifyPassword: verifyArgon2idPassword,
	}, nil
}

// LoginAttempt is one submission of the login form.
type LoginAttempt struct {
	// Identifier is the username or the email, as typed.
	Identifier string
	Password   string
	// Client is the address the limits count against: the proxy's view of
	// the browser, never a header the browser chose.
	Client netip.Addr
	// PreviousToken is the session cookie the request already carried, if
	// any. It is revoked on success, so no token outlives the login it
	// preceded.
	PreviousToken string
}

// Login checks the credentials and opens a fresh session. A failure is
// ErrInvalidCredentials or a *RateLimitedError; anything else is internal.
func (s *Service) Login(ctx context.Context, attempt LoginAttempt) (Session, error) {
	now := s.now()
	identifier := config.NormalizeAccountIdentifier(attempt.Identifier)
	if retryAfter, ok := s.limiter.admit(attempt.Client, identifier, now); !ok {
		return Session{}, &RateLimitedError{RetryAfter: retryAfter}
	}
	account, found := s.accountByIdentifier(identifier)
	hash := s.dummyHash
	if found {
		hash = account.hash
	}
	// Verified even when nothing was found, so a miss costs the same time as
	// a wrong password and the response time names no account.
	if !s.verifyPassword(hash, attempt.Password) || !found {
		return Session{}, ErrInvalidCredentials
	}
	s.limiter.forgetIdentifier(attempt.Client, identifier)
	if err := s.deleteSession(ctx, attempt.PreviousToken); err != nil {
		return Session{}, err
	}
	return s.createSession(ctx, account.user, now)
}

// Logout revokes the session the token names. A token that names nothing is
// not an error: the answer to the client is the same either way.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.deleteSession(ctx, token)
}

// PartnerOf returns the other account. It is derived here, never taken from
// a client, so no request can name whose data it acts on.
func (s *Service) PartnerOf(userID int64) (User, error) {
	switch userID {
	case s.accounts[0].user.ID:
		return s.accounts[1].user, nil
	case s.accounts[1].user.ID:
		return s.accounts[0].user, nil
	}
	return User{}, fmt.Errorf("user %d is not a configured account", userID)
}

// Users returns the two configured accounts in configuration order.
func (s *Service) Users() [2]User {
	return [2]User{s.accounts[0].user, s.accounts[1].user}
}

func (s *Service) accountByIdentifier(identifier string) (knownAccount, bool) {
	for _, a := range s.accounts {
		if identifier != "" && (identifier == a.user.Username || identifier == a.user.Email) {
			return a, true
		}
	}
	return knownAccount{}, false
}

// userByID only answers for the configured accounts: a users row left behind
// by an account removed from the configuration logs nobody in.
func (s *Service) userByID(id int64) (User, bool) {
	for _, a := range s.accounts {
		if a.user.ID == id {
			return a.user, true
		}
	}
	return User{}, false
}

// RunSessionPruning prunes expired sessions now and then every interval until
// ctx ends. Failures are logged and retried on the next tick.
func (s *Service) RunSessionPruning(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := s.PruneExpiredSessions(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.WarnContext(ctx, "prune expired sessions", "error", err)
		} else if n > 0 {
			logger.InfoContext(ctx, "pruned expired sessions", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type userContextKey struct{}

// ContextWithUser attaches the authenticated account to a request context.
func ContextWithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// UserFromContext returns the authenticated account. Every route behind the
// session gate has one; ok is false only on the public routes.
func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey{}).(User)
	return user, ok
}
