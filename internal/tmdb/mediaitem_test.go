package tmdb

import (
	"encoding/json"
	"testing"
)

func TestMediaItemDecodesEachKindIntoItsOwnType(t *testing.T) {
	raw := `[
		{"media_type": "movie", "id": 1, "title": "A movie", "adult": true, "video": true},
		{"media_type": "tv", "id": 2, "name": "A series", "origin_country": ["BR"]},
		{"media_type": "person", "id": 3, "name": "A person", "known_for": [{"media_type": "movie", "id": 4}]},
		{"media_type": "collection", "id": 5, "title": "A collection"},
		{"media_type": "episode", "id": 6}
	]`
	var items []MediaItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatal(err)
	}
	if m := items[0].Movie; m == nil || m.ID != 1 || m.Title != "A movie" || !m.Adult || !m.Video {
		t.Errorf("movie row decoded as %+v", items[0])
	}
	if s := items[1].TV; s == nil || s.ID != 2 || s.MediaType != "tv" || len(s.OriginCountry) != 1 {
		t.Errorf("series row decoded as %+v", items[1])
	}
	if p := items[2].Person; p == nil || p.ID != 3 || len(p.KnownFor) != 1 || p.KnownFor[0].Movie == nil {
		t.Errorf("person row decoded as %+v", items[2])
	}
	if c := items[3].Collection; c == nil || c.ID != 5 || c.Title != "A collection" {
		t.Errorf("collection row decoded as %+v", items[3])
	}
	unknown := items[4]
	if unknown.MediaType != "episode" || unknown.Movie != nil || unknown.TV != nil || unknown.Person != nil || unknown.Collection != nil {
		t.Errorf("unknown row decoded as %+v, want only its media_type", unknown)
	}
	for i, item := range items[:4] {
		if countKinds(item) != 1 {
			t.Errorf("row %d has %d kinds set, want 1", i, countKinds(item))
		}
	}
}

func countKinds(item MediaItem) int {
	n := 0
	for _, set := range []bool{item.Movie != nil, item.TV != nil, item.Person != nil, item.Collection != nil} {
		if set {
			n++
		}
	}
	return n
}

func TestOptionalKeepsAbsentNullAndSetApart(t *testing.T) {
	var ids ExternalIDs
	if err := json.Unmarshal([]byte(`{"imdb_id": "tt1", "tvdb_id": null, "tvrage_id": 42}`), &ids); err != nil {
		t.Fatal(err)
	}
	if !ids.IMDbID.Present || ids.IMDbID.Null || ids.IMDbID.Value != "tt1" {
		t.Errorf("imdb_id = %+v, want set to tt1", ids.IMDbID)
	}
	if !ids.TVDBID.Present || !ids.TVDBID.Null {
		t.Errorf("tvdb_id = %+v, want null", ids.TVDBID)
	}
	if ids.FacebookID.Present {
		t.Errorf("facebook_id = %+v, want absent", ids.FacebookID)
	}
	out, err := json.Marshal(ids)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"imdb_id":"tt1","tvdb_id":null,"tvrage_id":42}`; string(out) != want {
		t.Errorf("re-encoded as %s, want %s", out, want)
	}
}
