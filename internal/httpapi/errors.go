// Package httpapi is the HTTP surface of the api process.
package httpapi

import (
	"encoding/json"
	"net/http"
)

// ErrorBody is the shape of every error response: the same {message, errors}
// object Seerr's own server returns, so the lifted frontend reads it unchanged.
type ErrorBody struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

// WriteError answers with status and the error shape. details are optional;
// errors is always an array, never null, so a client can iterate it blindly.
func WriteError(w http.ResponseWriter, status int, message string, details ...string) {
	if details == nil {
		details = []string{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Message: message, Errors: details})
}

func writeStatusError(w http.ResponseWriter, status int) {
	WriteError(w, status, http.StatusText(status))
}
