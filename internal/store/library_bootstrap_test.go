package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const libraryConcurrentRuns = 10

// newLibraryDatabaseFile is newLibraryDatabase on a path the caller keeps, so
// the concurrent tests can open one Database per goroutine: each has its own
// writer connection, the way two processes on the same file would.
func newLibraryDatabaseFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := MigrateState(context.Background(), db, libraryEmbeddedMigrations(t), filepath.Join(dir, RollbackDirName)); err != nil {
		t.Fatalf("MigrateState: %v", err)
	}
	libraryInsertUser(t, db.Write, libraryUserA)
	libraryInsertUser(t, db.Write, libraryUserB)
	return path
}

// libraryRunConcurrently opens one Database per run, releases every run at the
// same moment, and returns each run's error.
func libraryRunConcurrently(t *testing.T, path string, run func(i int, db *Database) error) []error {
	t.Helper()
	handles := make([]*Database, libraryConcurrentRuns)
	for i := range handles {
		handles[i] = openTestDatabase(t, path)
	}
	errs := make([]error, libraryConcurrentRuns)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, db := range handles {
		wg.Go(func() {
			<-start
			errs[i] = run(i, db)
		})
	}
	close(start)
	wg.Wait()
	return errs
}

type libraryListSummary struct {
	couple     int
	watchlists []int64 // owner ids, ascending
	total      int
}

func libraryReadLists(t *testing.T, db *sql.DB) libraryListSummary {
	t.Helper()
	var s libraryListSummary
	s.couple = libraryCount(t, db, "SELECT count(*) FROM lists WHERE kind = 'couple' AND owner_user_id IS NULL")
	s.total = libraryCount(t, db, "SELECT count(*) FROM lists")
	rows, err := db.Query("SELECT owner_user_id FROM lists WHERE kind = 'watchlist' ORDER BY owner_user_id")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var owner int64
		if err := rows.Scan(&owner); err != nil {
			t.Fatal(err)
		}
		s.watchlists = append(s.watchlists, owner)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return s
}

func libraryRequireBootstrappedLists(t *testing.T, db *sql.DB, owners ...int64) {
	t.Helper()
	got := libraryReadLists(t, db)
	if got.couple != 1 || !slices.Equal(got.watchlists, owners) || got.total != 1+len(owners) {
		t.Fatalf("lists = %+v, want one couple list and one watchlist for each of %v", got, owners)
	}
}

func TestBootstrapLibraryListsCreatesTheSingletonLists(t *testing.T) {
	db := newLibraryDatabase(t)
	if err := BootstrapLibraryLists(context.Background(), db, time.Unix(libraryFixtureTime, 0)); err != nil {
		t.Fatalf("BootstrapLibraryLists: %v", err)
	}
	libraryRequireBootstrappedLists(t, db.Write, libraryUserA, libraryUserB)
}

func TestBootstrapLibraryListsRunConcurrentlyLeavesOneOfEach(t *testing.T) {
	path := newLibraryDatabaseFile(t)
	errs := libraryRunConcurrently(t, path, func(_ int, db *Database) error {
		return BootstrapLibraryLists(context.Background(), db, time.Unix(libraryFixtureTime, 0))
	})
	if err := errors.Join(errs...); err != nil {
		t.Fatalf("a concurrent bootstrap failed: %v", err)
	}
	libraryRequireBootstrappedLists(t, openTestDatabase(t, path).Read, libraryUserA, libraryUserB)
}

// A rerun must leave existing lists, and what is in them, alone, and still
// give an account added since then its watchlist.
func TestBootstrapLibraryListsRerunKeepsExistingLists(t *testing.T) {
	db := newLibraryDatabase(t)
	ctx := context.Background()
	if err := BootstrapLibraryLists(ctx, db, time.Unix(libraryFixtureTime, 0)); err != nil {
		t.Fatal(err)
	}
	var coupleID int64
	if err := db.Write.QueryRow("SELECT id FROM lists WHERE kind = 'couple'").Scan(&coupleID); err != nil {
		t.Fatal(err)
	}
	libraryExpectAccepted(t, "item in the couple list", libraryInsert(db.Write, "list_items", libraryItemRow(coupleID, "movie", 550)))
	libraryInsertUser(t, db.Write, 3)

	if err := BootstrapLibraryLists(ctx, db, time.Unix(libraryFixtureTime+60, 0)); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	libraryRequireBootstrappedLists(t, db.Write, libraryUserA, libraryUserB, 3)
	if n := libraryCount(t, db.Write, "SELECT count(*) FROM list_items WHERE list_id = ?", coupleID); n != 1 {
		t.Fatalf("the couple list holds %d items after a rerun, want 1", n)
	}
}

// The couple list is created after the watchlists; a trigger that refuses it
// shows whether the watchlists were committed on their own.
func TestBootstrapLibraryListsIsOneTransaction(t *testing.T) {
	db := newLibraryDatabase(t)
	if _, err := db.Write.Exec(`CREATE TRIGGER refuse_couple_list BEFORE INSERT ON lists WHEN NEW.kind = 'couple'
		BEGIN SELECT RAISE(ABORT, 'couple list refused by the test'); END`); err != nil {
		t.Fatal(err)
	}
	err := BootstrapLibraryLists(context.Background(), db, time.Unix(libraryFixtureTime, 0))
	if err == nil || !strings.Contains(err.Error(), "couple list refused by the test") {
		t.Fatalf("BootstrapLibraryLists error = %v, want the trigger's refusal", err)
	}
	if n := libraryCount(t, db.Write, "SELECT count(*) FROM lists"); n != 0 {
		t.Fatalf("a failed bootstrap left %d lists behind", n)
	}
}

// Writers on separate connections contend for the same key: the unique index
// must let exactly one through, and distinct keys must all land.
func TestLibraryConcurrentWritesKeepOneRowPerKey(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table+"/same season", func(t *testing.T) {
			path := newLibraryDatabaseFile(t)
			errs := libraryRunConcurrently(t, path, func(_ int, db *Database) error {
				return libraryInsert(db.Write, table, librarySeasonRow(table, 1))
			})
			libraryRequireOneWinner(t, errs)
			if n := libraryCount(t, openTestDatabase(t, path).Read, fmt.Sprintf("SELECT count(*) FROM %s", table)); n != 1 {
				t.Fatalf("%d rows for one key", n)
			}
		})
		t.Run(table+"/distinct seasons", func(t *testing.T) {
			path := newLibraryDatabaseFile(t)
			errs := libraryRunConcurrently(t, path, func(i int, db *Database) error {
				return libraryInsert(db.Write, table, librarySeasonRow(table, int64(i)))
			})
			if err := errors.Join(errs...); err != nil {
				t.Fatalf("a write to a distinct key failed: %v", err)
			}
			if n := libraryCount(t, openTestDatabase(t, path).Read, fmt.Sprintf("SELECT count(*) FROM %s", table)); n != libraryConcurrentRuns {
				t.Fatalf("%d rows, want %d", n, libraryConcurrentRuns)
			}
		})
	}
}

func libraryRequireOneWinner(t *testing.T, errs []error) {
	t.Helper()
	succeeded := 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !strings.Contains(err.Error(), libraryUniqueFailed):
			t.Fatalf("a losing write failed for another reason: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("%d writes of the same key succeeded, want exactly 1", succeeded)
	}
}
