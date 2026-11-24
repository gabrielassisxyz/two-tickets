package main

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

// Short bounds so a stalled connection is cut in milliseconds; the outer
// wait is long enough that only a missing bound can exceed it.
var testServerTimeouts = serverTimeouts{
	readHeader: time.Second,
	read:       200 * time.Millisecond,
	write:      200 * time.Millisecond,
	idle:       time.Second,
}

const boundedWait = 3 * time.Second

func startTestAPIServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := newAPIServer(handler, testServerTimeouts, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	return listener.Addr().String()
}

func TestAPIServerGivesUpOnABodyThatStalls(t *testing.T) {
	readErr := make(chan error, 1)
	addr := startTestAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr <- err
	}))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	// Promise ten body bytes, send two, then go quiet.
	if _, err := io.WriteString(conn, "PUT /api/v1/library/rating HTTP/1.1\r\nHost: api\r\nContent-Length: 10\r\n\r\n{\""); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-readErr:
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("reading the stalled body ended with %v, want a timeout", err)
		}
	case <-time.After(boundedWait):
		t.Fatalf("the handler was still reading a stalled body after %s", boundedWait)
	}
}

func TestAPIServerCutsAResponseThatOverrunsTheWriteBound(t *testing.T) {
	addr := startTestAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * testServerTimeouts.write)
		_, _ = io.WriteString(w, "too late")
	}))

	client := &http.Client{Timeout: boundedWait}
	resp, err := client.Get("http://" + addr + "/api/v1/recommendations")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatalf("a response written after the write bound arrived with status %d, want the connection cut", resp.StatusCode)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("the client waited out its own %s instead of seeing the connection cut: %v", boundedWait, err)
	}
}

func TestProductionServerTimeoutsBoundEveryPhase(t *testing.T) {
	bounds := map[string]time.Duration{
		"read header": productionServerTimeouts.readHeader,
		"read":        productionServerTimeouts.read,
		"write":       productionServerTimeouts.write,
		"idle":        productionServerTimeouts.idle,
	}
	for name, bound := range bounds {
		if bound <= 0 {
			t.Errorf("the %s timeout is %s; zero or less means no bound", name, bound)
		}
	}
	if productionServerTimeouts.read < productionServerTimeouts.readHeader {
		t.Errorf("read %s ends before read header %s; the read bound covers the headers too",
			productionServerTimeouts.read, productionServerTimeouts.readHeader)
	}
}
