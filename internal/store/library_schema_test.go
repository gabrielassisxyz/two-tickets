package store

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The library tests write rows with direct SQL through the writer pool, which
// runs foreign_keys=ON like the service does. Every rejection is matched on
// the constraint that must refuse it, so a row refused for some other reason
// (a typo in the fixture, a different rule) does not count as the rule
// holding; and every rule is paired with a row it must accept.

const (
	libraryFixtureTime = int64(1_700_000_000)
	libraryUserA       = int64(1)
	libraryUserB       = int64(2)
	libraryUnknownUser = int64(99)
)

// libraryRow maps a column to its value; a nil value is SQL NULL, and a column
// left out takes its default.
type libraryRow map[string]any

func (r libraryRow) with(column string, value any) libraryRow {
	out := maps.Clone(r)
	out[column] = value
	return out
}

func (r libraryRow) without(column string) libraryRow {
	out := maps.Clone(r)
	delete(out, column)
	return out
}

func libraryInsert(db *sql.DB, table string, row libraryRow) error {
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

func libraryEmbeddedMigrations(t *testing.T) []Migration {
	t.Helper()
	migrations, err := LoadMigrations(StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

func libraryInsertUser(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	key := fmt.Sprintf("account%d", id)
	if _, err := db.Exec(`INSERT INTO users (id, config_key, username, email, display_name, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'unused-hash', ?, ?)`,
		id, key, key, key+"@example.com", key, libraryFixtureTime, libraryFixtureTime); err != nil {
		t.Fatalf("insert user %d: %v", id, err)
	}
}

// newLibraryDatabase is a state.db at the newest schema holding the two
// accounts, the way startup leaves it before the bootstrap runs.
func newLibraryDatabase(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	db := openTestDatabase(t, filepath.Join(dir, StateFileName))
	if err := MigrateState(context.Background(), db, libraryEmbeddedMigrations(t), filepath.Join(dir, RollbackDirName)); err != nil {
		t.Fatalf("MigrateState: %v", err)
	}
	libraryInsertUser(t, db.Write, libraryUserA)
	libraryInsertUser(t, db.Write, libraryUserB)
	return db
}

func libraryExpectAccepted(t *testing.T, label string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: refused a valid row: %v", label, err)
	}
}

func libraryExpectRejected(t *testing.T, label string, err error, wantInError string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: accepted a row that %q must refuse", label, wantInError)
	}
	if !strings.Contains(err.Error(), wantInError) {
		t.Fatalf("%s: refused for the wrong reason: %v (want %q)", label, err, wantInError)
	}
}

func libraryCheckFailed(name string) string { return "CHECK constraint failed: " + name }

const (
	libraryUniqueFailed     = "UNIQUE constraint failed"
	libraryForeignKeyFailed = "FOREIGN KEY constraint failed"
)

// watches and ratings describe the same thing, a title or a season of it, and
// share every rule about which title they point at. The tests run each such
// rule against both tables.
var libraryTitleTables = []string{"watches", "ratings"}

func libraryTitleRow(table, mediaType string, tmdbID int64, scope string, season any) libraryRow {
	row := libraryRow{
		"user_id":       libraryUserA,
		"media_type":    mediaType,
		"tmdb_id":       tmdbID,
		"scope":         scope,
		"season_number": season,
		"created_at":    libraryFixtureTime,
	}
	switch table {
	case "watches":
		row["watched_at"] = libraryFixtureTime
	case "ratings":
		row["level"] = "good"
		row["scale"] = "sentiment-7-v1"
		row["updated_at"] = libraryFixtureTime
	}
	return row
}

func libraryMovieRow(table string) libraryRow {
	return libraryTitleRow(table, "movie", 550, "movie", nil)
}

func libraryShowRow(table string) libraryRow {
	return libraryTitleRow(table, "tv", 1396, "show", nil)
}

func librarySeasonRow(table string, season int64) libraryRow {
	return libraryTitleRow(table, "tv", 1396, "season", season)
}

func TestLibraryTitleRowsAreUniqueWithoutASeason(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			for _, row := range []libraryRow{libraryMovieRow(table), libraryShowRow(table)} {
				libraryExpectAccepted(t, "first row", libraryInsert(db, table, row))
				libraryExpectRejected(t, "same user, title and scope again", libraryInsert(db, table, row), libraryUniqueFailed)
				libraryExpectAccepted(t, "the other user", libraryInsert(db, table, row.with("user_id", libraryUserB)))
			}
			libraryExpectAccepted(t, "another movie", libraryInsert(db, table, libraryMovieRow(table).with("tmdb_id", int64(551))))
		})
	}
}

func TestLibraryTitleRowsAreUniquePerSeason(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectAccepted(t, "season 1", libraryInsert(db, table, librarySeasonRow(table, 1)))
			libraryExpectRejected(t, "season 1 again", libraryInsert(db, table, librarySeasonRow(table, 1)), libraryUniqueFailed)
			libraryExpectAccepted(t, "season 2", libraryInsert(db, table, librarySeasonRow(table, 2)))
			libraryExpectAccepted(t, "season 1 for the other user", libraryInsert(db, table, librarySeasonRow(table, 1).with("user_id", libraryUserB)))
			libraryExpectAccepted(t, "the whole show beside its seasons", libraryInsert(db, table, libraryShowRow(table)))
		})
	}
}

func TestLibraryScopeMatchesTheMediaType(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			want := libraryCheckFailed(table + "_scope_matches_media_type")
			libraryExpectRejected(t, "movie with scope show", libraryInsert(db, table, libraryMovieRow(table).with("scope", "show")), want)
			libraryExpectRejected(t, "tv with scope movie", libraryInsert(db, table, libraryShowRow(table).with("scope", "movie")), want)
			libraryExpectAccepted(t, "movie with scope movie", libraryInsert(db, table, libraryMovieRow(table)))
		})
	}
}

func TestLibrarySeasonNumberIsSetExactlyForSeasonScope(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			want := libraryCheckFailed(table + "_season_only_for_season_scope")
			libraryExpectRejected(t, "season scope without a number", libraryInsert(db, table, librarySeasonRow(table, 1).with("season_number", nil)), want)
			libraryExpectRejected(t, "show scope with a number", libraryInsert(db, table, libraryShowRow(table).with("season_number", int64(1))), want)
			libraryExpectRejected(t, "movie with a number", libraryInsert(db, table, libraryMovieRow(table).with("season_number", int64(1))), want)
		})
	}
}

func TestLibrarySeasonNumberIsNotNegative(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectRejected(t, "season -1", libraryInsert(db, table, librarySeasonRow(table, -1)), libraryCheckFailed(table+"_season_number_not_negative"))
			libraryExpectAccepted(t, "season 0, the specials", libraryInsert(db, table, librarySeasonRow(table, 0)))
		})
	}
}

func TestLibraryScopeAndMediaTypeAreKnownValues(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectRejected(t, "scope episode", libraryInsert(db, table, libraryShowRow(table).with("scope", "episode")), libraryCheckFailed(table+"_scope_known"))
			libraryExpectRejected(t, "media type person", libraryInsert(db, table, libraryShowRow(table).with("media_type", "person")), libraryCheckFailed(table+"_media_type_known"))
		})
	}
}

// A NULL media type would slip past every CHECK that compares it, so the
// column's NOT NULL is the only thing keeping a TMDB id tied to its type.
func TestLibraryTMDBIDNeverAppearsWithoutItsMediaType(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectRejected(t, "null media type", libraryInsert(db, table, libraryShowRow(table).with("media_type", nil)), "NOT NULL constraint failed: "+table+".media_type")
		})
	}
}

func TestLibraryTMDBIDIsPositive(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			want := libraryCheckFailed(table + "_tmdb_id_positive")
			libraryExpectRejected(t, "tmdb_id 0", libraryInsert(db, table, libraryMovieRow(table).with("tmdb_id", int64(0))), want)
			libraryExpectRejected(t, "tmdb_id -1", libraryInsert(db, table, libraryMovieRow(table).with("tmdb_id", int64(-1))), want)
			libraryExpectAccepted(t, "tmdb_id 1", libraryInsert(db, table, libraryMovieRow(table).with("tmdb_id", int64(1))))
		})
	}
}

// Without STRICT, SQLite stores 'abc' in an INTEGER column, and 'abc' > 0 is
// true, so tmdb_id_positive would pass it.
func TestLibraryTablesAreStrict(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectRejected(t, "text tmdb_id", libraryInsert(db, table, libraryMovieRow(table).with("tmdb_id", "abc")), "cannot store TEXT value in INTEGER column")
		})
	}
}

func TestLibraryRowsReferenceAnExistingUser(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectRejected(t, "unknown user", libraryInsert(db, table, libraryMovieRow(table).with("user_id", libraryUnknownUser)), libraryForeignKeyFailed)
		})
	}
}

// Accounts are never deleted by the service; if one ever is, its library must
// stop the delete rather than vanish with it.
func TestLibraryDeletingAUserWithATitleRowFails(t *testing.T) {
	for _, table := range libraryTitleTables {
		t.Run(table, func(t *testing.T) {
			db := newLibraryDatabase(t).Write
			libraryExpectAccepted(t, "row for user B", libraryInsert(db, table, libraryMovieRow(table).with("user_id", libraryUserB)))
			_, err := db.Exec("DELETE FROM users WHERE id = ?", libraryUserB)
			libraryExpectRejected(t, "delete user B", err, libraryForeignKeyFailed)
		})
	}
}

func TestLibraryWatchSourceDefaultsToManualAndIsAKnownValue(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectRejected(t, "source trakt", libraryInsert(db, "watches", libraryMovieRow("watches").with("source", "trakt")), libraryCheckFailed("watches_source_known"))
	libraryExpectAccepted(t, "no source given", libraryInsert(db, "watches", libraryMovieRow("watches").without("source")))
	var source string
	if err := db.QueryRow("SELECT source FROM watches").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "manual" {
		t.Fatalf("default source = %q, want manual", source)
	}
}

// Each listed column refuses NULL; season_number, level, scale, review,
// owner_user_id and note are the nullable ones and are covered by the rules
// that say when they may be NULL.
func TestLibraryRequiredColumnsRefuseNull(t *testing.T) {
	db := newLibraryDatabase(t)
	required := map[string][]string{
		"watches":    {"user_id", "media_type", "tmdb_id", "scope", "watched_at", "source", "created_at"},
		"ratings":    {"user_id", "media_type", "tmdb_id", "scope", "created_at", "updated_at"},
		"lists":      {"kind", "name", "created_at"},
		"list_items": {"list_id", "media_type", "tmdb_id", "added_by_user_id", "added_at"},
	}
	listID := libraryInsertCustomList(t, db.Write, libraryUserA, "Favorites")
	valid := map[string]libraryRow{
		"watches":    libraryMovieRow("watches"),
		"ratings":    libraryMovieRow("ratings"),
		"lists":      libraryListRow("custom", libraryUserA, "Other"),
		"list_items": libraryItemRow(listID, "movie", 550),
	}
	for table, columns := range required {
		for _, column := range columns {
			t.Run(table+"."+column, func(t *testing.T) {
				err := libraryInsert(db.Write, table, valid[table].with(column, nil))
				libraryExpectRejected(t, "null "+column, err, "NOT NULL constraint failed: "+table+"."+column)
			})
		}
	}
}
