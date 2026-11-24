// Package tmdb provides the process-wide TMDB client: one rate limiter,
// bounded concurrency, timeouts, retries, singleflight and a disposable SQLite
// response cache. The generic Get carries the limiter, retry and cache; the
// named methods in methods.go are thin wrappers over it.
package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

const (
	// DefaultBaseURL is TMDB's v3 API root.
	DefaultBaseURL = "https://api.themoviedb.org/3"
	// DefaultLanguage and DefaultRegion shape every response unless a caller
	// overrides them per request.
	DefaultLanguage = "pt-BR"
	DefaultRegion   = "BR"

	defaultMaxAttempts          = 3
	defaultRetryBudget          = 30 * time.Second
	defaultAttemptTimeout       = 20 * time.Second
	defaultConnectTimeout       = 5 * time.Second
	defaultHeaderTimeout        = 10 * time.Second
	defaultMaxConcurrency       = 10
	defaultNegativeTTL          = 7 * 24 * time.Hour
	defaultBackoffBase          = 500 * time.Millisecond
	maxResponseBytes      int64 = 10 << 20
)

// Clock reports the current time. Tests inject a fake to advance expiry
// without waiting.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// responseCache is the subset of *store.CacheStore the client uses, so tests
// can substitute a fake. *store.CacheStore satisfies it.
type responseCache interface {
	Get(ctx context.Context, key string) (store.CacheEntry, bool, error)
	Set(ctx context.Context, key string, status int, body []byte, expiresAt int64) error
}

// Options customises the client. Production uses the bead's numbers; tests run
// against httptest servers with reduced limits and stubbed sleeps.
type Options struct {
	BaseURL string
	APIKey  config.Secret
	// HTTPClient carries the transport timeouts. When nil, a default client
	// with connect 5 s and response-header 10 s timeouts is built.
	HTTPClient *http.Client
	// Limiter is the global 20 requests per second gate. The settled design
	// is rate.NewLimiter(rate.Every(50*time.Millisecond), 1): a burst of 20
	// would let 40 starts land inside one second.
	Limiter *rate.Limiter
	Clock   Clock
	// Sleep waits between retries. It must respect ctx: the caller's
	// cancellation ends a wait in progress at once. Defaults to a timer.
	Sleep func(ctx context.Context, d time.Duration) error
	// Jitter returns a float in [0,1) for full jitter between retries.
	// Defaults to rand.Float64.
	Jitter func() float64
	// MaxAttempts caps tries; RetryBudget caps the whole loop.
	MaxAttempts int
	RetryBudget time.Duration
	// AttemptTimeout bounds one try (total 20 s in production).
	AttemptTimeout time.Duration
	// MaxConcurrency bounds concurrent upstream requests.
	MaxConcurrency int
	Cache          responseCache
	FormatVersion  int
	Logger         *slog.Logger
	Language       string
	Region         string
}

// Client is one process-wide TMDB client.
type Client struct {
	baseURL        string
	apiKey         config.Secret
	http           *http.Client
	limiter        *rate.Limiter
	clock          Clock
	sleep          func(ctx context.Context, d time.Duration) error
	jitter         func() float64
	maxAttempts    int
	retryBudget    time.Duration
	attemptTimeout time.Duration
	sem            chan struct{}
	cache          responseCache
	formatVersion  int
	logger         *slog.Logger
	language       string
	region         string
	group          singleflight.Group
}

// NewClient builds a client, filling every zero Option with the production
// default from the bead.
func NewClient(opts Options) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: defaultConnectTimeout}).DialContext,
				ResponseHeaderTimeout: defaultHeaderTimeout,
			},
		}
	}
	limiter := opts.Limiter
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Every(50*time.Millisecond), 1)
	}
	clock := opts.Clock
	if clock == nil {
		clock = systemClock{}
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	jitter := opts.Jitter
	if jitter == nil {
		jitter = rand.Float64
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	retryBudget := opts.RetryBudget
	if retryBudget <= 0 {
		retryBudget = defaultRetryBudget
	}
	attemptTimeout := opts.AttemptTimeout
	if attemptTimeout <= 0 {
		attemptTimeout = defaultAttemptTimeout
	}
	maxConcurrency := opts.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = defaultMaxConcurrency
	}
	formatVersion := opts.FormatVersion
	if formatVersion <= 0 {
		formatVersion = CacheFormatVersion
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	language := opts.Language
	if language == "" {
		language = DefaultLanguage
	}
	region := opts.Region
	if region == "" {
		region = DefaultRegion
	}
	return &Client{
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		apiKey:         opts.APIKey,
		http:           httpClient,
		limiter:        limiter,
		clock:          clock,
		sleep:          sleep,
		jitter:         jitter,
		maxAttempts:    maxAttempts,
		retryBudget:    retryBudget,
		attemptTimeout: attemptTimeout,
		sem:            make(chan struct{}, maxConcurrency),
		cache:          opts.Cache,
		formatVersion:  formatVersion,
		logger:         logger,
		language:       language,
		region:         region,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Get fetches path with params, honouring the limiter, retry budget and cache,
// and decodes the JSON body into dest. Language and region default to pt-BR
// and BR; include_adult defaults to false so adult content stays excluded.
func (c *Client) Get(ctx context.Context, path string, params map[string]string, dest any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	merged := make(map[string]string, len(params)+3)
	for k, v := range params {
		merged[k] = v
	}
	if _, ok := merged["language"]; !ok {
		merged["language"] = c.language
	}
	if _, ok := merged["region"]; !ok {
		merged["region"] = c.region
	}
	if _, ok := merged["include_adult"]; !ok {
		merged["include_adult"] = "false"
	}
	key := BuildCacheKey(c.formatVersion, http.MethodGet, path, merged)
	ttl := TTLForPath(path)

	// Singleflight wraps the whole cache-miss-then-fetch path, keyed by the
	// cache key, so 100 concurrent identical cold requests make 1 upstream
	// request. The shared value is the raw body; each caller decodes into its
	// own dest.
	shared, err, _ := c.group.Do(key, func() (any, error) {
		return c.fetchUnshared(ctx, path, merged, key, ttl)
	})
	if err != nil {
		return err
	}
	body, ok := shared.([]byte)
	if !ok {
		return fmt.Errorf("tmdb: unexpected shared type %T", shared)
	}
	if dest == nil {
		return nil
	}
	if len(body) == 0 {
		return fmt.Errorf("tmdb: empty response for %s", redactPath(path))
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("tmdb: decode %s: %w", redactPath(path), err)
	}
	return nil
}

func (c *Client) fetchUnshared(ctx context.Context, path string, params map[string]string, key string, ttl time.Duration) ([]byte, error) {
	now := c.clock.Now()
	if c.cache != nil {
		entry, ok, err := c.cache.Get(ctx, key)
		if err != nil {
			// A broken cache must not break the catalog: fetch live.
			c.logger.WarnContext(ctx, "tmdb cache get failed", "path", redactPath(path), "error", err)
		} else if ok && entry.ExpiresAt > now.Unix() {
			switch entry.Status {
			case http.StatusOK, http.StatusNotFound:
				return entry.Body, nil
			default:
				// Only 200 and the intentional 404 negative are ever
				// stored; anything else here is from an older build.
			}
		}
	}
	body, status, err := c.fetchWithRetry(ctx, path, params)
	if err != nil {
		return nil, err
	}
	switch status {
	case http.StatusOK:
		if c.cache != nil {
			expiresAt := now.Add(ttl).Unix()
			if err := c.cache.Set(ctx, key, status, body, expiresAt); err != nil {
				c.logger.WarnContext(ctx, "tmdb cache set failed", "path", redactPath(path), "error", err)
			}
		}
		return body, nil
	case http.StatusNotFound:
		if !isNegativeCacheable(path) {
			return nil, &TMDBError{Status: status, Path: redactPath(path), Message: "not found"}
		}
		negative := []byte("null")
		if c.cache != nil {
			expiresAt := now.Add(negativeTTLFor(ttl)).Unix()
			if err := c.cache.Set(ctx, key, status, negative, expiresAt); err != nil {
				c.logger.WarnContext(ctx, "tmdb cache set failed", "path", redactPath(path), "error", err)
			}
		}
		return negative, nil
	default:
		return nil, &TMDBError{Status: status, Path: redactPath(path), Message: "unexpected status"}
	}
}

func negativeTTLFor(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultNegativeTTL
	}
	return ttl
}

func (c *Client) newRequest(ctx context.Context, path string, params map[string]string) (*http.Request, error) {
	raw := c.baseURL + path
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("tmdb: bad path %s: %w", redactPath(path), err)
	}
	query := parsed.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	// The v3 key travels in api_key. It is added here, at the last moment,
	// and never stored in the cache key or a log line.
	if key := c.apiKey.Reveal(); key != "" {
		query.Set("api_key", key)
	}
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("tmdb: build request for %s: %w", redactPath(path), err)
	}
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func redactPath(path string) string {
	if path == "" {
		return "(empty path)"
	}
	return path
}
