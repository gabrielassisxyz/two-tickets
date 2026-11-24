package store

import (
	"database/sql"
	"testing"
)

func libraryListRow(kind string, owner any, name string) libraryRow {
	return libraryRow{"kind": kind, "owner_user_id": owner, "name": name, "created_at": libraryFixtureTime}
}

func libraryItemRow(listID int64, mediaType string, tmdbID int64) libraryRow {
	return libraryRow{
		"list_id":          listID,
		"media_type":       mediaType,
		"tmdb_id":          tmdbID,
		"added_by_user_id": libraryUserA,
		"added_at":         libraryFixtureTime,
	}
}

func libraryInsertCustomList(t *testing.T, db *sql.DB, owner int64, name string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow("INSERT INTO lists (kind, owner_user_id, name, created_at) VALUES ('custom', ?, ?, ?) RETURNING id",
		owner, name, libraryFixtureTime).Scan(&id)
	if err != nil {
		t.Fatalf("insert custom list: %v", err)
	}
	return id
}

func libraryCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLibraryThereIsOneCoupleList(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectAccepted(t, "first couple list", libraryInsert(db, "lists", libraryListRow("couple", nil, "Couple")))
	libraryExpectRejected(t, "second couple list", libraryInsert(db, "lists", libraryListRow("couple", nil, "Another")), libraryUniqueFailed)
}

func TestLibraryEachOwnerHasOneWatchlist(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectAccepted(t, "A's watchlist", libraryInsert(db, "lists", libraryListRow("watchlist", libraryUserA, "Watchlist")))
	libraryExpectRejected(t, "A's second watchlist", libraryInsert(db, "lists", libraryListRow("watchlist", libraryUserA, "Again")), libraryUniqueFailed)
	libraryExpectAccepted(t, "B's watchlist", libraryInsert(db, "lists", libraryListRow("watchlist", libraryUserB, "Watchlist")))
	libraryExpectAccepted(t, "A's first custom list", libraryInsert(db, "lists", libraryListRow("custom", libraryUserA, "Horror")))
	libraryExpectAccepted(t, "A's second custom list", libraryInsert(db, "lists", libraryListRow("custom", libraryUserA, "Horror")))
}

func TestLibraryOnlyTheCoupleListHasNoOwner(t *testing.T) {
	db := newLibraryDatabase(t).Write
	want := libraryCheckFailed("lists_owner_null_only_for_couple")
	libraryExpectRejected(t, "watchlist without an owner", libraryInsert(db, "lists", libraryListRow("watchlist", nil, "Watchlist")), want)
	libraryExpectRejected(t, "custom list without an owner", libraryInsert(db, "lists", libraryListRow("custom", nil, "Horror")), want)
	libraryExpectRejected(t, "couple list with an owner", libraryInsert(db, "lists", libraryListRow("couple", libraryUserA, "Couple")), want)
}

func TestLibraryListKindAndNameAreValid(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectRejected(t, "kind shared", libraryInsert(db, "lists", libraryListRow("shared", libraryUserA, "Shared")), libraryCheckFailed("lists_kind_known"))
	libraryExpectRejected(t, "empty name", libraryInsert(db, "lists", libraryListRow("custom", libraryUserA, "")), libraryCheckFailed("lists_name_not_empty"))
	libraryExpectRejected(t, "unknown owner", libraryInsert(db, "lists", libraryListRow("custom", libraryUnknownUser, "Horror")), libraryForeignKeyFailed)
}

func TestLibraryDeletingAUserWithAListFails(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryInsertCustomList(t, db, libraryUserB, "Horror")
	_, err := db.Exec("DELETE FROM users WHERE id = ?", libraryUserB)
	libraryExpectRejected(t, "delete the owner", err, libraryForeignKeyFailed)
}

func TestLibraryDeletingAListCascadesItsItems(t *testing.T) {
	db := newLibraryDatabase(t).Write
	doomed := libraryInsertCustomList(t, db, libraryUserA, "Doomed")
	kept := libraryInsertCustomList(t, db, libraryUserA, "Kept")
	for _, row := range []libraryRow{libraryItemRow(doomed, "movie", 550), libraryItemRow(doomed, "tv", 1396), libraryItemRow(kept, "movie", 550)} {
		libraryExpectAccepted(t, "item", libraryInsert(db, "list_items", row))
	}
	if _, err := db.Exec("DELETE FROM lists WHERE id = ?", doomed); err != nil {
		t.Fatalf("delete a list with items: %v", err)
	}
	if n := libraryCount(t, db, "SELECT count(*) FROM list_items WHERE list_id = ?", doomed); n != 0 {
		t.Fatalf("%d items outlived their list", n)
	}
	if n := libraryCount(t, db, "SELECT count(*) FROM list_items WHERE list_id = ?", kept); n != 1 {
		t.Fatalf("the other list holds %d items, want 1", n)
	}
}

// Movie 1396 and show 1396 are different titles, so the key must hold both.
func TestLibraryListItemIsUniquePerListAndTitle(t *testing.T) {
	db := newLibraryDatabase(t).Write
	list := libraryInsertCustomList(t, db, libraryUserA, "Horror")
	other := libraryInsertCustomList(t, db, libraryUserA, "Drama")
	libraryExpectAccepted(t, "movie 1396", libraryInsert(db, "list_items", libraryItemRow(list, "movie", 1396)))
	libraryExpectRejected(t, "movie 1396 again", libraryInsert(db, "list_items", libraryItemRow(list, "movie", 1396).with("added_by_user_id", libraryUserB)), libraryUniqueFailed)
	libraryExpectAccepted(t, "show 1396", libraryInsert(db, "list_items", libraryItemRow(list, "tv", 1396)))
	libraryExpectAccepted(t, "movie 1396 in another list", libraryInsert(db, "list_items", libraryItemRow(other, "movie", 1396)))
}

func TestLibraryListItemReferencesAndTitleAreValid(t *testing.T) {
	db := newLibraryDatabase(t).Write
	list := libraryInsertCustomList(t, db, libraryUserA, "Horror")
	item := libraryItemRow(list, "movie", 550)
	libraryExpectRejected(t, "unknown list", libraryInsert(db, "list_items", item.with("list_id", int64(9999))), libraryForeignKeyFailed)
	libraryExpectRejected(t, "unknown adder", libraryInsert(db, "list_items", item.with("added_by_user_id", libraryUnknownUser)), libraryForeignKeyFailed)
	libraryExpectRejected(t, "media type person", libraryInsert(db, "list_items", item.with("media_type", "person")), libraryCheckFailed("list_items_media_type_known"))
	libraryExpectRejected(t, "tmdb_id 0", libraryInsert(db, "list_items", item.with("tmdb_id", int64(0))), libraryCheckFailed("list_items_tmdb_id_positive"))
	libraryExpectRejected(t, "tmdb_id -1", libraryInsert(db, "list_items", item.with("tmdb_id", int64(-1))), libraryCheckFailed("list_items_tmdb_id_positive"))
	libraryExpectRejected(t, "text tmdb_id", libraryInsert(db, "list_items", item.with("tmdb_id", "abc")), "cannot store TEXT value in INTEGER column")
	libraryExpectAccepted(t, "valid item with a note", libraryInsert(db, "list_items", item.with("note", "Halloween")))
}

func TestLibraryListsTableIsStrict(t *testing.T) {
	db := newLibraryDatabase(t).Write
	libraryExpectRejected(t, "text owner", libraryInsert(db, "lists", libraryListRow("custom", "someone", "Horror")), "cannot store TEXT value in INTEGER column")
}
