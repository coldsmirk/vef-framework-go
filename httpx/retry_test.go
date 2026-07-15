package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRetryServer starts an httptest server closed on test cleanup.
func newRetryServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// newRetryClient builds a client with a fast retry policy layered under opts.
func newRetryClient(t *testing.T, cfg RetryConfig) *Client {
	t.Helper()

	if cfg.InitialBackoff == 0 {
		cfg.InitialBackoff = time.Millisecond
	}

	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = 2 * time.Millisecond
	}

	client, err := New(WithRetry(cfg))
	require.NoError(t, err, "New should succeed")

	return client
}

// TestRetryConfigWithDefaults tests zero-field resolution.
func TestRetryConfigWithDefaults(t *testing.T) {
	t.Run("ZeroFieldsResolve", func(t *testing.T) {
		cfg := RetryConfig{}.withDefaults()

		assert.Equal(t, defaultRetryMaxAttempts, cfg.MaxAttempts, "MaxAttempts should default")
		assert.Equal(t, defaultRetryInitialBackoff, cfg.InitialBackoff, "InitialBackoff should default")
		assert.Equal(t, defaultRetryMaxBackoff, cfg.MaxBackoff, "MaxBackoff should default")
	})

	t.Run("ExplicitFieldsSurvive", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 5, InitialBackoff: time.Second, MaxBackoff: 10 * time.Second}.withDefaults()

		assert.Equal(t, 5, cfg.MaxAttempts, "An explicit MaxAttempts should survive")
		assert.Equal(t, time.Second, cfg.InitialBackoff, "An explicit InitialBackoff should survive")
		assert.Equal(t, 10*time.Second, cfg.MaxBackoff, "An explicit MaxBackoff should survive")
	})
}

// TestShouldRetry tests the default policy and its RetryIf replacement.
func TestShouldRetry(t *testing.T) {
	policy := RetryConfig{}.withDefaults()

	tests := []struct {
		name   string
		method string
		resp   *Response
		err    error
		want   bool
	}{
		{"RetryableStatusOnIdempotent", http.MethodGet, &Response{statusCode: http.StatusServiceUnavailable}, nil, true},
		{"SuccessNotRetried", http.MethodGet, &Response{statusCode: http.StatusOK}, nil, false},
		{"ClientErrorNotRetried", http.MethodGet, &Response{statusCode: http.StatusBadRequest}, nil, false},
		{"PostNotRetried", http.MethodPost, &Response{statusCode: http.StatusServiceUnavailable}, nil, false},
		{"TransportErrorRetried", http.MethodGet, nil, errors.New("connection reset"), true},
		{"DeadlineNotRetried", http.MethodGet, nil, fmt.Errorf("wrapped: %w", context.DeadlineExceeded), false},
		{"CancellationNotRetried", http.MethodGet, nil, fmt.Errorf("wrapped: %w", context.Canceled), false},
		{"OversizeBodyNotRetried", http.MethodGet, nil, fmt.Errorf("wrapped: %w", ErrResponseTooLarge), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, policy.shouldRetry(tt.method, tt.resp, tt.err), "The default policy should classify the outcome correctly")
		})
	}

	t.Run("RetryIfReplacesPolicy", func(t *testing.T) {
		custom := RetryConfig{RetryIf: func(resp *Response, _ error) bool {
			return resp != nil && resp.StatusCode() == http.StatusServiceUnavailable
		}}.withDefaults()

		assert.True(t, custom.shouldRetry(http.MethodPost, &Response{statusCode: http.StatusServiceUnavailable}, nil), "RetryIf should be able to retry a POST")
		assert.False(t, custom.shouldRetry(http.MethodGet, nil, errors.New("boom")), "RetryIf should replace the default policy entirely")
	})
}

// TestBackoffDelay tests jittered exponential backoff and the Retry-After
// override.
func TestBackoffDelay(t *testing.T) {
	policy := RetryConfig{InitialBackoff: 100 * time.Millisecond, MaxBackoff: 2 * time.Second}.withDefaults()

	t.Run("RetryAfterWins", func(t *testing.T) {
		assert.Equal(t, 500*time.Millisecond, policy.backoffDelay(1, 500*time.Millisecond), "A server hint should be used verbatim")
	})

	t.Run("RetryAfterCapped", func(t *testing.T) {
		assert.Equal(t, 2*time.Second, policy.backoffDelay(1, 5*time.Second), "A server hint should be capped at MaxBackoff")
	})

	t.Run("JitterStaysUnderWindow", func(t *testing.T) {
		for range 50 {
			delay := policy.backoffDelay(1, 0)
			assert.GreaterOrEqual(t, delay, time.Duration(0), "The jittered delay should not be negative")
			assert.Less(t, delay, 100*time.Millisecond, "The first retry should jitter within the initial backoff")
		}

		for range 50 {
			assert.Less(t, policy.backoffDelay(3, 0), 400*time.Millisecond, "The third retry should jitter within the doubled window")
		}
	})

	t.Run("GrowthCappedAtMax", func(t *testing.T) {
		for range 50 {
			assert.Less(t, policy.backoffDelay(30, 0), 2*time.Second, "A late retry should be capped at MaxBackoff")
		}
	})

	t.Run("ShiftOverflowGuarded", func(t *testing.T) {
		assert.Less(t, policy.backoffDelay(80, 0), 2*time.Second, "An extreme retry index should not overflow the window")
	})
}

// TestRetryAfterHint tests Retry-After header parsing.
func TestRetryAfterHint(t *testing.T) {
	withHeader := func(value string) *Response {
		header := make(http.Header)
		if value != "" {
			header.Set("Retry-After", value)
		}

		return &Response{header: header}
	}

	t.Run("Seconds", func(t *testing.T) {
		assert.Equal(t, 3*time.Second, retryAfterHint(withHeader("3")), "Delta-seconds should parse")
	})

	t.Run("HTTPDate", func(t *testing.T) {
		hint := retryAfterHint(withHeader(time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)))
		assert.Greater(t, hint, 80*time.Second, "A future HTTP date should yield the remaining wait")
		assert.LessOrEqual(t, hint, 90*time.Second, "The wait should not exceed the given date")
	})

	t.Run("PastDateClamped", func(t *testing.T) {
		hint := retryAfterHint(withHeader(time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)))
		assert.Equal(t, time.Duration(0), hint, "A past HTTP date should clamp to zero")
	})

	t.Run("Malformed", func(t *testing.T) {
		assert.Equal(t, time.Duration(0), retryAfterHint(withHeader("soon")), "A malformed value should be ignored")
	})

	t.Run("Absent", func(t *testing.T) {
		assert.Equal(t, time.Duration(0), retryAfterHint(withHeader("")), "An absent header should yield zero")
	})
}

// TestRetryLoop tests the attempt loop end to end against a live server.
func TestRetryLoop(t *testing.T) {
	t.Run("RecoversAfterUnavailable", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			if hits.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)

				return
			}

			_, _ = w.Write([]byte("recovered"))
		})

		resp, err := newRetryClient(t, RetryConfig{MaxAttempts: 3}).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "The call should recover within the attempt budget")
		assert.Equal(t, "recovered", resp.String(), "The final response should be the successful one")
		assert.Equal(t, 3, resp.Attempts(), "Every attempt should be counted")
		assert.Equal(t, int32(3), hits.Load(), "The server should have been hit once per attempt")
	})

	t.Run("PostNotRetriedByDefault", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		resp, err := newRetryClient(t, RetryConfig{MaxAttempts: 3}).NewRequest().Post(t.Context(), server.URL)
		require.NoError(t, err, "The 503 should be returned, not retried")
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode(), "The last response should surface")
		assert.Equal(t, int32(1), hits.Load(), "A POST must not be retried under the default policy")
	})

	t.Run("RetryIfEnablesPostRetry", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			if hits.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		})

		cfg := RetryConfig{MaxAttempts: 3, RetryIf: func(resp *Response, _ error) bool {
			return resp != nil && resp.StatusCode() == http.StatusServiceUnavailable
		}}

		resp, err := newRetryClient(t, cfg).NewRequest().Post(t.Context(), server.URL)
		require.NoError(t, err, "The call should recover")
		assert.True(t, resp.IsSuccess(), "RetryIf should let the POST retry succeed")
		assert.Equal(t, int32(2), hits.Load(), "The POST should have been retried once")
	})

	t.Run("ExhaustionReturnsLastResponse", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		resp, err := newRetryClient(t, RetryConfig{MaxAttempts: 2}).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Exhausted retries with a response should not be an error")
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode(), "The last response should surface")
		assert.Equal(t, 2, resp.Attempts(), "The attempt count should reflect the exhausted budget")
		assert.Equal(t, int32(2), hits.Load(), "Every attempt should reach the server")
	})

	t.Run("TransportErrorRetried", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			if hits.Add(1) == 1 {
				hijacker, ok := w.(http.Hijacker)
				require.True(t, ok, "The test server should support hijacking")

				conn, _, err := hijacker.Hijack()
				require.NoError(t, err, "Hijack should succeed")
				require.NoError(t, conn.Close(), "The connection should drop")

				return
			}

			_, _ = w.Write([]byte("recovered"))
		})

		resp, err := newRetryClient(t, RetryConfig{MaxAttempts: 2}).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "A dropped connection should be retried")
		assert.Equal(t, "recovered", resp.String(), "The retry should succeed")
		assert.Equal(t, 2, resp.Attempts(), "The transport failure should count as an attempt")
	})

	t.Run("StreamBodyNeverRetried", func(t *testing.T) {
		var hits atomic.Int32

		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		resp, err := newRetryClient(t, RetryConfig{MaxAttempts: 3}).NewRequest().
			SetBodyReader(strings.NewReader("streamed"), "application/octet-stream").
			Put(t.Context(), server.URL)
		require.NoError(t, err, "The 503 should be returned, not retried")
		assert.Equal(t, 1, resp.Attempts(), "A streamed body cannot be replayed, so no retry may run")
		assert.Equal(t, int32(1), hits.Load(), "The server should have been hit exactly once")
	})

	t.Run("ContextCancelStopsBackoff", func(t *testing.T) {
		server := newRetryServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		cfg := RetryConfig{MaxAttempts: 3, InitialBackoff: 5 * time.Second, MaxBackoff: 5 * time.Second}

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()

		resp, err := newRetryClient(t, cfg).NewRequest().Get(ctx, server.URL)
		require.NoError(t, err, "The last response should surface when the context dies mid-backoff")
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode(), "The 503 should be returned")
		assert.Less(t, time.Since(start), 3*time.Second, "The backoff should be abandoned when the context expires")
	})
}
