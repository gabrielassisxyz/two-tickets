// Command two-tickets is the api process: it owns the catalog, login, the
// library and the recommendations, and serves them over HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/httpapi"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

const (
	commandServe = "serve"

	exitFailure = 1
	exitUsage   = 2

	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second

	sessionPruneInterval = 24 * time.Hour
)

// serveDeps is everything serve takes from the outside world, so a test can run
// the real startup sequence with a fake environment or a broken migration set.
type serveDeps struct {
	lookupEnv       config.LookupEnv
	logOutput       io.Writer
	stateMigrations fs.FS
	listen          func(network, address string) (net.Listener, error)
}

func defaultServeDeps() serveDeps {
	return serveDeps{
		lookupEnv:       os.LookupEnv,
		logOutput:       os.Stderr,
		stateMigrations: store.StateMigrations(),
		listen:          net.Listen,
	}
}

func main() {
	os.Exit(dispatchCommand(os.Args[1:], os.Stderr))
}

// dispatchCommand picks the subcommand. With no argument the process serves,
// which is what the container runs.
func dispatchCommand(args []string, stderr io.Writer) int {
	command := commandServe
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case commandServe:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runServe(ctx, defaultServeDeps())
	default:
		_, _ = fmt.Fprintf(stderr, "two-tickets: unknown command %q (available: %s)\n", command, commandServe)
		return exitUsage
	}
}

func runServe(ctx context.Context, deps serveDeps) int {
	logger := slog.New(slog.NewJSONHandler(deps.logOutput, nil))
	if err := serve(ctx, deps, logger); err != nil {
		logger.Error("two-tickets stopped", "error", err)
		return exitFailure
	}
	return 0
}

// serve runs the startup sequence in the order the probes rely on: settings,
// databases, migrations, and only then the listener. A failure at any step
// returns before the port is bound, so nothing ever sees the process ready.
func serve(ctx context.Context, deps serveDeps, logger *slog.Logger) error {
	cfg, err := config.Load(deps.lookupEnv)
	authCfg, authErr := config.LoadAuth(deps.lookupEnv)
	if err := errors.Join(err, authErr); err != nil {
		return err
	}
	logger.Info("configuration loaded", "config", cfg, "auth", authCfg)
	if err := requireDirectory(cfg.DataDir); err != nil {
		return err
	}

	state, err := store.OpenDatabase(ctx, filepath.Join(cfg.DataDir, store.StateFileName))
	if err != nil {
		return fmt.Errorf("open %s: %w", store.StateFileName, err)
	}
	defer closeAndLog(logger, store.StateFileName, state)
	cache, err := store.OpenCacheStore(ctx, filepath.Join(cfg.DataDir, store.CacheFileName))
	if err != nil {
		return fmt.Errorf("open %s: %w", store.CacheFileName, err)
	}
	stopCachePruning := startCachePruning(ctx, cache, cachePruneInterval, cfg.CacheMaxBytes, logger)
	defer stopCachePruning()

	migrations, err := store.LoadMigrations(deps.stateMigrations)
	if err != nil {
		return err
	}
	if err := store.MigrateState(ctx, state, migrations, filepath.Join(cfg.DataDir, store.RollbackDirName)); err != nil {
		return fmt.Errorf("migrate %s: %w", store.StateFileName, err)
	}

	authService, err := auth.NewService(ctx, state, authCfg.Accounts, nil)
	if err != nil {
		return fmt.Errorf("accounts: %w", err)
	}
	// After the accounts: each watchlist references its owner's users row.
	if err := store.BootstrapLibraryLists(ctx, state, time.Now()); err != nil {
		return fmt.Errorf("library lists: %w", err)
	}
	stopPruning := startSessionPruning(ctx, authService, logger)
	defer stopPruning()

	listener, err := deps.listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	client := newTMDBClient(cfg, cache, logger)
	catalogService := newCatalogService(client)
	router := httpapi.NewRouter(httpapi.RouterDeps{
		Logger:         logger,
		Readiness:      []httpapi.ReadinessCheck{state, cacheReadiness{cache: cache}},
		Auth:           authService,
		AllowedOrigins: authCfg.AllowedOrigins,
		TrustedProxies: authCfg.TrustedProxies,
		Catalog:        catalogService,
		Library:        newLibraryService(state, authService, client, catalogService, logger),
	})
	return serveUntilDone(ctx, listener, router, logger)
}

// startSessionPruning runs the daily prune in the background. The returned
// function stops it and waits, so it never runs against a closed database.
func startSessionPruning(ctx context.Context, service *auth.Service, logger *slog.Logger) func() {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { service.RunSessionPruning(ctx, sessionPruneInterval, logger) })
	return func() {
		cancel()
		wg.Wait()
	}
}

func serveUntilDone(ctx context.Context, listener net.Listener, handler http.Handler, logger *slog.Logger) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	logger.Info("listening", "addr", listener.Addr().String())

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("stopped")
	return nil
}

func requireDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %w", config.EnvDataDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %s is not a directory", config.EnvDataDir, path)
	}
	return nil
}

func closeAndLog(logger *slog.Logger, name string, db io.Closer) {
	if err := db.Close(); err != nil {
		logger.Error("close database", "file", name, "error", err)
	}
}
