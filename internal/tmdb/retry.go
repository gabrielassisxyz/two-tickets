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
	start := c.clock.Now()
	deadline := start.Add(c.retryBudget)
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case c.sem <- struct{}{}:
		}
		body, status, retryable, retryAfter, err := c.doAttempt(ctx, path, params)
		<-c.sem
		if err == nil {
			switch {
			case status == http.StatusOK:
				return body, status, nil
			case status == http.StatusNotFound && isNegativeCacheable(path):
				return body, status, nil
			case status == http.StatusTooManyRequests:
				lastErr = &TMDBError{Status: status, Path: redactPath(path), Message: "rate limited"}
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
				if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
					return nil, 0, sleepErr
				}
				continue
			case status >= 500 && status <= 599:
				lastErr = &TMDBError{Status: status, Path: redactPath(path), Message: "server error"}
				if attempt == c.maxAttempts {
					break
				}
				delay := c.backoff(attempt)
				if !c.withinBudget(deadline, delay) {
					break
				}
				if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
					return nil, 0, sleepErr
				}
				continue
			default:
				return nil, status, &TMDBError{Status: status, Path: redactPath(path), Message: "request failed"}
			}
		}
		// Transport or timeout failure: only these, 429 and 5xx are retried.
		// Any other 4xx above returned already.
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		lastErr = err
		if !retryable {
			return nil, 0, err
		}
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
		if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
			return nil, 0, sleepErr
		}
	}
	if lastErr == nil {
		lastErr = &TMDBError{Path: redactPath(path), Message: "retry budget exhausted"}
	}
	c.logger.WarnContext(ctx, "tmdb request failed", "path", redactPath(path), "error", lastErr)
	return nil, 0, lastErr
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
