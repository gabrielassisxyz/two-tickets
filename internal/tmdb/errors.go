package tmdb

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// TMDBError is a failed TMDB answer without the API key. The path is stored
// redacted and the message never carries the query string.
type TMDBError struct {
	Status  int
	Path    string
	Message string
}

func (e *TMDBError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("tmdb: %s: status %d: %s", e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("tmdb: %s: %s", e.Path, e.Message)
}

// sanitiseError rebuilds transport errors with the key removed from the URL.
// Go's HTTP client copies the full URL, key included, into every *url.Error,
// so returning or logging such an error as it is would leak the credential.
// The rebuilt error keeps the operation and the underlying cause, with
// api_key redacted (or no URL at all when it cannot be parsed). As a second
// net, any lingering copy of the key in the message is replaced.
func sanitiseError(err error, key string) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		redacted := redactURLString(urlErr.URL, key)
		rebuilt := &url.Error{Op: urlErr.Op, URL: redacted, Err: urlErr.Err}
		if key != "" {
			return errors.New(strings.ReplaceAll(rebuilt.Error(), key, "[REDACTED]"))
		}
		return rebuilt
	}
	if key != "" && strings.Contains(err.Error(), key) {
		return errors.New(strings.ReplaceAll(err.Error(), key, "[REDACTED]"))
	}
	return err
}

func redactURLString(raw, key string) string {
	if raw == "" {
		return "(no url)"
	}
	// The *url.Error URL field is usually the full URL string. Parse it and
	// drop api_key; when parsing fails, fall back to blanking the key text.
	parsed, err := url.Parse(raw)
	if err != nil {
		if key != "" {
			return strings.ReplaceAll(raw, key, "[REDACTED]")
		}
		return "(unparseable url)"
	}
	query := parsed.Query()
	if query.Has("api_key") {
		query.Set("api_key", "[REDACTED]")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	if key != "" && strings.Contains(parsed.String(), key) {
		return strings.ReplaceAll(parsed.String(), key, "[REDACTED]")
	}
	return parsed.String()
}
