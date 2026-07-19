package oracle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
)

func TestBuildDatabaseURL(t *testing.T) {
	provider := NewProvider()

	t.Run("UseDefaults", func(t *testing.T) {
		databaseURL, err := provider.buildDatabaseURL(&config.DataSourceConfig{
			User:     "vef",
			Password: "secret",
			Database: "FREEPDB1",
		})
		require.NoError(t, err, "Default config should build without error")

		assert.Equal(t, "oracle://vef:secret@127.0.0.1:1521/FREEPDB1", databaseURL, "Should default host/port and carry no options")
	})

	t.Run("UseProvidedValues", func(t *testing.T) {
		databaseURL, err := provider.buildDatabaseURL(&config.DataSourceConfig{
			Host:     "db.internal",
			Port:     15210,
			User:     "vef",
			Password: "secret",
			Database: "EXCHANGE",
		})
		require.NoError(t, err, "Provided config should build without error")

		assert.Equal(t, "oracle://vef:secret@db.internal:15210/EXCHANGE", databaseURL, "Should use configured host/port/service")
	})

	t.Run("ServiceNameRequired", func(t *testing.T) {
		_, err := provider.buildDatabaseURL(&config.DataSourceConfig{User: "vef"})

		require.ErrorIs(t, err, ErrOracleServiceRequired, "A missing database (service name) should fail fast")
	})

	t.Run("SSLRequireDisablesVerification", func(t *testing.T) {
		databaseURL, err := provider.buildDatabaseURL(&config.DataSourceConfig{
			Database: "FREEPDB1",
			SSLMode:  config.SSLRequire,
		})
		require.NoError(t, err, "require mode should build without error")

		assert.Contains(t, databaseURL, "SSL=TRUE", "require mode should enable TCPS")
		assert.Contains(t, databaseURL, "SSL VERIFY=FALSE", "require mode should skip verification")
	})

	t.Run("SSLVerifyModesKeepVerification", func(t *testing.T) {
		for _, mode := range []config.SSLMode{config.SSLVerifyCA, config.SSLVerifyFull} {
			databaseURL, err := provider.buildDatabaseURL(&config.DataSourceConfig{
				Database: "FREEPDB1",
				SSLMode:  mode,
			})
			require.NoError(t, err, "%s mode should build without error", mode)

			assert.Contains(t, databaseURL, "SSL=TRUE", "%s mode should enable TCPS", mode)
			assert.NotContains(t, databaseURL, "SSL VERIFY", "%s mode should keep go-ora's default verification on", mode)
		}
	})

	t.Run("RootCertUnsupported", func(t *testing.T) {
		_, err := provider.buildDatabaseURL(&config.DataSourceConfig{
			Database:    "FREEPDB1",
			SSLMode:     config.SSLVerifyFull,
			SSLRootCert: "/etc/ssl/ca.pem",
		})

		require.ErrorIs(t, err, ErrOracleRootCertUnsupported, "A PEM root cert must fail fast instead of being ignored")
	})

	t.Run("UnknownSSLModeErrors", func(t *testing.T) {
		_, err := provider.buildDatabaseURL(&config.DataSourceConfig{
			Database: "FREEPDB1",
			SSLMode:  "bogus",
		})

		require.ErrorIs(t, err, dbtls.ErrUnknownSSLMode, "An unknown ssl_mode should surface the shared sentinel")
	})
}
