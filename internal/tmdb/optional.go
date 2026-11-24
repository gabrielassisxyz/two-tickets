package tmdb

import (
	"bytes"
	"encoding/json"
)

// Optional is a JSON field that can be absent, null or set. TMDB uses all
// three for the same key (a movie's external_ids has no tvdb_id at all, a
// series' has "tvdb_id": null), and Seerr's responses pass that difference
// through, so a pointer, which has only two states, would lose it.
//
// Tag the field `omitzero`: the zero value is the absent state, and it is
// what decoding leaves behind when the key is missing.
type Optional[T any] struct {
	Value   T
	Null    bool
	Present bool
}

// IsZero reports the absent state, which `omitzero` drops from the output.
func (o Optional[T]) IsZero() bool { return !o.Present }

func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.Null {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// UnmarshalJSON only runs when the key is in the document, so it always
// marks the field present.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	*o = Optional[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}
