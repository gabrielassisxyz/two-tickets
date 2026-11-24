package tmdb

import "encoding/json"

// MediaItem is one row of a list that mixes kinds (trending, multi search):
// a movie, a series, a person or a collection, told apart by media_type.
// Exactly one of the pointers is set for a known kind and none for any other
// kind, whose media_type is kept so a caller can see what it dropped.
type MediaItem struct {
	MediaType  string
	Movie      *MovieBrief
	TV         *TVBrief
	Person     *PersonBrief
	Collection *CollectionBrief
}

func (m *MediaItem) UnmarshalJSON(data []byte) error {
	var kind struct {
		MediaType string `json:"media_type"`
	}
	if err := json.Unmarshal(data, &kind); err != nil {
		return err
	}
	*m = MediaItem{MediaType: kind.MediaType}
	switch kind.MediaType {
	case "movie":
		m.Movie = new(MovieBrief)
		return json.Unmarshal(data, m.Movie)
	case "tv":
		m.TV = new(TVBrief)
		return json.Unmarshal(data, m.TV)
	case "person":
		m.Person = new(PersonBrief)
		return json.Unmarshal(data, m.Person)
	case "collection":
		m.Collection = new(CollectionBrief)
		return json.Unmarshal(data, m.Collection)
	}
	return nil
}
