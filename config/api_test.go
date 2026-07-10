package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIRateLimitConfig(t *testing.T) {
	t.Run("EffectiveMax", func(t *testing.T) {
		tests := []struct {
			name string
			max  int
			want int
		}{
			{name: "ZeroResolvesToDefault", max: 0, want: DefaultAPIRateLimitMax},
			{name: "NegativeResolvesToDefault", max: -5, want: DefaultAPIRateLimitMax},
			{name: "ExplicitValueWins", max: 30, want: 30},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := APIRateLimitConfig{Max: tt.max}
				assert.Equal(t, tt.want, cfg.EffectiveMax(), "EffectiveMax should resolve %d to %d", tt.max, tt.want)
			})
		}
	})

	t.Run("EffectivePeriod", func(t *testing.T) {
		tests := []struct {
			name   string
			period time.Duration
			want   time.Duration
		}{
			{name: "ZeroResolvesToDefault", period: 0, want: DefaultAPIRateLimitPeriod},
			{name: "NegativeResolvesToDefault", period: -time.Second, want: DefaultAPIRateLimitPeriod},
			{name: "ExplicitValueWins", period: time.Minute, want: time.Minute},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := APIRateLimitConfig{Period: tt.period}
				assert.Equal(t, tt.want, cfg.EffectivePeriod(), "EffectivePeriod should resolve %v to %v", tt.period, tt.want)
			})
		}
	})
}
