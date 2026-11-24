package library

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
)

func TestListRules(t *testing.T) {
	kit := newLibraryKit(t)
	aliceWatchlist := kit.listIDOf(t, listKindWatchlist, kit.alice.ID)
	bobWatchlist := kit.listIDOf(t, listKindWatchlist, kit.bob.ID)
	couple := kit.listIDOf(t, listKindCouple, 0)
	created := decodeLibraryJSON[listBody](t, kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/library/lists", `{"name":"  Noir  "}`, http.StatusCreated))
	if created.Kind != listKindCustom || created.Name != "Noir" || created.Owner != listOwnerMe || !created.CanRename {
		t.Fatalf("created list = %+v", created)
	}
	bobCustom := created.ID

	// Watchlists and the couple list are fixed, whoever asks.
	for _, id := range []int64{aliceWatchlist, bobWatchlist, couple} {
		for _, caller := range []struct {
			name string
			resp libraryResponse
		}{
			{"alice renames", kit.serve(t, kit.alice, http.MethodPatch, fmt.Sprintf("/api/v1/library/lists/%d", id), `{"name":"x"}`)},
			{"bob renames", kit.serve(t, kit.bob, http.MethodPatch, fmt.Sprintf("/api/v1/library/lists/%d", id), `{"name":"x"}`)},
			{"alice deletes", kit.serve(t, kit.alice, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d", id), "")},
			{"bob deletes", kit.serve(t, kit.bob, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d", id), "")},
		} {
			if caller.resp.status != http.StatusForbidden {
				t.Errorf("list %d: %s = %d %s, want 403", id, caller.name, caller.resp.status, caller.resp.body)
			}
		}
	}
	// The partner's custom list is read-only to the caller.
	for name, resp := range map[string]libraryResponse{
		"rename":           kit.serve(t, kit.alice, http.MethodPatch, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), `{"name":"mine now"}`),
		"delete":           kit.serve(t, kit.alice, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), ""),
		"add item":         kit.serve(t, kit.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", bobCustom), `{"mediaType":"movie","tmdbId":1}`),
		"remove item":      kit.serve(t, kit.alice, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d/items/movie/1", bobCustom), ""),
		"add to watchlist": kit.serve(t, kit.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", bobWatchlist), `{"mediaType":"movie","tmdbId":1}`),
	} {
		if resp.status != http.StatusForbidden {
			t.Errorf("alice %s on bob's list = %d %s, want 403", name, resp.status, resp.body)
		}
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM lists WHERE id = ? AND name = 'Noir'`, bobCustom); got != 1 {
		t.Errorf("bob's list changed under alice's refused rename")
	}
	if got := kit.countRows(t, `SELECT COUNT(*) FROM list_items`); got != 0 {
		t.Errorf("refused item adds stored %d rows", got)
	}
	// Both add to and remove from the couple list.
	kit.mustServe(t, kit.alice, http.MethodPut, fmt.Sprintf("/api/v1/library/lists/%d/items", couple), `{"mediaType":"movie","tmdbId":9}`, http.StatusOK)
	kit.mustServe(t, kit.bob, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d/items/movie/9", couple), "", http.StatusNoContent)
	// The owner renames and deletes; deleting again is a no-op.
	renamed := decodeLibraryJSON[listBody](t, kit.mustServe(t, kit.bob, http.MethodPatch, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), `{"name":"Neo-noir"}`, http.StatusOK))
	if renamed.Name != "Neo-noir" {
		t.Errorf("renamed = %+v", renamed)
	}
	kit.mustServe(t, kit.bob, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), "", http.StatusNoContent)
	kit.mustServe(t, kit.bob, http.MethodDelete, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), "", http.StatusNoContent)
	kit.mustServe(t, kit.bob, http.MethodPatch, fmt.Sprintf("/api/v1/library/lists/%d", bobCustom), `{"name":"gone"}`, http.StatusNotFound)
	kit.mustServe(t, kit.bob, http.MethodGet, fmt.Sprintf("/api/v1/library/lists/%d/items", bobCustom), "", http.StatusNotFound)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/library/lists", `{"name":"   "}`, http.StatusBadRequest)
}

// Adding the same item twice leaves one row and answers 200; removing an item
// that is not there answers 204.
func TestListItemsAreIdempotent(t *testing.T) {
	kit := newLibraryKit(t)
	watchlist := kit.listIDOf(t, listKindWatchlist, kit.alice.ID)
	path := fmt.Sprintf("/api/v1/library/lists/%d/items", watchlist)
	first := decodeLibraryJSON[listItemPutAnswer](t, kit.mustServe(t, kit.alice, http.MethodPut, path, `{"mediaType":"tv","tmdbId":7,"note":"after work"}`, http.StatusOK))
	kit.clock.Advance(time.Hour)
	second := decodeLibraryJSON[listItemPutAnswer](t, kit.mustServe(t, kit.alice, http.MethodPut, path, `{"mediaType":"tv","tmdbId":7}`, http.StatusOK))
	if got := kit.countRows(t, `SELECT COUNT(*) FROM list_items WHERE list_id = ?`, watchlist); got != 1 {
		t.Fatalf("rows after adding twice = %d, want 1", got)
	}
	if second.AddedAt != first.AddedAt || second.Note == nil || *second.Note != "after work" {
		t.Errorf("second add = %+v, want the first add's time and note kept", second)
	}
	kit.mustServe(t, kit.alice, http.MethodDelete, path+"/tv/7", "", http.StatusNoContent)
	kit.mustServe(t, kit.alice, http.MethodDelete, path+"/tv/7", "", http.StatusNoContent)
	kit.mustServe(t, kit.alice, http.MethodDelete, path+"/movie/12345", "", http.StatusNoContent)
	if got := kit.countRows(t, `SELECT COUNT(*) FROM list_items`); got != 0 {
		t.Errorf("rows after removing = %d", got)
	}
}

func TestListsPageShowsBothPeoplesListsWithCounts(t *testing.T) {
	kit := newLibraryKit(t)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":1,"mediaType":"movie"}`, http.StatusCreated)
	kit.mustServe(t, kit.bob, http.MethodPost, "/api/v1/watchlist", `{"tmdbId":2,"mediaType":"movie"}`, http.StatusCreated)
	kit.mustServe(t, kit.alice, http.MethodPost, "/api/v1/library/lists", `{"name":"Mine"}`, http.StatusCreated)

	lists := decodeLibraryJSON[listsBody](t, kit.mustServe(t, kit.alice, http.MethodGet, "/api/v1/library/lists", "", http.StatusOK)).Results

	var shape []string
	for _, list := range lists {
		shape = append(shape, fmt.Sprintf("%s:%s:%d", list.Owner, list.Kind, list.ItemCount))
	}
	want := []string{"me:watchlist:0", "me:custom:0", "shared:couple:0", "partner:watchlist:2"}
	if !slices.Equal(shape, want) {
		t.Errorf("lists = %v, want %v", shape, want)
	}
}
