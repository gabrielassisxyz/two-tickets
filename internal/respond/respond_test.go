package respond

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorAlwaysCarriesAnErrorsArray(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusNotFound, "Not Found")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got, want := rec.Body.String(), `{"message":"Not Found","errors":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestJSONIsNeverCached(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, map[string]string{"status": "ok"})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got, want := rec.Body.String(), `{"status":"ok"}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
