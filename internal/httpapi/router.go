package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/library"
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
	// Auth resolves sessions. Every route but the health routes and the
	// public surface listed in isPublicRoute requires one.
	Auth *auth.Service
	// AllowedOrigins are the origins a mutation must come from.
	AllowedOrigins []string
	// TrustedProxies are the networks whose forwarding headers are read.
	TrustedProxies []netip.Prefix
	// Catalog serves the catalog routes, behind the session gate and the
	// origin check. Nil leaves them out.
	Catalog *catalog.Service
	// Library serves the library routes and the Seerr watchlist routes,
	// behind the same two checks. Nil leaves them out.
	Library *library.Service
}

// routeRegistrar is what the router needs from a route family;
// *catalog.Service and *library.Service satisfy it, and a test registers a
// stub through it.
type routeRegistrar interface {
	Register(mux *http.ServeMux)
}

const readinessTimeout = 2 * time.Second

// NewRouter returns the api's HTTP handler. Health routes are unauthenticated
// and answer with a status code only: a probe is reachable by anything that
// can reach the port, so it never describes the configuration or the
// dependencies behind it.
func NewRouter(deps RouterDeps) http.Handler {
	// A nil pointer stored in the interface would not compare equal to nil,
	// so a family is only converted when it is set.
	var families []routeRegistrar
	if deps.Catalog != nil {
		families = append(families, deps.Catalog)
	}
	if deps.Library != nil {
		families = append(families, deps.Library)
	}
	return newRouter(deps, families...)
}

func newRouter(deps RouterDeps, families ...routeRegistrar) http.Handler {
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

	routes := authRoutes{service: deps.Auth, trustedProxies: deps.TrustedProxies, logger: logger}
	api := http.NewServeMux()
	routes.register(api)
	registerSettingsRoutes(api)
	for _, family := range families {
		family.Register(api)
	}
	api.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeStatusError(w, http.StatusNotFound)
	})
	// Everything that is not a health route goes through both checks, so a
	// route added later is protected without anyone remembering to ask.
	mux.Handle("/", requireAllowedOrigin(deps.AllowedOrigins, routes.requireSession(api)))
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
