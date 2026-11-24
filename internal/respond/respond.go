// Package respond writes the api's JSON responses. It is the only place that
// knows the error shape and the JSON headers, and it imports nothing of the
// api's own, so any handler package can answer without importing httpapi.
package respond

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

// Error answers with status and the error shape. details are optional;
// errors is always an array, never null, so a client can iterate it blindly.
func Error(w http.ResponseWriter, status int, message string, details ...string) {
	if details == nil {
		details = []string{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Message: message, Errors: details})
}

// JSON answers 200 with body encoded as JSON. Responses carry account data,
// so no cache may keep them.
func JSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}
