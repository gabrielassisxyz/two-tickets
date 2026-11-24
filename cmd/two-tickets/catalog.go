package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/catalog"
	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

const cachePruneInterval = 24 * time.Hour

// newCatalogService builds the one TMDB client the process has, over cache.db,
// and the catalog on top of it.
func newCatalogService(cfg config.Config, cache *store.CacheStore, logger *slog.Logger) *catalog.Service {
	client := tmdb.NewClient(tmdb.Options{APIKey: cfg.TMDBAPIKey, Cache: cache, Logger: logger})
	return catalog.NewService(client, nil)
}

// prunableCache is the part of *store.CacheStore the prune loop uses.
type prunableCache interface {
	PruneAndMaybeRotate(ctx context.Context, nowUnix int64, maxBytes int64) (bool, error)
	Close() error
}

// startCachePruning prunes cache.db now and then every interval, rotating it
// once it stays over maxBytes. The returned function stops the loop, waits for
// it, and only then closes the cache, so a prune never runs against a closed
// file; serve defers it in place of a plain close.
func startCachePruning(ctx context.Context, cache prunableCache, interval time.Duration, maxBytes int64, logger *slog.Logger) func() {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { runCachePruning(ctx, cache, interval, maxBytes, logger) })
	return func() {
		cancel()
		wg.Wait()
		closeAndLog(logger, store.CacheFileName, cache)
	}
}

// runCachePruning logs a failure and retries on the next tick; a failure
// caused by the loop being stopped is not logged.
func runCachePruning(ctx context.Context, cache prunableCache, interval time.Duration, maxBytes int64, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		rotated, err := cache.PruneAndMaybeRotate(ctx, time.Now().Unix(), maxBytes)
		switch {
		case err != nil && ctx.Err() != nil:
			return
		case err != nil:
			logger.WarnContext(ctx, "prune cache", "error", err)
		case rotated:
			logger.InfoContext(ctx, "rotated cache", "max_bytes", maxBytes)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// cacheReadiness is the readiness check for cache.db: a lookup of a key no
// entry has, which is a trivial query on the read pool like Database.Ping.
type cacheReadiness struct {
	cache *store.CacheStore
}

func (c cacheReadiness) Ping(ctx context.Context) error {
	_, _, err := c.cache.Get(ctx, "")
	return err
}
