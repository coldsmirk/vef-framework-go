package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestComputeBackoff verifies the exponential growth schedule, the cap at
// deleteMaxBackoff, and that large attempt values do not overflow.
func TestComputeBackoff(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{
			name:    "attempt 0 equals base backoff",
			attempt: 0,
			want:    deleteBaseBackoff, // 30s << 0 = 30s
		},
		{
			name:    "attempt 1 doubles",
			attempt: 1,
			want:    deleteBaseBackoff * 2, // 30s << 1 = 60s
		},
		{
			name:    "attempt 2 quadruples",
			attempt: 2,
			want:    deleteBaseBackoff * 4, // 30s << 2 = 120s
		},
		{
			name:    "attempt 6 is still below cap",
			attempt: 6,
			want:    deleteBaseBackoff << 6, // 30s * 64 = 1920s = 32min
		},
		{
			name:    "attempt 7 hits the cap",
			attempt: 7,
			// 30s << 7 = 3840s = 64min, which exceeds 1h cap, so cap applies.
			want: deleteMaxBackoff,
		},
		{
			name:    "attempt 8 is still capped at max backoff",
			attempt: 8,
			want:    deleteMaxBackoff,
		},
		{
			name:    "very large attempt does not overflow",
			attempt: 1000,
			want:    deleteMaxBackoff,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeBackoff(tc.attempt)
			assert.Equal(t, tc.want, got, "computeBackoff(%d) should return %s", tc.attempt, tc.want)
		})
	}
}

// TestClassifyDeleteError verifies every distinct error category and
// the fallback for unknown / nil errors.
func TestClassifyDeleteError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error falls back to transient",
			err:  nil,
			want: "transient",
		},
		{
			name: "unknown error falls back to transient",
			err:  errors.New("some unexpected backend error"),
			want: "transient",
		},
		{
			name: "ErrAccessDenied maps to access_denied",
			err:  storage.ErrAccessDenied,
			want: "access_denied",
		},
		{
			name: "wrapped ErrAccessDenied still maps to access_denied",
			err:  errors.Join(errors.New("outer"), storage.ErrAccessDenied),
			want: "access_denied",
		},
		{
			name: "ErrBucketNotFound maps to bucket_not_found",
			err:  storage.ErrBucketNotFound,
			want: "bucket_not_found",
		},
		{
			name: "wrapped ErrBucketNotFound still maps to bucket_not_found",
			err:  errors.Join(errors.New("outer"), storage.ErrBucketNotFound),
			want: "bucket_not_found",
		},
		{
			name: "ErrUploadSessionNotFound maps to session_not_found",
			err:  storage.ErrUploadSessionNotFound,
			want: "session_not_found",
		},
		{
			name: "wrapped ErrUploadSessionNotFound still maps to session_not_found",
			err:  errors.Join(errors.New("outer"), storage.ErrUploadSessionNotFound),
			want: "session_not_found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyDeleteError(tc.err)
			assert.Equal(t, tc.want, got, "classifyDeleteError(%v) should return %q", tc.err, tc.want)
		})
	}
}
