package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ReadinessCheck answers a trivial query against one dependency.
// *store.Database satisfies it.
type ReadinessCheck interface {
	Ping(ctx context.Context) error
}

// RouterDeps is what the router needs from the rest of the process.
type RouterDeps struct {
	Logger *slog.Logger
	// Readiness is checked in order on every /readyz; all must answer.
	Readiness []ReadinessCheck
}

const readinessTimeout = 2 * time.Second

// NewRouter returns the api's HTTP handler. Health routes are unauthenticated
// and answer with a status code only: a probe is reachable by anything that
// can reach the port, so it never describes the configuration or the
// dependencies behind it.
func NewRouter(deps RouterDeps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	mux := http.NewServeMux()
	// Liveness: the process is up and serving. Nothing else is checked, so a
	// slow database never gets the process restarted.
	mux.Handle("/healthz", allowRead(http.HandlerFunc(answerOK)))
	// Startup: the process only starts listening once migrations have
	// finished, so being able to answer at all is the signal.
	mux.Handle("/startupz", allowRead(http.HandlerFunc(answerOK)))
	mux.Handle("/readyz", allowRead(readinessHandler(deps.Readiness, logger)))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeStatusError(w, http.StatusNotFound)
	})
	return recoverPanics(mux, logger)
}

func answerOK(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// readinessHandler runs a trivial query on each dependency, never an integrity
// scan. The failure reason goes to the log, not to the probe.
func readinessHandler(checks []ReadinessCheck, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		for _, check := range checks {
			if err := check.Ping(ctx); err != nil {
				logger.WarnContext(ctx, "readiness check failed", "error", err)
				writeStatusError(w, http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

// allowRead answers 405 in the error shape for anything but GET and HEAD.
// ServeMux's own method patterns would answer 405 in plain text instead.
func allowRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodHead}, ", "))
			writeStatusError(w, http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverPanics turns a panicking handler into a 500 in the error shape. The
// panic value is logged and never sent to the client.
func recoverPanics(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			logger.ErrorContext(r.Context(), "handler panicked", "panic", recovered, "path", r.URL.Path)
			writeStatusError(w, http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
