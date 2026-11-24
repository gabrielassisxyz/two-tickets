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
	"syscall"
	"time"

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
	if err != nil {
		return err
	}
	logger.Info("configuration loaded", "config", cfg)
	if err := requireDirectory(cfg.DataDir); err != nil {
		return err
	}

	state, err := store.OpenDatabase(ctx, filepath.Join(cfg.DataDir, store.StateFileName))
	if err != nil {
		return fmt.Errorf("open %s: %w", store.StateFileName, err)
	}
	defer closeAndLog(logger, store.StateFileName, state)
	cache, err := store.OpenDatabase(ctx, filepath.Join(cfg.DataDir, store.CacheFileName))
	if err != nil {
		return fmt.Errorf("open %s: %w", store.CacheFileName, err)
	}
	defer closeAndLog(logger, store.CacheFileName, cache)

	migrations, err := store.LoadMigrations(deps.stateMigrations)
	if err != nil {
		return err
	}
	if err := store.MigrateState(ctx, state, migrations, filepath.Join(cfg.DataDir, store.RollbackDirName)); err != nil {
		return fmt.Errorf("migrate %s: %w", store.StateFileName, err)
	}

	listener, err := deps.listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	router := httpapi.NewRouter(httpapi.RouterDeps{Logger: logger, Readiness: []httpapi.ReadinessCheck{state, cache}})
	return serveUntilDone(ctx, listener, router, logger)
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

func closeAndLog(logger *slog.Logger, name string, db *store.Database) {
	if err := db.Close(); err != nil {
		logger.Error("close database", "file", name, "error", err)
	}
}
