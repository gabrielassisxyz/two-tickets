-- The library: what each person watched, how they rated it, and their lists.
--
-- Every rule the data must obey is a constraint here rather than a check in
-- Go, so no code path, script or later bug can store a row the library does
-- not allow. Constraints carry names because SQLite reports a CHECK failure by
-- name. Timestamps are Unix seconds.
--
-- A TMDB id is never stored without its media type: movie and TV ids are
-- separate sequences that collide, so every key holding tmdb_id also holds
-- media_type, and media_type is NOT NULL wherever tmdb_id appears (a NULL
-- would make every CHECK below that compares it evaluate to NULL, which
-- SQLite treats as a pass).
--
-- References to users carry no ON DELETE action: removing an account that
-- still has library rows fails instead of silently discarding them.

-- scope says what was watched: a movie, a whole show, or one season of a show,
-- in which case season_number names it (0 is TMDB's specials season). source
-- is 'manual' until a watched-sync source exists.
CREATE TABLE watches (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    media_type    TEXT    NOT NULL,
    tmdb_id       INTEGER NOT NULL,
    season_number INTEGER,
    scope         TEXT    NOT NULL,
    watched_at    INTEGER NOT NULL,
    source        TEXT    NOT NULL DEFAULT 'manual',
    created_at    INTEGER NOT NULL,
    CONSTRAINT watches_media_type_known CHECK (media_type IN ('movie', 'tv')),
    CONSTRAINT watches_tmdb_id_positive CHECK (tmdb_id > 0),
    CONSTRAINT watches_scope_known CHECK (scope IN ('movie', 'show', 'season')),
    CONSTRAINT watches_scope_matches_media_type CHECK ((scope = 'movie') = (media_type = 'movie')),
    CONSTRAINT watches_season_only_for_season_scope CHECK ((season_number IS NOT NULL) = (scope = 'season')),
    CONSTRAINT watches_season_number_not_negative CHECK (season_number >= 0),
    CONSTRAINT watches_source_known CHECK (source IN ('manual'))
) STRICT;

-- A UNIQUE index treats NULLs as distinct, so one index over season_number
-- would let a movie or a whole show be marked watched any number of times.
-- The pair splits the rows by whether season_number is set.
CREATE UNIQUE INDEX watches_unique_without_season
    ON watches (user_id, media_type, tmdb_id, scope) WHERE season_number IS NULL;
CREATE UNIQUE INDEX watches_unique_with_season
    ON watches (user_id, media_type, tmdb_id, scope, season_number) WHERE season_number IS NOT NULL;

-- level is a name on the sentiment-7-v1 scale and scale records that, so a
-- future scale can convert old ratings without guessing which scale they used.
-- A row is a level, a review, or both. length() counts characters, not bytes.
CREATE TABLE ratings (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    media_type    TEXT    NOT NULL,
    tmdb_id       INTEGER NOT NULL,
    season_number INTEGER,
    scope         TEXT    NOT NULL,
    level         TEXT,
    scale         TEXT,
    review        TEXT,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    CONSTRAINT ratings_media_type_known CHECK (media_type IN ('movie', 'tv')),
    CONSTRAINT ratings_tmdb_id_positive CHECK (tmdb_id > 0),
    CONSTRAINT ratings_scope_known CHECK (scope IN ('movie', 'show', 'season')),
    CONSTRAINT ratings_scope_matches_media_type CHECK ((scope = 'movie') = (media_type = 'movie')),
    CONSTRAINT ratings_season_only_for_season_scope CHECK ((season_number IS NOT NULL) = (scope = 'season')),
    CONSTRAINT ratings_season_number_not_negative CHECK (season_number >= 0),
    CONSTRAINT ratings_level_known CHECK (level IN ('hated', 'disliked', 'meh', 'fine', 'good', 'liked', 'loved')),
    CONSTRAINT ratings_level_has_scale CHECK (level IS NULL OR scale IS 'sentiment-7-v1'),
    CONSTRAINT ratings_scale_only_with_level CHECK (scale IS NULL OR level IS NOT NULL),
    CONSTRAINT ratings_level_or_review CHECK (level IS NOT NULL OR review IS NOT NULL),
    CONSTRAINT ratings_review_length CHECK (length(review) BETWEEN 1 AND 10000)
) STRICT;

CREATE UNIQUE INDEX ratings_unique_without_season
    ON ratings (user_id, media_type, tmdb_id, scope) WHERE season_number IS NULL;
CREATE UNIQUE INDEX ratings_unique_with_season
    ON ratings (user_id, media_type, tmdb_id, scope, season_number) WHERE season_number IS NOT NULL;

-- A watchlist and a custom list belong to one person; the couple list belongs
-- to both, which is what a NULL owner means. Startup creates each person's
-- watchlist and the couple list, and the two partial indexes keep them single
-- however many times, or however concurrently, that runs.
CREATE TABLE lists (
    id            INTEGER PRIMARY KEY,
    kind          TEXT    NOT NULL,
    owner_user_id INTEGER REFERENCES users (id),
    name          TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    CONSTRAINT lists_kind_known CHECK (kind IN ('watchlist', 'couple', 'custom')),
    CONSTRAINT lists_owner_null_only_for_couple CHECK ((owner_user_id IS NULL) = (kind = 'couple')),
    CONSTRAINT lists_name_not_empty CHECK (length(name) > 0)
) STRICT;

CREATE UNIQUE INDEX lists_one_watchlist_per_owner ON lists (owner_user_id) WHERE kind = 'watchlist';
CREATE UNIQUE INDEX lists_one_couple_list ON lists (kind) WHERE kind = 'couple';

-- A title appears in a list at most once. Deleting a list takes its items
-- with it; an item never outlives the list that held it.
CREATE TABLE list_items (
    list_id          INTEGER NOT NULL REFERENCES lists (id) ON DELETE CASCADE,
    media_type       TEXT    NOT NULL,
    tmdb_id          INTEGER NOT NULL,
    added_by_user_id INTEGER NOT NULL REFERENCES users (id),
    added_at         INTEGER NOT NULL,
    note             TEXT,
    PRIMARY KEY (list_id, media_type, tmdb_id),
    CONSTRAINT list_items_media_type_known CHECK (media_type IN ('movie', 'tv')),
    CONSTRAINT list_items_tmdb_id_positive CHECK (tmdb_id > 0)
) STRICT, WITHOUT ROWID;
