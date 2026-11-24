-- The recommender's memory: runs, their results, and what was shown.
--
-- A run is a single personalised computation for one user. It is written with
-- completed_at NULL and becomes visible only when publishing sets completed_at,
-- after every result is in; readers select the newest completed run, so a run
-- that fails before publishing never replaces the published one. The watermark
-- is the value recs.Watermark computed when the run was built, so a later read
-- can tell whether ratings or watches moved since.
--
-- Every rule the data must obey is a constraint here rather than a check in
-- Go, so no code path, script or later bug can store a row the recommender
-- does not allow. Constraints carry names because SQLite reports a CHECK
-- failure by name. Timestamps are Unix seconds.
--
-- A TMDB id is never stored without its media type: movie and TV ids are
-- separate sequences that collide, so every key holding tmdb_id also holds
-- media_type, and media_type is NOT NULL wherever tmdb_id appears (a NULL
-- would make every CHECK below that compares it evaluate to NULL, which
-- SQLite treats as a pass).
--
-- References to users carry no ON DELETE action: removing an account that
-- still has runs or exposures fails instead of silently discarding them.
-- References to rec_runs carry ON DELETE CASCADE: deleting a run takes its
-- results and exposures with it.
--
-- reasons is a JSON array of seed titles (strings). Computed results alone do
-- not count as shown: only rec_exposures records what entered the viewport,
-- once per (run_id, rank), and in v0.1 exposures never change eligibility.

CREATE TABLE rec_runs (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id),
    algorithm     TEXT    NOT NULL,
    watermark     TEXT    NOT NULL,
    started_at    INTEGER NOT NULL,
    completed_at  INTEGER,
    CONSTRAINT rec_runs_algorithm_not_empty CHECK (length(algorithm) > 0),
    CONSTRAINT rec_runs_watermark_not_empty CHECK (length(watermark) > 0),
    CONSTRAINT rec_runs_completed_after_start CHECK (completed_at IS NULL OR completed_at >= started_at)
) STRICT;

-- The read path selects the newest completed run for a user; the partial
-- index keeps that lookup on completed rows only.
CREATE INDEX rec_runs_newest_completed
    ON rec_runs (user_id, completed_at DESC, id DESC) WHERE completed_at IS NOT NULL;

CREATE TABLE rec_results (
    run_id      INTEGER NOT NULL REFERENCES rec_runs (id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL,
    media_type  TEXT    NOT NULL,
    tmdb_id     INTEGER NOT NULL,
    score       REAL    NOT NULL,
    reasons     TEXT    NOT NULL,
    CONSTRAINT rec_results_rank_positive CHECK (rank > 0),
    CONSTRAINT rec_results_media_type_known CHECK (media_type IN ('movie', 'tv')),
    CONSTRAINT rec_results_tmdb_id_positive CHECK (tmdb_id > 0),
    CONSTRAINT rec_results_reasons_is_json_array CHECK (json_valid(reasons) AND json_type(reasons) = 'array')
) STRICT;

CREATE UNIQUE INDEX rec_results_by_rank ON rec_results (run_id, rank);
CREATE UNIQUE INDEX rec_results_by_title ON rec_results (run_id, media_type, tmdb_id);

-- One row per shown card. The primary key rejects a second exposure of the
-- same rank at the database level; recording uses INSERT ... ON CONFLICT DO
-- NOTHING so a repeat is a no-op rather than an error.
CREATE TABLE rec_exposures (
    run_id      INTEGER NOT NULL REFERENCES rec_runs (id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL,
    user_id     INTEGER NOT NULL REFERENCES users (id),
    exposed_at  INTEGER NOT NULL,
    PRIMARY KEY (run_id, rank),
    CONSTRAINT rec_exposures_rank_positive CHECK (rank > 0)
) STRICT, WITHOUT ROWID;
