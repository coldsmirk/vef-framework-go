package lock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveAcquireConfig(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		want    acquireConfig
		wantErr error
	}{
		{
			name: "DefaultsWhenNoOptions",
			opts: nil,
			want: acquireConfig{ttl: DefaultTTL, retryInterval: DefaultRetryInterval},
		},
		{
			name: "ExplicitValuesWin",
			opts: []Option{WithTTL(time.Minute), WithWait(time.Second), WithRetryInterval(10 * time.Millisecond), WithAutoRenew(true)},
			want: acquireConfig{ttl: time.Minute, wait: time.Second, retryInterval: 10 * time.Millisecond, autoRenew: true},
		},
		{
			name: "NonPositiveTTLFallsBackToDefault",
			opts: []Option{WithTTL(0)},
			want: acquireConfig{ttl: DefaultTTL, retryInterval: DefaultRetryInterval},
		},
		{
			name: "NegativeRetryIntervalFallsBackToDefault",
			opts: []Option{WithRetryInterval(-time.Second)},
			want: acquireConfig{ttl: DefaultTTL, retryInterval: DefaultRetryInterval},
		},
		{
			name: "LaterOptionOverridesEarlier",
			opts: []Option{WithAutoRenew(true), WithAutoRenew(false)},
			want: acquireConfig{ttl: DefaultTTL, retryInterval: DefaultRetryInterval},
		},
		{
			name: "MinimumAutoRenewTTLAccepted",
			opts: []Option{WithTTL(MinAutoRenewTTL), WithAutoRenew(true)},
			want: acquireConfig{ttl: MinAutoRenewTTL, retryInterval: DefaultRetryInterval, autoRenew: true},
		},
		{
			name:    "ShortAutoRenewTTLRejected",
			opts:    []Option{WithTTL(MinAutoRenewTTL - time.Nanosecond), WithAutoRenew(true)},
			wantErr: ErrAutoRenewTTLTooShort,
		},
		{
			name: "ShortTTLWithoutAutoRenewAccepted",
			opts: []Option{WithTTL(time.Millisecond)},
			want: acquireConfig{ttl: time.Millisecond, retryInterval: DefaultRetryInterval},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAcquireConfig(tt.opts)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "invalid option combinations should return their sentinel error")

				return
			}

			if assert.NoError(t, err, "valid option combinations should resolve") {
				assert.Equal(t, tt.want, got, "resolved config should apply options over defaults")
			}
		})
	}
}
