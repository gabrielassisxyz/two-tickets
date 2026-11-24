package recs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

const (
	recsFixtureTime = int64(1_700_000_000)
	recsUserA       = int64(1)
	recsUserB       = int64(2)
)

func newRecsTestDatabaseFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, store.StateFileName)
	db, err := store.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	migrations, err := store.LoadMigrations(store.StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateState(context.Background(), db, migrations, filepath.Join(dir, store.RollbackDirName)); err != nil {
		t.Fatalf("MigrateState: %v", err)
	}
	recsInsertAccount(t, db.Write, recsUserA, "account1")
	recsInsertAccount(t, db.Write, recsUserB, "account2")
	return path
}

func recsInsertAccount(t *testing.T, db *sql.DB, id int64, key string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO users (id, config_key, username, email, display_name, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'unused-hash', ?, ?)`,
		id, key, key, key+"@example.com", key, recsFixtureTime, recsFixtureTime); err != nil {
		t.Fatalf("insert user %d: %v", id, err)
	}
}

func newRecsTestDatabase(t *testing.T) *store.Database {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, store.StateFileName)
	db, err := store.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := store.LoadMigrations(store.StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateState(context.Background(), db, migrations, filepath.Join(dir, store.RollbackDirName)); err != nil {
		t.Fatalf("MigrateState: %v", err)
	}
	recsInsertAccount(t, db.Write, recsUserA, "account1")
	recsInsertAccount(t, db.Write, recsUserB, "account2")
	return db
}

func recsInsertWatch(t *testing.T, db *sql.DB, userID, tmdbID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO watches (user_id, media_type, tmdb_id, scope, watched_at, created_at)
		VALUES (?, 'movie', ?, 'movie', ?, ?)`, userID, tmdbID, recsFixtureTime, recsFixtureTime); err != nil {
		t.Fatalf("insert watch: %v", err)
	}
}

func recsInsertRating(t *testing.T, db *sql.DB, userID, tmdbID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO ratings (user_id, media_type, tmdb_id, scope, level, scale, created_at, updated_at)
		VALUES (?, 'movie', ?, 'movie', 'good', 'sentiment-7-v1', ?, ?)`, userID, tmdbID, recsFixtureTime, recsFixtureTime); err != nil {
		t.Fatalf("insert rating: %v", err)
	}
}

func recsCreatePublished(t *testing.T, db *sql.DB, userID int64, results []Result) int64 {
	t.Helper()
	ctx := context.Background()
	watermark, err := Watermark(ctx, db, userID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := CreateRun(ctx, db, userID, "v1-test", watermark, recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db, run, results); err != nil {
		t.Fatal(err)
	}
	if err := PublishRun(ctx, db, run, recsFixtureTime); err != nil {
		t.Fatal(err)
	}
	return run
}

func recsSampleResults() []Result {
	return []Result{
		{Rank: 1, MediaType: "movie", TmdbID: 550, Score: 0.95, Reasons: []string{"Central do Brasil"}},
		{Rank: 2, MediaType: "tv", TmdbID: 1396, Score: 0.9, Reasons: []string{"Bacurau", "Aquarius"}},
		{Rank: 3, MediaType: "movie", TmdbID: 551, Score: 0.8, Reasons: []string{}},
	}
}

// An unpublished run is invisible; publishing makes it the newest completed
// run.
func TestRecsUnpublishedRunIsInvisibleUntilPublished(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	run, err := CreateRun(ctx, db.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db.Write, run, recsSampleResults()); err != nil {
		t.Fatal(err)
	}
	if page, err := ReadLatest(ctx, db.Write, recsUserA, "", 10, 0); err != nil || page.Total != 0 || len(page.Results) != 0 {
		t.Fatalf("unpublished read = %+v (%v), want empty", page, err)
	}
	if err := PublishRun(ctx, db.Write, run, recsFixtureTime); err != nil {
		t.Fatal(err)
	}
	page, err := ReadLatest(ctx, db.Write, recsUserA, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.RunID != run || page.Total != 3 || len(page.Results) != 3 {
		t.Fatalf("published read = %+v, want run %d with 3 results", page, run)
	}
	if page.Results[0].TmdbID != 550 || page.Results[1].TmdbID != 1396 {
		t.Fatalf("results out of rank order: %+v", page.Results)
	}
}

// A second run that fails before publishing leaves the first in place.
func TestRecsFailedRunLeavesThePublishedOneInPlace(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	first := recsCreatePublished(t, db.Write, recsUserA, recsSampleResults())
	second, err := CreateRun(ctx, db.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db.Write, second, recsSampleResults()); err != nil {
		t.Fatal(err)
	}
	// No PublishRun: the writer dies here.
	page, err := ReadLatest(ctx, db.Write, recsUserA, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.RunID != first {
		t.Fatalf("newest completed = %d, want %d", page.RunID, first)
	}
}

func TestRecsDuplicateRankOrTitleFails(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	run, err := CreateRun(ctx, db.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db.Write, run, recsSampleResults()[:1]); err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db.Write, run, []Result{{Rank: 1, MediaType: "tv", TmdbID: 1396, Score: 0.5, Reasons: []string{"x"}}}); err == nil {
		t.Fatal("duplicate (run_id, rank) succeeded")
	}
	if err := AddResults(ctx, db.Write, run, []Result{{Rank: 9, MediaType: "movie", TmdbID: 550, Score: 0.5, Reasons: []string{"x"}}}); err == nil {
		t.Fatal("duplicate (run_id, media_type, tmdb_id) succeeded")
	}
}

// Recording the same exposure twice returns no error and writes one row.
func TestRecsExposureIsIdempotent(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	run := recsCreatePublished(t, db.Write, recsUserA, recsSampleResults())
	if err := RecordExposure(ctx, db.Write, run, 1, recsUserA, recsFixtureTime); err != nil {
		t.Fatal(err)
	}
	if err := RecordExposure(ctx, db.Write, run, 1, recsUserA, recsFixtureTime+60); err != nil {
		t.Fatalf("repeat exposure: %v", err)
	}
	var n int
	if err := db.Write.QueryRow("SELECT count(*) FROM rec_exposures WHERE run_id = ? AND rank = ?", run, 1).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d exposures for one rank, want 1", n)
	}
}

func TestRecsWatermarkMovesOnlyForItsUser(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	empty, err := Watermark(ctx, db.Write, recsUserA)
	if err != nil {
		t.Fatal(err)
	}
	if empty != "0:0:0:0" {
		t.Fatalf("empty watermark = %q, want 0:0:0:0", empty)
	}
	recsCreatePublished(t, db.Write, recsUserA, recsSampleResults())
	moved, err := WatermarkMoved(ctx, db.Write, recsUserA)
	if err != nil || moved {
		t.Fatalf("moved = %v (%v) right after publishing, want false", moved, err)
	}
	recsInsertRating(t, db.Write, recsUserB, 550)
	if moved, err := WatermarkMoved(ctx, db.Write, recsUserA); err != nil || moved {
		t.Fatalf("another user's rating moved the watermark: %v (%v)", moved, err)
	}
	recsInsertRating(t, db.Write, recsUserA, 550)
	if moved, err := WatermarkMoved(ctx, db.Write, recsUserA); err != nil || !moved {
		t.Fatalf("a new rating did not move the watermark: %v (%v)", moved, err)
	}
	recsCreatePublished(t, db.Write, recsUserA, recsSampleResults())
	recsInsertWatch(t, db.Write, recsUserA, 551)
	if moved, err := WatermarkMoved(ctx, db.Write, recsUserA); err != nil || !moved {
		t.Fatalf("a new watch did not move the watermark: %v (%v)", moved, err)
	}
}

func TestRecsReadFiltersByMediaTypeBeforePagination(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	recsCreatePublished(t, db.Write, recsUserA, recsSampleResults())
	page, err := ReadLatest(ctx, db.Write, recsUserA, "movie", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("movie total = %d, want 2", page.Total)
	}
	for _, r := range page.Results {
		if r.MediaType != "movie" {
			t.Fatalf("movie read returned %+v", r)
		}
	}
	first, err := ReadLatest(ctx, db.Write, recsUserA, "movie", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 2 || len(first.Results) != 1 {
		t.Fatalf("paged movie read = total %d with %d rows, want 2 with 1", first.Total, len(first.Results))
	}
	second, err := ReadLatest(ctx, db.Write, recsUserA, "movie", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Results) != 1 || second.Results[0].TmdbID == first.Results[0].TmdbID {
		t.Fatalf("second movie page = %+v, want the other movie", second.Results)
	}
}

func TestRecsReasonsRoundTrip(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	want := []string{"Central do Brasil", "Bacurau", "São Paulo, Sociedade Anônima"}
	run := recsCreatePublished(t, db.Write, recsUserA, []Result{{Rank: 1, MediaType: "movie", TmdbID: 550, Score: 0.99, Reasons: want}})
	page, err := ReadLatest(ctx, db.Write, recsUserA, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.RunID != run || len(page.Results) != 1 || !slices.Equal(page.Results[0].Reasons, want) {
		t.Fatalf("round trip = %+v, want reasons %q", page, want)
	}
}

func TestRecsPublishRefusesASecondPublishOrAMissingRun(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	run, err := CreateRun(ctx, db.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := PublishRun(ctx, db.Write, run, recsFixtureTime); err != nil {
		t.Fatal(err)
	}
	if err := PublishRun(ctx, db.Write, run, recsFixtureTime); err == nil {
		t.Fatal("a second publish succeeded")
	}
	if err := PublishRun(ctx, db.Write, 9999, recsFixtureTime); err == nil {
		t.Fatal("publishing a missing run succeeded")
	}
}

func TestRecsReadRejectsAnUnknownMediaType(t *testing.T) {
	db := newRecsTestDatabase(t)
	if _, err := ReadLatest(context.Background(), db.Write, recsUserA, "person", 10, 0); err == nil {
		t.Fatal("reading with media type person succeeded")
	}
}

func TestRecsNilReasonsEncodeAsAnEmptyArray(t *testing.T) {
	db := newRecsTestDatabase(t)
	ctx := context.Background()
	run, err := CreateRun(ctx, db.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddResults(ctx, db.Write, run, []Result{{Rank: 1, MediaType: "movie", TmdbID: 550, Score: 0.9}}); err != nil {
		t.Fatalf("nil reasons: %v", err)
	}
	if err := PublishRun(ctx, db.Write, run, recsFixtureTime); err != nil {
		t.Fatal(err)
	}
	page, err := ReadLatest(ctx, db.Write, recsUserA, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 || page.Results[0].Reasons == nil || len(page.Results[0].Reasons) != 0 {
		t.Fatalf("nil reasons round trip = %+v, want an empty list", page)
	}
}

// Concurrent exposures of the same card keep one row and no error.
func TestRecsConcurrentExposuresKeepOneRow(t *testing.T) {
	path := newRecsTestDatabaseFile(t)
	opener, err := store.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opener.Close() })
	run := recsCreatePublished(t, opener.Write, recsUserA, recsSampleResults())
	const writers = 10
	errs := make([]error, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			handle, err := store.OpenDatabase(context.Background(), path)
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = handle.Close() }()
			errs[i] = RecordExposure(context.Background(), handle.Write, run, 1, recsUserA, recsFixtureTime+int64(i))
		}(i)
	}
	close(start)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatalf("a concurrent exposure failed: %v", err)
	}
	var n int
	if err := opener.Write.QueryRow("SELECT count(*) FROM rec_exposures WHERE run_id = ? AND rank = ?", run, 1).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d exposures for one rank, want 1", n)
	}
}

// Concurrent publishers both land; the newest completed run wins the read.
func TestRecsConcurrentRunsBothPublish(t *testing.T) {
	path := newRecsTestDatabaseFile(t)
	opener, err := store.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opener.Close() })
	const writers = 5
	ids := make([]int64, writers)
	errs := make([]error, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			handle, err := store.OpenDatabase(context.Background(), path)
			if err != nil {
				errs[i] = err
				return
			}
			defer func() { _ = handle.Close() }()
			ctx := context.Background()
			id, err := CreateRun(ctx, handle.Write, recsUserA, "v1-test", "0:0:0:0", recsFixtureTime+int64(i))
			if err != nil {
				errs[i] = err
				return
			}
			if err := AddResults(ctx, handle.Write, id, []Result{{Rank: 1, MediaType: "movie", TmdbID: int64(550 + i), Score: 0.9, Reasons: []string{"x"}}}); err != nil {
				errs[i] = err
				return
			}
			errs[i] = PublishRun(ctx, handle.Write, id, recsFixtureTime+int64(i))
			ids[i] = id
		}(i)
	}
	close(start)
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatalf("a concurrent publish failed: %v", err)
	}
	page, err := ReadLatest(context.Background(), opener.Write, recsUserA, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The run with the greatest (completed_at, id) wins; completed_at grows
	// with i, so the newest must hold the highest tmdb id.
	if page.Total != 1 || page.Results[0].TmdbID != int64(550+writers-1) {
		t.Fatalf("newest completed = %+v, want the run completed last", page)
	}
}
