package catalogtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// jsonObject is a decoded JSON object that remembers its key order. The
// comparison rule reports differences in the order JavaScript's Object.keys
// yields them, and a Go map would lose that order.
type jsonObject struct {
	keys   []string
	values map[string]any
}

func newJSONObject() *jsonObject {
	return &jsonObject{values: map[string]any{}}
}

// set mirrors assignment in JSON.parse: a repeated key keeps the position of
// its first occurrence and takes the value of its last.
func (o *jsonObject) set(key string, value any) {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

func (o *jsonObject) has(key string) bool {
	_, exists := o.values[key]
	return exists
}

// objectKeys is Object.keys: array-index keys first, ascending by value, then
// every other key in insertion order.
func (o *jsonObject) objectKeys() []string {
	var indexKeys, otherKeys []string
	for _, key := range o.keys {
		if isArrayIndexKey(key) {
			indexKeys = append(indexKeys, key)
		} else {
			otherKeys = append(otherKeys, key)
		}
	}
	sort.Slice(indexKeys, func(i, j int) bool {
		left, _ := strconv.ParseUint(indexKeys[i], 10, 64)
		right, _ := strconv.ParseUint(indexKeys[j], 10, 64)
		return left < right
	})
	return append(indexKeys, otherKeys...)
}

// isArrayIndexKey reports whether JavaScript treats key as an array index: the
// canonical decimal form of an integer from 0 to 2^32-2.
func isArrayIndexKey(key string) bool {
	if key == "" || (len(key) > 1 && key[0] == '0') {
		return false
	}
	for _, r := range key {
		if r < '0' || r > '9' {
			return false
		}
	}
	value, err := strconv.ParseUint(key, 10, 64)
	return err == nil && value <= math.MaxUint32-1
}

// decodeOrderedJSON parses one JSON document into nil, bool, float64, string,
// []any and *jsonObject. Numbers become float64 because that is what
// JSON.parse produces, so equality and formatting follow JavaScript.
func decodeOrderedJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeOrderedValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected data after the JSON value")
	}
	return value, nil
}

func decodeOrderedValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token := token.(type) {
	case json.Delim:
		if token == '{' {
			return decodeOrderedObject(decoder)
		}
		if token == '[' {
			return decodeOrderedArray(decoder)
		}
		return nil, fmt.Errorf("unexpected %q", token)
	case json.Number:
		// An out-of-range literal parses to ±Inf, as it does in JSON.parse.
		number, err := strconv.ParseFloat(string(token), 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, err
		}
		return number, nil
	default:
		return token, nil
	}
}

func decodeOrderedObject(decoder *json.Decoder) (any, error) {
	object := newJSONObject()
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		value, err := decodeOrderedValue(decoder)
		if err != nil {
			return nil, err
		}
		object.set(keyToken.(string), value)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return object, nil
}

func decodeOrderedArray(decoder *json.Decoder) (any, error) {
	items := []any{}
	for decoder.More() {
		value, err := decodeOrderedValue(decoder)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return items, nil
}

// jsonTypeOf is jsonTypeOf in golden.mjs: typeof, with null and arrays told
// apart from objects.
func jsonTypeOf(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		return "object"
	}
}

// stringifyPrimitive is JSON.stringify for the values the rule prints: a
// string, a number or a boolean.
func stringifyPrimitive(value any) string {
	switch value := value.(type) {
	case string:
		return stringifyString(value)
	case float64:
		return stringifyNumber(value)
	case bool:
		return strconv.FormatBool(value)
	default:
		return "null"
	}
}

func stringifyString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

// stringifyNumber is JavaScript's Number toString: the shortest digits that
// round-trip, in decimal notation from 1e-6 up to 1e21 and in exponent
// notation ("1e+21", "1.5e-7") outside it.
func stringifyNumber(value float64) string {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return "null"
	}
	if value == 0 {
		return "0"
	}
	magnitude := math.Abs(value)
	if magnitude >= 1e-6 && magnitude < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	formatted := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponent, _ := strings.Cut(formatted, "e")
	sign, digits := exponent[:1], strings.TrimLeft(exponent[1:], "0")
	return mantissa + "e" + sign + digits
}
