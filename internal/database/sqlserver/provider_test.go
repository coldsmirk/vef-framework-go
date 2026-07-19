package sqlserver

import (
	"testing"

	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestBuildConfig(t *testing.T) {
	provider := NewProvider()

	t.Run("UseDefaults", func(t *testing.T) {
		connCfg, err := provider.buildConfig(&config.DataSourceConfig{})
		require.NoError(t, err, "Default config should build without error")

		assert.Equal(t, "127.0.0.1", connCfg.Host, "Should default host")
		assert.Equal(t, uint64(1433), connCfg.Port, "Should default port")
		assert.Empty(t, connCfg.Database, "Should leave database empty for the login default catalog")
		assert.Equal(t, []string{"tcp"}, connCfg.Protocols, "Should dial over tcp")
		assert.Equal(t, msdsn.Encryption(msdsn.EncryptionDisabled), connCfg.Encryption, "Should refuse encryption when ssl_mode is omitted")
		assert.Nil(t, connCfg.TLSConfig, "Should carry no TLS config in plaintext mode")
	})

	t.Run("UseProvidedValues", func(t *testing.T) {
		connCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Host:     "db.internal",
			Port:     14330,
			User:     "vef",
			Password: "secret",
			Database: "exchange",
		})
		require.NoError(t, err, "Provided config should build without error")

		assert.Equal(t, "db.internal", connCfg.Host, "Should use configured host")
		assert.Equal(t, uint64(14330), connCfg.Port, "Should use configured port")
		assert.Equal(t, "vef", connCfg.User, "Should use configured user")
		assert.Equal(t, "secret", connCfg.Password, "Should use configured password")
		assert.Equal(t, "exchange", connCfg.Database, "Should use configured database")
	})

	t.Run("SSLRequireEnablesTLS", func(t *testing.T) {
		connCfg, err := provider.buildConfig(&config.DataSourceConfig{
			SSLMode: config.SSLRequire,
		})
		require.NoError(t, err, "require mode should build without error")

		assert.Equal(t, msdsn.Encryption(msdsn.EncryptionRequired), connCfg.Encryption, "require mode should force TDS encryption")
		require.NotNil(t, connCfg.TLSConfig, "require mode should attach a TLS config")
		assert.True(t, connCfg.TLSConfig.InsecureSkipVerify, "require mode should skip verification")
	})

	t.Run("SSLVerifyFullPinsHost", func(t *testing.T) {
		connCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Host:    "db.internal",
			SSLMode: config.SSLVerifyFull,
		})
		require.NoError(t, err, "verify-full mode should build without error")

		assert.Equal(t, msdsn.Encryption(msdsn.EncryptionRequired), connCfg.Encryption, "verify-full mode should force TDS encryption")
		require.NotNil(t, connCfg.TLSConfig, "verify-full mode should attach a TLS config")
		assert.Equal(t, "db.internal", connCfg.TLSConfig.ServerName, "verify-full should pin the configured host")
	})

	t.Run("UnknownSSLModeErrors", func(t *testing.T) {
		_, err := provider.buildConfig(&config.DataSourceConfig{
			SSLMode: "bogus",
		})
		require.Error(t, err, "An unknown ssl_mode should surface an error from the provider")
	})
}
