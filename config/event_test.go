package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestEventInboxConfigDefaults(t *testing.T) {
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
}
