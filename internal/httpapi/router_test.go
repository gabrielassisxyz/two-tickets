package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gabrielassisxyz/two-tickets/internal/respond"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

type recordedResponse struct {
	status int
	header http.Header
	body   []byte
}

func request(t *testing.T, server *httptest.Server, method, path string) recordedResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return recordedResponse{status: resp.StatusCode, header: resp.Header, body: body}
}

// startRouter serves the router on an ephemeral port, the way the binary does.
func startRouter(t *testing.T, checks ...ReadinessCheck) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(NewRouter(RouterDeps{Readiness: checks}))
	t.Cleanup(server.Close)
	return server
}

func openDatabase(t *testing.T) (*store.Database, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), store.StateFileName)
	db, err := store.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func TestHealthRoutesAnswer200WithAnEmptyBodyWhenHealthy(t *testing.T) {
	db, _ := openDatabase(t)
	server := startRouter(t, db)
	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			resp := request(t, server, method, path)
			if resp.status != http.StatusOK {
				t.Errorf("%s %s = %d, want 200", method, path, resp.status)
			}
			if len(resp.body) != 0 {
				t.Errorf("%s %s body = %q, want empty", method, path, resp.body)
			}
		}
	}
}

func TestReadyzAnswers503WhenTheDatabaseCannotAnswer(t *testing.T) {
	db, _ := openDatabase(t)
	server := startRouter(t, db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	resp := request(t, server, http.MethodGet, "/readyz")
	if resp.status != http.StatusServiceUnavailable {
		t.Fatalf("/readyz with a closed database = %d, want 503", resp.status)
	}
	assertErrorShape(t, resp)

	// Liveness does not depend on the database.
	if got := request(t, server, http.MethodGet, "/healthz").status; got != http.StatusOK {
		t.Errorf("/healthz with a closed database = %d, want 200", got)
	}
}

type leakyFailingCheck struct{ detail string }

func (c leakyFailingCheck) Ping(context.Context) error { return errors.New(c.detail) }

// The failing check's error carries a file path and a dependency version, the
// two things a probe answer must never repeat.
func TestHealthResponsesCarryNoPathVersionOrConfiguration(t *testing.T) {
	db, dbPath := openDatabase(t)
	detail := fmt.Sprintf("open %s: sqlite 3.51.0 (modernc.org/sqlite v1.63.0) listen 127.0.0.1:8080", dbPath)
	forbidden := []string{dbPath, filepath.Dir(dbPath), "sqlite", "modernc", runtime.Version(), "go1.", "127.0.0.1:8080"}

	healthy := startRouter(t, db)
	failing := startRouter(t, db, leakyFailingCheck{detail: detail})
	var responses []recordedResponse
	for _, server := range []*httptest.Server{healthy, failing} {
		for _, path := range []string{"/healthz", "/readyz", "/startupz", "/nope"} {
			responses = append(responses, request(t, server, http.MethodGet, path))
		}
	}
	if responses[5].status != http.StatusServiceUnavailable {
		t.Fatalf("/readyz with a failing check = %d, want 503", responses[5].status)
	}
	for _, resp := range responses {
		var text bytes.Buffer
		_ = resp.header.Write(&text)
		text.Write(resp.body)
		for _, word := range forbidden {
			if strings.Contains(text.String(), word) {
				t.Errorf("response %d leaks %q:\n%s", resp.status, word, text.String())
			}
		}
	}
}

func assertErrorShape(t *testing.T, resp recordedResponse) {
	t.Helper()
	if ct := resp.header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("status %d: Content-Type = %q, want application/json", resp.status, ct)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.body, &raw); err != nil {
		t.Fatalf("status %d: body %q is not a JSON object: %v", resp.status, resp.body, err)
	}
	if len(raw) != 2 {
		t.Errorf("status %d: body %s has keys other than message and errors", resp.status, resp.body)
	}
	var message string
	if err := json.Unmarshal(raw["message"], &message); err != nil || message == "" {
		t.Errorf("status %d: message = %s, want a non-empty string", resp.status, raw["message"])
	}
	if message != http.StatusText(resp.status) {
		t.Errorf("status %d: message %q does not match the status", resp.status, message)
	}
	var details []any
	if err := json.Unmarshal(raw["errors"], &details); err != nil || details == nil {
		t.Errorf("status %d: errors = %s, want an array", resp.status, raw["errors"])
	}
}

func TestErrorResponsesUseTheErrorShape(t *testing.T) {
	db, _ := openDatabase(t)
	server := startRouter(t, db)
	cases := []struct {
		method, path string
		want         int
	}{
		// Without a session every route but the public ones answers 401;
		// the 404 behind the gate is covered in auth_routes_test.go.
		{http.MethodGet, "/no-such-route", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/backdrops", http.StatusNotFound},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/readyz", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		resp := request(t, server, tc.method, tc.path)
		if resp.status != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, resp.status, tc.want)
			continue
		}
		assertErrorShape(t, resp)
	}
}

func TestWriteErrorCarriesDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), "page must be a number")
	resp := recordedResponse{status: rec.Code, header: rec.Header(), body: rec.Body.Bytes()}
	assertErrorShape(t, resp)
	var body ErrorBody
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0] != "page must be a number" {
		t.Fatalf("errors = %v", body.Errors)
	}
}

// A handler outside httpapi answers errors through respond; the client must
// not be able to tell which of the two wrote it.
func TestRespondErrorIsByteIdenticalToWriteError(t *testing.T) {
	for _, details := range [][]string{nil, {"page must be a number", "query is required"}} {
		viaHTTPAPI := httptest.NewRecorder()
		WriteError(viaHTTPAPI, http.StatusBadRequest, "Bad Request", details...)
		viaRespond := httptest.NewRecorder()
		respond.Error(viaRespond, http.StatusBadRequest, "Bad Request", details...)

		if viaHTTPAPI.Code != viaRespond.Code {
			t.Errorf("details %v: status %d vs %d", details, viaHTTPAPI.Code, viaRespond.Code)
		}
		var left, right bytes.Buffer
		_ = viaHTTPAPI.Header().Write(&left)
		_ = viaRespond.Header().Write(&right)
		if left.String() != right.String() {
			t.Errorf("details %v: headers differ:\n%s\nvs\n%s", details, left.String(), right.String())
		}
		if !bytes.Equal(viaHTTPAPI.Body.Bytes(), viaRespond.Body.Bytes()) {
			t.Errorf("details %v: body %q vs %q", details, viaHTTPAPI.Body.Bytes(), viaRespond.Body.Bytes())
		}
	}
}

func TestPanickingHandlerAnswers500WithoutThePanicValue(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret internal detail")
	}), logger)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp := request(t, server, http.MethodGet, "/")
	if resp.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.status)
	}
	assertErrorShape(t, resp)
	if bytes.Contains(resp.body, []byte("secret internal detail")) {
		t.Fatal("the panic value reached the client")
	}
	if !strings.Contains(logs.String(), "secret internal detail") {
		t.Fatal("the panic value was not logged")
	}
}
