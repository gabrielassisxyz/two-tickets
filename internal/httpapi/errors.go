// Package httpapi is the HTTP surface of the api process.
package httpapi

import (
	"net/http"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
)

// ErrorBody is the shape of every error response; respond owns it.
type ErrorBody = respond.ErrorBody

// WriteError answers with status and the error shape. details are optional;
// errors is always an array, never null, so a client can iterate it blindly.
func WriteError(w http.ResponseWriter, status int, message string, details ...string) {
	respond.Error(w, status, message, details...)
}

func writeStatusError(w http.ResponseWriter, status int) {
	WriteError(w, status, http.StatusText(status))
}
