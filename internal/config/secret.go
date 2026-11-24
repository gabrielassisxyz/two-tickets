package config

import (
	"fmt"
	"io"
	"log/slog"
)

const redactedText = "[REDACTED]"

// Secret holds a credential that must never reach a log line or a response.
// The value sits in an unexported field and every formatting path prints a
// placeholder, so dumping a Config with fmt, slog or encoding/json is safe.
// Reveal is the single way out, and it is meant to be called only where the
// credential is sent to the service that expects it.
type Secret struct {
	value string
}

// NewSecret wraps a credential.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// Reveal returns the credential itself.
func (s Secret) Reveal() string {
	return s.value
}

// IsZero reports whether no credential is held.
func (s Secret) IsZero() bool {
	return s.value == ""
}

func (s Secret) String() string {
	return redactedText
}

func (s Secret) GoString() string {
	return redactedText
}

// Format covers every fmt verb, including the ones that bypass String.
func (s Secret) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, redactedText)
}

func (s Secret) LogValue() slog.Value {
	return slog.StringValue(redactedText)
}

// MarshalText also covers encoding/json, which falls back to it.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(redactedText), nil
}
