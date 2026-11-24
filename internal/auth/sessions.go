package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

const (
	// SessionLifetime is how long a session lives past its last extension:
	// 180 days, so a phone stays logged in for months.
	SessionLifetime = 180 * 24 * time.Hour
	// SessionRenewalInterval is the least time between two extensions. The
	// frontend polls auth/me every 30 seconds; extending on each poll would
	// turn every read into a write.
	SessionRenewalInterval = 24 * time.Hour

	sessionTokenBytes = 32
)

// ErrNoSession means the request carries no live session: no token, a token
// of the wrong shape, an unknown or expired one, or one whose account is no
// longer configured.
var ErrNoSession = errors.New("no valid session")

// Session is a live login. Token is the cookie value; only its SHA-256 is
// stored, so the database alone cannot be used to log in.
type Session struct {
	Token     string
	User      User
	ExpiresAt time.Time
}

func newSessionToken() (string, [sha256.Size]byte, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", [sha256.Size]byte{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw), sha256.Sum256(raw), nil
}

// sessionTokenHash decodes a cookie value and hashes it. Anything that is not
// exactly the token shape is refused before the database is asked.
func sessionTokenHash(token string) ([sha256.Size]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != sessionTokenBytes {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256(raw), true
}

func (s *Service) createSession(ctx context.Context, user User, now time.Time) (Session, error) {
	token, hash, err := newSessionToken()
	if err != nil {
		return Session{}, err
	}
	expires := now.Add(SessionLifetime)
	if _, err := s.db.Write.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, hash[:], user.ID, now.Unix(), now.Unix(), expires.Unix()); err != nil {
		return Session{}, err
	}
	return Session{Token: token, User: user, ExpiresAt: time.Unix(expires.Unix(), 0).UTC()}, nil
}

// deleteSession removes the session the token names, if there is one.
func (s *Service) deleteSession(ctx context.Context, token string) error {
	hash, ok := sessionTokenHash(token)
	if !ok {
		return nil
	}
	_, err := s.db.Write.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash[:])
	return err
}

// Authenticate resolves a cookie value to its session. renewed reports that
// the session was extended just now and the cookie must be sent again.
func (s *Service) Authenticate(ctx context.Context, token string) (_ Session, renewed bool, _ error) {
	hash, ok := sessionTokenHash(token)
	if !ok {
		return Session{}, false, ErrNoSession
	}
	now := s.now()
	var userID, lastSeen, expires int64
	err := s.db.Read.QueryRowContext(ctx, `SELECT user_id, last_seen_at, expires_at FROM sessions
		WHERE token_hash = ? AND expires_at > ?`, hash[:], now.Unix()).Scan(&userID, &lastSeen, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, ErrNoSession
	}
	if err != nil {
		return Session{}, false, err
	}
	user, ok := s.userByID(userID)
	if !ok {
		return Session{}, false, ErrNoSession
	}
	session := Session{Token: token, User: user, ExpiresAt: time.Unix(expires, 0).UTC()}
	if now.Sub(time.Unix(lastSeen, 0)) < SessionRenewalInterval {
		return session, false, nil
	}
	return s.extendSession(ctx, session, hash, now)
}

func (s *Service) extendSession(ctx context.Context, session Session, hash [sha256.Size]byte, now time.Time) (Session, bool, error) {
	expires := now.Add(SessionLifetime).Unix()
	result, err := s.db.Write.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE token_hash = ?`,
		now.Unix(), expires, hash[:])
	if err != nil {
		return Session{}, false, err
	}
	// A logout that landed between the read and this write wins.
	if n, err := result.RowsAffected(); err != nil || n == 0 {
		return Session{}, false, errors.Join(ErrNoSession, err)
	}
	session.ExpiresAt = time.Unix(expires, 0).UTC()
	return session, true, nil
}

// PruneExpiredSessions deletes every session past its expiry and reports how
// many went. Expired sessions are already refused; this only reclaims space.
func (s *Service) PruneExpiredSessions(ctx context.Context) (int64, error) {
	result, err := s.db.Write.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, s.now().Unix())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
