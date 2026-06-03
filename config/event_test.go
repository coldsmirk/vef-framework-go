package config_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestEventConfigEffectiveAccessors(t *testing.T) {
	t.Run("DefaultsWhenUnset", func(t *testing.T) {
		cfg := config.EventConfig{}
		require.Equal(t, "memory", cfg.EffectiveDefaultTransport(), "Default transport should be memory")
		require.Equal(t, 4096, cfg.EffectiveAsyncQueueSize(), "Default async queue size should be 4096")
		require.Equal(t, 4, cfg.EffectiveAsyncWorkers(), "Default async workers should be 4")
		require.Equal(t, 5*time.Second, cfg.EffectivePublishTimeout(), "Default publish timeout should be five seconds")
	})

	t.Run("OverridesWhenPositive", func(t *testing.T) {
		cfg := config.EventConfig{
			DefaultTransport: "outbox",
			AsyncQueueSize:   8,
			AsyncWorkers:     2,
			PublishTimeout:   time.Minute,
		}
		require.Equal(t, "outbox", cfg.EffectiveDefaultTransport(), "Configured transport should override the default")
		require.Equal(t, 8, cfg.EffectiveAsyncQueueSize(), "Configured async queue size should override the default")
		require.Equal(t, 2, cfg.EffectiveAsyncWorkers(), "Configured async workers should override the default")
		require.Equal(t, time.Minute, cfg.EffectivePublishTimeout(), "Configured publish timeout should override the default")
	})

	t.Run("NonPositiveFallsBackToDefault", func(t *testing.T) {
		cfg := config.EventConfig{AsyncQueueSize: -1, AsyncWorkers: -1, PublishTimeout: -time.Second}
		require.Equal(t, 4096, cfg.EffectiveAsyncQueueSize(), "Negative async queue size should re-select the default")
		require.Equal(t, 4, cfg.EffectiveAsyncWorkers(), "Negative async workers should re-select the default")
		require.Equal(t, 5*time.Second, cfg.EffectivePublishTimeout(), "Negative publish timeout should re-select the default")
	})
}

func TestEventOutboxConfigDefaults(t *testing.T) {
	t.Run("DefaultsWhenUnset", func(t *testing.T) {
		cfg := config.EventOutboxTransportConfig{}
		require.Equal(t, time.Hour, cfg.EffectiveCleanupInterval(), "Default outbox cleanup interval should be one hour")
		require.Equal(t, 7*24*time.Hour, cfg.EffectiveCompletedTTL(), "Default outbox completed TTL should be seven days")
	})

	t.Run("OverridesWhenPositive", func(t *testing.T) {
		cfg := config.EventOutboxTransportConfig{CleanupInterval: 2 * time.Hour, CompletedTTL: 24 * time.Hour}
		require.Equal(t, 2*time.Hour, cfg.EffectiveCleanupInterval(), "Configured outbox cleanup interval should override the default")
		require.Equal(t, 24*time.Hour, cfg.EffectiveCompletedTTL(), "Configured outbox completed TTL should override the default")
	})
}

func TestEventInboxConfigDefaults(t *testing.T) {
	t.Run("Retention", func(t *testing.T) {
		cfg := config.EventInboxConfig{}
		require.Equal(t, 7*24*time.Hour, cfg.EffectiveRetention(), "Default inbox retention should be seven days")
	})

	t.Run("ProcessingLease", func(t *testing.T) {
		cfg := config.EventInboxConfig{}
		got := cfg.EffectiveProcessingLease()
		require.Equal(t, 10*time.Minute, got, "Default inbox processing lease should be ten minutes")
	})

	t.Run("ProcessingLeaseOverride", func(t *testing.T) {
		cfg := config.EventInboxConfig{ProcessingLease: time.Hour}
		got := cfg.EffectiveProcessingLease()
		require.Equal(t, time.Hour, got, "Configured inbox processing lease should override the default")
	})

	t.Run("CleanupInterval", func(t *testing.T) {
		cfg := config.EventInboxConfig{}
		require.Equal(t, time.Hour, cfg.EffectiveCleanupInterval(), "Default inbox cleanup interval should be one hour")
	})
}

func TestEventConfigValidate(t *testing.T) {
	enabled := func(maxRetries int, retention time.Duration) config.EventConfig {
		cfg := config.EventConfig{}
		cfg.Middleware.Inbox = true
		cfg.Transports.Outbox.Enabled = true
		cfg.Transports.Outbox.MaxRetries = maxRetries
		cfg.Inbox.Retention = retention

		return cfg
	}

	t.Run("SkippedWhenInboxDisabled", func(t *testing.T) {
		cfg := config.EventConfig{}
		cfg.Transports.Outbox.Enabled = true
		cfg.Inbox.Retention = time.Second
		require.NoError(t, cfg.Validate(), "Validation should be skipped when the inbox middleware is disabled")
	})

	t.Run("SkippedWhenOutboxDisabled", func(t *testing.T) {
		cfg := config.EventConfig{}
		cfg.Middleware.Inbox = true
		cfg.Inbox.Retention = time.Second
		require.NoError(t, cfg.Validate(), "Validation should be skipped when the outbox transport is disabled")
	})

	t.Run("RetentionShorterThanHorizonFails", func(t *testing.T) {
		// maxRetries=10 -> horizon = sum_{k=1..10} 2^k = 2046s (~34m).
		cfg := enabled(10, time.Minute)
		require.ErrorIs(t, cfg.Validate(), config.ErrInboxRetentionTooShort,
			"Retention below the backoff horizon should be rejected")
	})

	t.Run("RetentionLongerThanHorizonPasses", func(t *testing.T) {
		cfg := enabled(10, 7*24*time.Hour)
		require.NoError(t, cfg.Validate(), "Retention comfortably above the backoff horizon should pass")
	})

	t.Run("DefaultMaxRetriesWhenUnset", func(t *testing.T) {
		// maxRetries<=0 defaults to 10 -> horizon ~34m, so a 1m retention still fails.
		cfg := enabled(0, time.Minute)
		require.ErrorIs(t, cfg.Validate(), config.ErrInboxRetentionTooShort,
			"Unset max_retries should default to 10 and still enforce the horizon")
	})

	t.Run("LargeMaxRetriesSaturatesAndStaysFailClosed", func(t *testing.T) {
		// maxRetries=60 makes the raw seconds*time.Second overflow int64 to a
		// negative value; the saturating horizon must keep the guard tripping.
		cfg := enabled(60, 7*24*time.Hour)
		require.ErrorIs(t, cfg.Validate(), config.ErrInboxRetentionTooShort,
			"An overflowing backoff horizon must saturate and keep the retention guard fail-closed")
	})

	t.Run("MaxRetriesAtSaturationBoundary", func(t *testing.T) {
		// Largest representable duration is math.MaxInt64 ns (~292 years);
		// any practical retention is smaller, so saturation always fails.
		cfg := enabled(62, time.Duration(math.MaxInt64))
		require.ErrorIs(t, cfg.Validate(), config.ErrInboxRetentionTooShort,
			"Even the maximum retention cannot exceed a saturated horizon")
	})
}
