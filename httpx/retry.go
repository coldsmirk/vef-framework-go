package httpx

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/coldsmirk/go-collections"
)

const (
	// defaultRetryMaxAttempts is the total attempt count, the first call
	// included, when RetryConfig.MaxAttempts is zero.
	defaultRetryMaxAttempts = 3

	// defaultRetryInitialBackoff is the base retry delay when
	// RetryConfig.InitialBackoff is zero.
	defaultRetryInitialBackoff = 100 * time.Millisecond

	// defaultRetryMaxBackoff caps the retry delay when RetryConfig.MaxBackoff
	// is zero.
	defaultRetryMaxBackoff = 2 * time.Second
)

// retryableStatuses are the response codes the default retry policy acts on:
// the server explicitly signals throttling or transient unavailability.
var retryableStatuses = collections.NewHashSetFrom(
	http.StatusTooManyRequests,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
)

// idempotentMethods are the HTTP methods the default retry policy considers
// safe to repeat.
var idempotentMethods = collections.NewHashSetFrom(
	http.MethodGet,
	http.MethodHead,
	http.MethodPut,
	http.MethodDelete,
	http.MethodOptions,
	http.MethodTrace,
)

// RetryConfig configures automatic retries, enabled via WithRetry. Zero
// fields fall back to the documented defaults.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts, the first call included.
	// Default 3.
	MaxAttempts int

	// InitialBackoff is the base delay before the first retry; every further
	// retry doubles it, with full jitter applied. Default 100ms.
	InitialBackoff time.Duration

	// MaxBackoff caps the delay between attempts, a server-sent Retry-After
	// included. Default 2s.
	MaxBackoff time.Duration

	// RetryIf decides whether a failed attempt is retried, replacing the
	// default policy entirely. The default retries a transport error or a
	// 429/502/503/504 response, and only for idempotent methods — a POST is
	// never retried unless RetryIf allows it. Exactly one of resp and err is
	// non-nil.
	RetryIf func(resp *Response, err error) bool
}

// withDefaults returns a copy with zero fields resolved to the defaults.
func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultRetryMaxAttempts
	}

	if c.InitialBackoff <= 0 {
		c.InitialBackoff = defaultRetryInitialBackoff
	}

	if c.MaxBackoff <= 0 {
		c.MaxBackoff = defaultRetryMaxBackoff
	}

	return c
}

// shouldRetry decides whether the outcome of one attempt warrants another.
func (c *RetryConfig) shouldRetry(method string, resp *Response, err error) bool {
	if c.RetryIf != nil {
		return c.RetryIf(resp, err)
	}

	if !idempotentMethods.Contains(method) {
		return false
	}

	if err != nil {
		return retryableError(err)
	}

	return retryableStatuses.Contains(resp.StatusCode())
}

// retryableError reports whether err looks transient: context expiry and the
// deterministic response-size failure are final; everything else — dial
// failures, resets, mid-body drops — is worth another attempt.
func retryableError(err error) bool {
	return !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) &&
		!errors.Is(err, ErrResponseTooLarge)
}

// backoffDelay computes the pause before the retry-th retry (1-based): a
// server-sent Retry-After wins, otherwise exponential backoff with full
// jitter; both are capped at MaxBackoff.
func (c *RetryConfig) backoffDelay(retry int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, c.MaxBackoff)
	}

	backoff := c.MaxBackoff

	if shift := retry - 1; shift < 63 {
		if d := c.InitialBackoff << shift; d > 0 && d < c.MaxBackoff {
			backoff = d
		}
	}

	if backoff <= 0 {
		return 0
	}

	return rand.N(backoff)
}

// retryAfterHint extracts the server's Retry-After delay, given in either
// delta-seconds or HTTP-date form; zero when absent or malformed.
func retryAfterHint(resp *Response) time.Duration {
	value := resp.Header("Retry-After")
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}

	if at, err := http.ParseTime(value); err == nil {
		return max(time.Until(at), 0)
	}

	return 0
}

// sleepBackoff waits d or until ctx is done, whichever comes first.
func sleepBackoff(ctx context.Context, d time.Duration) error {
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
