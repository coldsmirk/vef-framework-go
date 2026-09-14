package config

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIBodyEncodingConfig(t *testing.T) {
	aesKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	sm4Key := base64.StdEncoding.EncodeToString(make([]byte, 16))

	t.Run("Disabled", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{}

		assert.Equal(t, APIBodyEncodingAESGCMBase64, cfg.EffectiveEncoding(), "empty encoding should resolve to AES-GCM")
		assert.NoError(t, cfg.Validate(), "disabled protected body transport should not require a key")
	})

	t.Run("AES", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{Enabled: true, Key: aesKey}

		assert.NoError(t, cfg.Validate(), "the default AES-GCM encoding should accept a 32-byte key")
	})

	t.Run("SM4", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{
			Enabled:  true,
			Encoding: APIBodyEncodingSM4GCMBase64,
			Key:      sm4Key,
		}

		assert.NoError(t, cfg.Validate(), "SM4-GCM should accept a 16-byte key")
	})

	t.Run("MissingKey", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{Enabled: true}

		assert.ErrorIs(t, cfg.Validate(), ErrMissingAPIBodyEncodingKey, "enabled transport should require a key")
	})

	t.Run("MalformedKey", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{Enabled: true, Key: "not-base64"}

		assert.ErrorIs(t, cfg.Validate(), ErrInvalidAPIBodyEncodingKey, "key should be standard base64")
	})

	t.Run("InvalidAESKeySize", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{
			Enabled: true,
			Key:     base64.StdEncoding.EncodeToString(make([]byte, 15)),
		}

		assert.ErrorIs(t, cfg.Validate(), ErrInvalidAPIBodyEncodingKey, "AES should reject a 15-byte key")
	})

	t.Run("InvalidSM4KeySize", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{
			Enabled:  true,
			Encoding: APIBodyEncodingSM4GCMBase64,
			Key:      aesKey,
		}

		assert.ErrorIs(t, cfg.Validate(), ErrInvalidAPIBodyEncodingKey, "SM4 should reject a 32-byte key")
	})

	t.Run("UnsupportedEncoding", func(t *testing.T) {
		cfg := APIBodyEncodingConfig{
			Enabled:  true,
			Encoding: "rot13",
			Key:      aesKey,
		}

		assert.ErrorIs(t, cfg.Validate(), ErrInvalidAPIBodyEncoding, "unknown encodings should fail configuration")
	})
}

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
