-- The two fixed accounts and their login sessions.
--
-- users.id is what every other table references, so startup upserts the
-- configured accounts by config_key and never rewrites id. Timestamps are
-- Unix seconds.
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    config_key    TEXT    NOT NULL UNIQUE,
    username      TEXT    NOT NULL UNIQUE,
    email         TEXT    NOT NULL UNIQUE,
    display_name  TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;

-- token_hash is the SHA-256 of the cookie's token: a copy of this table is
-- not a set of working logins.
CREATE TABLE sessions (
    token_hash   BLOB    PRIMARY KEY CHECK (length(token_hash) = 32),
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
) STRICT, WITHOUT ROWID;

CREATE INDEX sessions_by_expiry ON sessions (expires_at);
CREATE INDEX sessions_by_user ON sessions (user_id);
