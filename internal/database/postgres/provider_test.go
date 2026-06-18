package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestConnectSSLMode(t *testing.T) {
	provider := NewProvider()

	t.Run("DisableConnectsPlaintext", func(t *testing.T) {
		db, err := provider.Connect(&config.DataSourceConfig{Kind: config.Postgres})

		require.NoError(t, err, "disable (default) should build the connector without error")
		require.NotNil(t, db, "a *sql.DB should be returned for the default plaintext path")
		require.NoError(t, db.Close(), "the unused handle should close cleanly")
	})

	t.Run("RequireConnects", func(t *testing.T) {
		db, err := provider.Connect(&config.DataSourceConfig{
			Kind:    config.Postgres,
			SSLMode: config.SSLRequire,
		})

		require.NoError(t, err, "require mode should build the connector without error")
		require.NotNil(t, db, "a *sql.DB should be returned when TLS is requested")
		require.NoError(t, db.Close(), "the unused handle should close cleanly")
	})

	t.Run("UnknownSSLModeErrors", func(t *testing.T) {
		db, err := provider.Connect(&config.DataSourceConfig{
			Kind:    config.Postgres,
			SSLMode: "bogus",
		})

		require.Error(t, err, "an unknown ssl_mode should fail fast at Connect")
		assert.Nil(t, db, "no handle should be returned when the ssl_mode is invalid")
	})
}
