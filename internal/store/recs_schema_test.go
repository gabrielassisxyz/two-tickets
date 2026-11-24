package store

import (
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// The recs constraint tests write rows with direct SQL through the writer
// pool, the way the library tests do. Every rejection is matched on the
// constraint that must refuse it, so a row refused for another reason does
// not count as the rule holding; every rule is paired with a row it must
// accept. Helpers reuse the library ones where they are not library-specific:
// libraryInsertUser, libraryExpectAccepted, libraryExpectRejected,
// libraryCheckFailed, libraryUniqueFailed, libraryForeignKeyFailed and
// newLibraryDatabase (now at the newest schema, recs included).

// recsRow maps a column to its value; a nil value is SQL NULL, and a column
// left out takes its default.
type recsRow map[string]any

func (r recsRow) with(column string, value any) recsRow {
	out := maps.Clone(r)
	out[column] = value
	return out
}

func recsInsert(db *sql.DB, table string, row recsRow) error {
	columns := slices.Sorted(maps.Keys(row))
	args := make([]any, len(columns))
	for i, column := range columns {
		args[i] = row[column]
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), placeholders)
	_, err := db.Exec(query, args...)
	return err
}

func recsRunRow(userID int64) recsRow {
	return recsRow{
		"user_id":    userID,
		"algorithm":  "v1-test",
		"watermark":  "0:0:0:0",
		"started_at": libraryFixtureTime,
	}
}

func recsInsertRun(t *testing.T, db *sql.DB, row recsRow) int64 {
	t.Helper()
	var id int64
	columns := slices.Sorted(maps.Keys(row))
	args := make([]any, len(columns))
	for i, column := range columns {
		args[i] = row[column]
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	query := fmt.Sprintf("INSERT INTO rec_runs (%s) VALUES (%s) RETURNING id", strings.Join(columns, ", "), placeholders)
	if err := db.QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("insert rec_runs row: %v", err)
	}
	return id
}

func recsResultRow(runID, rank int64, mediaType string, tmdbID int64) recsRow {
	return recsRow{
		"run_id":     runID,
		"rank":       rank,
		"media_type": mediaType,
		"tmdb_id":    tmdbID,
		"score":      0.9,
		"reasons":    `["Central do Brasil"]`,
	}
}

func recsExposureRow(runID, rank, userID int64) recsRow {
	return recsRow{
		"run_id":     runID,
		"rank":       rank,
		"user_id":    userID,
		"exposed_at": libraryFixtureTime,
	}
}

func TestRecsRunsReferenceAnExistingUser(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectRejected(t, "unknown user", recsInsert(db, "rec_runs", recsRunRow(libraryUnknownUser)), libraryForeignKeyFailed)
	libraryExpectAccepted(t, "known user", recsInsert(db, "rec_runs", recsRunRow(libraryUserA)))
}

func TestRecsRunsRequireAlgorithmAndWatermark(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectRejected(t, "empty algorithm", recsInsert(db, "rec_runs", recsRunRow(libraryUserA).with("algorithm", "")), libraryCheckFailed("rec_runs_algorithm_not_empty"))
	libraryExpectRejected(t, "empty watermark", recsInsert(db, "rec_runs", recsRunRow(libraryUserA).with("watermark", "")), libraryCheckFailed("rec_runs_watermark_not_empty"))
	libraryExpectAccepted(t, "named algorithm and watermark", recsInsert(db, "rec_runs", recsRunRow(libraryUserA)))
}

func TestRecsRunsCompletedAtIsNullUntilPublished(t *testing.T) {
	db := newLibraryDatabase(t).Write
	id := recsInsertRun(t, db, recsRunRow(libraryUserA))
	var completed any
	if err := db.QueryRow("SELECT completed_at FROM rec_runs WHERE id = ?", id).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != nil {
		t.Fatalf("completed_at = %v for a fresh run, want NULL", completed)
	}
	libraryExpectRejected(t, "completed before start", recsInsert(db, "rec_runs",
		recsRunRow(libraryUserA).with("completed_at", libraryFixtureTime-1)), libraryCheckFailed("rec_runs_completed_after_start"))
	libraryExpectAccepted(t, "completed at start", recsInsert(db, "rec_runs",
		recsRunRow(libraryUserA).with("completed_at", libraryFixtureTime)))
}

// Accounts are never deleted by the service; if one ever is, its runs must
// stop the delete rather than vanish with it.
func TestRecsDeletingAUserWithARunFails(t *testing.T) {
	db := newLibraryDatabase(t).Write
	recsInsertRun(t, db, recsRunRow(libraryUserB))
	_, err := db.Exec("DELETE FROM users WHERE id = ?", libraryUserB)
	libraryExpectRejected(t, "delete user B", err, libraryForeignKeyFailed)
}

func TestRecsDeletingARunCascadesItsResultsAndExposures(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectAccepted(t, "result", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550)))
	libraryExpectAccepted(t, "exposure", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA)))
	if _, err := db.Exec("DELETE FROM rec_runs WHERE id = ?", run); err != nil {
		t.Fatalf("delete a run with rows: %v", err)
	}
	if n := libraryCount(t, db, "SELECT count(*) FROM rec_results WHERE run_id = ?", run); n != 0 {
		t.Fatalf("%d results outlived their run", n)
	}
	if n := libraryCount(t, db, "SELECT count(*) FROM rec_exposures WHERE run_id = ?", run); n != 0 {
		t.Fatalf("%d exposures outlived their run", n)
	}
}

func TestRecsResultsAreUniquePerRank(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	other := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectAccepted(t, "rank 1", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550)))
	libraryExpectRejected(t, "rank 1 again", recsInsert(db, "rec_results", recsResultRow(run, 1, "tv", 1396)), libraryUniqueFailed)
	libraryExpectAccepted(t, "rank 2", recsInsert(db, "rec_results", recsResultRow(run, 2, "tv", 1396)))
	libraryExpectAccepted(t, "rank 1 in another run", recsInsert(db, "rec_results", recsResultRow(other, 1, "movie", 550)))
}

// Movie 1396 and show 1396 are different titles, so the key must hold both.
func TestRecsResultsAreUniquePerTitle(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectAccepted(t, "movie 1396", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 1396)))
	libraryExpectRejected(t, "movie 1396 again at another rank", recsInsert(db, "rec_results", recsResultRow(run, 2, "movie", 1396)), libraryUniqueFailed)
	libraryExpectAccepted(t, "show 1396", recsInsert(db, "rec_results", recsResultRow(run, 2, "tv", 1396)))
}

func TestRecsResultsRankMediaTypeAndIDAreValid(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	base := recsResultRow(run, 1, "movie", 550)
	libraryExpectRejected(t, "rank 0", recsInsert(db, "rec_results", base.with("rank", int64(0))), libraryCheckFailed("rec_results_rank_positive"))
	libraryExpectRejected(t, "media type person", recsInsert(db, "rec_results", base.with("media_type", "person")), libraryCheckFailed("rec_results_media_type_known"))
	libraryExpectRejected(t, "tmdb_id 0", recsInsert(db, "rec_results", base.with("tmdb_id", int64(0))), libraryCheckFailed("rec_results_tmdb_id_positive"))
	libraryExpectRejected(t, "unknown run", recsInsert(db, "rec_results", base.with("run_id", int64(9999))), libraryForeignKeyFailed)
}

// A NULL media type would slip past every CHECK that compares it, so the
// column's NOT NULL is the only thing keeping a TMDB id tied to its type.
func TestRecsResultsTMDBIDNeverAppearsWithoutItsMediaType(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectRejected(t, "null media type", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("media_type", nil)), "NOT NULL constraint failed: rec_results.media_type")
}

func TestRecsResultsReasonsIsAJSONArray(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	want := libraryCheckFailed("rec_results_reasons_is_json_array")
	libraryExpectRejected(t, "not JSON", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("reasons", "Central")), want)
	libraryExpectRejected(t, "JSON object", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("reasons", `{"title":"Central"}`)), want)
	libraryExpectRejected(t, "JSON string", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("reasons", `"Central"`)), want)
	libraryExpectAccepted(t, "empty array", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("reasons", `[]`)))
	libraryExpectAccepted(t, "seed titles", recsInsert(db, "rec_results", recsResultRow(run, 2, "movie", 551).with("reasons", `["Central do Brasil", "Bacurau"]`)))
}

// Without STRICT, SQLite stores 'abc' in an INTEGER column, and 'abc' > 0 is
// true, so tmdb_id_positive would pass it.
func TestRecsTablesAreStrict(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectRejected(t, "text tmdb_id", recsInsert(db, "rec_results", recsResultRow(run, 1, "movie", 550).with("tmdb_id", "abc")), "cannot store TEXT value in INTEGER column")
	libraryExpectRejected(t, "text exposure rank", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA).with("rank", "first")), "cannot store TEXT value in INTEGER column")
	libraryExpectRejected(t, "text run user", recsInsert(db, "rec_runs", recsRunRow(libraryUserA).with("user_id", "someone")), "cannot store TEXT value in INTEGER column")
}

func TestRecsExposuresAreUniquePerRank(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectAccepted(t, "first exposure of rank 1", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA)))
	err := recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA))
	if err == nil {
		t.Fatal("accepted a second exposure for the same (run_id, rank)")
	}
	if msg := err.Error(); !strings.Contains(msg, "PRIMARY KEY constraint failed") && !strings.Contains(msg, libraryUniqueFailed) {
		t.Fatalf("second exposure refused for the wrong reason: %v", err)
	}
	libraryExpectAccepted(t, "rank 2", recsInsert(db, "rec_exposures", recsExposureRow(run, 2, libraryUserA)))
}

func TestRecsExposuresReferenceARunAndAUser(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserA))
	libraryExpectRejected(t, "unknown run", recsInsert(db, "rec_exposures", recsExposureRow(9999, 1, libraryUserA)), libraryForeignKeyFailed)
	libraryExpectRejected(t, "unknown user", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUnknownUser)), libraryForeignKeyFailed)
	libraryExpectRejected(t, "rank 0", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA).with("rank", int64(0))), libraryCheckFailed("rec_exposures_rank_positive"))
	libraryExpectAccepted(t, "valid exposure", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserA)))
}

func TestRecsDeletingAUserWithAnExposureFails(t *testing.T) {
	db := newLibraryDatabase(t).Write
	run := recsInsertRun(t, db, recsRunRow(libraryUserB))
	libraryExpectAccepted(t, "exposure for user B", recsInsert(db, "rec_exposures", recsExposureRow(run, 1, libraryUserB)))
	_, err := db.Exec("DELETE FROM users WHERE id = ?", libraryUserB)
	libraryExpectRejected(t, "delete user B", err, libraryForeignKeyFailed)
}

// Each listed column refuses NULL; reasons, completed_at and the nullable
// library columns are covered by the rules that say when they may be NULL.
func TestRecsRequiredColumnsRefuseNull(t *testing.T) {
	db := newLibraryDatabase(t)
	run := recsInsertRun(t, db.Write, recsRunRow(libraryUserA))
	required := map[string][]string{
		"rec_runs":      {"user_id", "algorithm", "watermark", "started_at"},
		"rec_results":   {"run_id", "rank", "media_type", "tmdb_id", "score", "reasons"},
		"rec_exposures": {"run_id", "rank", "user_id", "exposed_at"},
	}
	valid := map[string]recsRow{
		"rec_runs":      recsRunRow(libraryUserA),
		"rec_results":   recsResultRow(run, 1, "movie", 550),
		"rec_exposures": recsExposureRow(run, 1, libraryUserA),
	}
	for table, columns := range required {
		for _, column := range columns {
			t.Run(table+"."+column, func(t *testing.T) {
				// Failed inserts leave no row behind, so every subtest can
				// reuse the same valid key: the NULL is what refuses it.
				err := recsInsert(db.Write, table, valid[table].with(column, nil))
				libraryExpectRejected(t, "null "+column, err, "NOT NULL constraint failed: "+table+"."+column)
			})
		}
	}
}
