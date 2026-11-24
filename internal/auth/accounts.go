package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// User is an account as the rest of the service sees it.
type User struct {
	ID          int64
	ConfigKey   string
	Username    string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// knownAccount is a configured account with the hash login verifies against.
type knownAccount struct {
	user User
	hash PasswordHash
}

type storedAccountRow struct {
	username, email, displayName, passwordHash string
	updatedAt                                  int64
}

// syncAccounts writes the configured accounts into users in one transaction,
// keyed by config_key: login fields and hashes follow the configuration, the
// numeric id never changes, so every row that references it stays attached.
func syncAccounts(ctx context.Context, db *store.Database, configured [2]config.Account, now time.Time) (_ [2]knownAccount, err error) {
	var known [2]knownAccount
	for i, c := range configured {
		hash, err := ParsePasswordHash(c.PasswordHash.Reveal())
		if err != nil {
			return known, fmt.Errorf("account %q password hash: %w", c.ConfigKey, err)
		}
		known[i].hash = hash
	}

	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		return known, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	existing, err := readStoredAccounts(ctx, tx, configured)
	if err != nil {
		return known, err
	}
	// Parks both rows' identifiers first, so two accounts can trade a username
	// or an email in one restart without tripping UNIQUE halfway through. No
	// configured value can collide with the placeholder: usernames hold no ":"
	// and emails need an "@".
	if _, err = tx.ExecContext(ctx, `UPDATE users SET username = 'pending:' || config_key, email = 'pending:' || config_key
		WHERE config_key IN (?, ?)`, configured[0].ConfigKey, configured[1].ConfigKey); err != nil {
		return known, err
	}
	for i, c := range configured {
		user, err := upsertAccount(ctx, tx, c, existing[c.ConfigKey], now)
		if err != nil {
			return known, fmt.Errorf("account %q: %w", c.ConfigKey, err)
		}
		known[i].user = user
	}
	return known, tx.Commit()
}

func readStoredAccounts(ctx context.Context, tx *sql.Tx, configured [2]config.Account) (map[string]*storedAccountRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT config_key, username, email, display_name, password_hash, updated_at
		FROM users WHERE config_key IN (?, ?)`, configured[0].ConfigKey, configured[1].ConfigKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	existing := map[string]*storedAccountRow{}
	for rows.Next() {
		var key string
		var row storedAccountRow
		if err := rows.Scan(&key, &row.username, &row.email, &row.displayName, &row.passwordHash, &row.updatedAt); err != nil {
			return nil, err
		}
		existing[key] = &row
	}
	return existing, rows.Err()
}

func upsertAccount(ctx context.Context, tx *sql.Tx, c config.Account, prior *storedAccountRow, now time.Time) (User, error) {
	updatedAt := now.Unix()
	if prior != nil && prior.username == c.Username && prior.email == c.Email &&
		prior.displayName == c.DisplayName && prior.passwordHash == c.PasswordHash.Reveal() {
		updatedAt = prior.updatedAt
	}
	user := User{ConfigKey: c.ConfigKey, Username: c.Username, Email: c.Email, DisplayName: c.DisplayName}
	var createdAt int64
	err := tx.QueryRowContext(ctx, `INSERT INTO users
		(config_key, username, email, display_name, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (config_key) DO UPDATE SET
			username = excluded.username, email = excluded.email, display_name = excluded.display_name,
			password_hash = excluded.password_hash, updated_at = ?
		RETURNING id, created_at`,
		c.ConfigKey, c.Username, c.Email, c.DisplayName, c.PasswordHash.Reveal(), now.Unix(), updatedAt, updatedAt,
	).Scan(&user.ID, &createdAt)
	if err != nil {
		return User{}, err
	}
	user.CreatedAt = time.Unix(createdAt, 0).UTC()
	user.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return user, nil
}
