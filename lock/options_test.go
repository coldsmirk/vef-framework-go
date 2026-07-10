package lock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveAcquireConfig(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want acquireConfig
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveAcquireConfig(tt.opts), "resolved config should apply options over defaults")
		})
	}
}
