package tmdb

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// fetchWithRetry runs the limiter, bounded concurrency, timeout and retry
// loop. It returns the body with its status on any HTTP answer, including
// errors the caller decides on, and an error only when no answer should be
// trusted (transport failure, timeout, retry exhaustion, cancellation).
func (c *Client) fetchWithRetry(ctx context.Context, path string, params map[string]string) ([]byte, int, error) {
	deadline := c.clock.Now().Add(c.retryBudget)
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := c.acquireSlot(ctx); err != nil {
			return nil, 0, err
		}
		body, status, retryable, retryAfter, err := c.doAttempt(ctx, path, params)
		<-c.sem
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
			if !retryable {
				return nil, 0, err
			}
			lastErr = err
		} else {
			switch {
			case status == http.StatusOK, status == http.StatusNotFound && isNegativeCacheable(path):
				return body, status, nil
			case status == http.StatusTooManyRequests:
				lastErr = &TMDBError{Status: status, Path: redactPath(path), Message: "rate limited"}
			case status >= 500 && status <= 599:
				lastErr = &TMDBError{Status: status, Path: redactPath(path), Message: "server error"}
			default:
				// Any other 4xx is an answer, not a failure to get one.
				return nil, status, &TMDBError{Status: status, Path: redactPath(path), Message: "request failed"}
			}
		}
		// Only a retryable failure reaches this point. The decisions stay out
		// of the switch above on purpose: a break inside a switch leaves the
		// switch, not the loop, and once turned a 500 into a status-less error.
		if attempt == c.maxAttempts {
			break
		}
		delay := retryAfter
		if delay <= 0 {
			delay = c.backoff(attempt)
		}
		if !c.withinBudget(deadline, delay) {
			break
		}
		if err := c.sleep(ctx, delay); err != nil {
			return nil, 0, err
		}
	}
	c.logger.WarnContext(ctx, "tmdb request failed", "path", redactPath(path), "error", lastErr)
	return nil, 0, lastErr
}

// acquireSlot waits for the global rate limiter, then for a concurrency slot,
// both under the caller's context.
func (c *Client) acquireSlot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.sem <- struct{}{}:
		return nil
	}
}

func (c *Client) withinBudget(deadline time.Time, delay time.Duration) bool {
	return !c.clock.Now().Add(delay).After(deadline)
}

func (c *Client) backoff(attempt int) time.Duration {
	base := defaultBackoffBase << (attempt - 1)
	if base <= 0 {
		base = defaultBackoffBase
	}
	return time.Duration(c.jitter() * float64(base))
}

// doAttempt runs one HTTP try under the per-attempt timeout. The returned
// retryable flag is true for timeouts and transport failures; HTTP statuses
// are decided by the caller.
func (c *Client) doAttempt(ctx context.Context, path string, params map[string]string) (body []byte, status int, retryable bool, retryAfter time.Duration, err error) {
	attemptCtx, cancel := context.WithTimeout(ctx, c.attemptTimeout)
	defer cancel()
	request, err := c.newRequest(attemptCtx, path, params)
	if err != nil {
		return nil, 0, false, 0, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		sanitised := sanitiseError(err, c.apiKey.Reveal())
		if attemptCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
			// The per-attempt timeout fired: a retryable timeout, never cached.
			return nil, 0, true, 0, sanitised
		}
		if ctx.Err() != nil {
			return nil, 0, false, 0, ctx.Err()
		}
		// Transport failures retry; anything proving the request itself is
		// bad does not reach here.
		return nil, 0, isRetryableTransportError(err), 0, sanitised
	}
	defer func() { _ = response.Body.Close() }()
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		sanitised := sanitiseError(err, c.apiKey.Reveal())
		if ctx.Err() != nil {
			return nil, 0, false, 0, ctx.Err()
		}
		return nil, 0, true, 0, sanitised
	}
	if int64(len(content)) > maxResponseBytes {
		return nil, 0, false, 0, &TMDBError{Status: response.StatusCode, Path: redactPath(path), Message: "response too large"}
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return content, response.StatusCode, true, parseRetryAfter(response.Header.Get("Retry-After"), c.clock.Now()), nil
	}
	return content, response.StatusCode, false, 0, nil
}

func isRetryableTransportError(err error) bool {
	if err == nil {
		return false
	}
	// Timeouts retry. Other transport errors (connection reset, no route)
	// retry as well: from the client's seat they look like a slow TMDB.
	return true
}

// parseRetryAfter honours Retry-After on 429: seconds or an HTTP date. An
// unparseable value means no hint, not a failure.
func parseRetryAfter(value string, now time.Time) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC} {
		if date, err := time.Parse(layout, trimmed); err == nil {
			delay := date.Sub(now)
			if delay < 0 {
				return 0
			}
			return delay
		}
	}
	return 0
}
