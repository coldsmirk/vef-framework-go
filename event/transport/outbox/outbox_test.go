package outbox_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event/transport/outbox"
)

func TestConfigDefaults(t *testing.T) {
	t.Run("RelayInterval", func(t *testing.T) {
		got := outbox.Config{}.EffectiveRelayInterval()
		require.Equal(t, 10*time.Second, got, "Default relay interval should balance latency and polling load")
	})

	t.Run("RelayIntervalOverride", func(t *testing.T) {
		got := outbox.Config{RelayInterval: 2 * time.Second}.EffectiveRelayInterval()
		require.Equal(t, 2*time.Second, got, "Configured relay interval should override the default")
	})
}
