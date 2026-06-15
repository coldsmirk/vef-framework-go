package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestBuildConfig(t *testing.T) {
	provider := NewProvider()

	t.Run("UseDefaults", func(t *testing.T) {
		mysqlCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Database: "vef_test",
		})
		require.NoError(t, err, "Default config should build without error")

		assert.Equal(t, "root", mysqlCfg.User, "Should default user to root")
		assert.Equal(t, "127.0.0.1:3306", mysqlCfg.Addr, "Should default host/port")
		assert.Equal(t, "vef_test", mysqlCfg.DBName, "Should set database name")
		assert.True(t, mysqlCfg.ParseTime, "Should enable ParseTime")
		assert.Equal(t, "utf8mb4_unicode_ci", mysqlCfg.Collation, "Should set collation")
		assert.True(t, mysqlCfg.MultiStatements, "Should enable multi-statements for migration scripts")
		assert.Nil(t, mysqlCfg.TLS, "Should default to plaintext (no TLS) when ssl_mode is omitted")
	})

	t.Run("UseProvidedValues", func(t *testing.T) {
		mysqlCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Host:     "db.internal",
			Port:     3307,
			User:     "vef",
			Password: "secret",
			Database: "approval",
		})
		require.NoError(t, err, "Provided config should build without error")

		assert.Equal(t, "vef", mysqlCfg.User, "Should use configured user")
		assert.Equal(t, "secret", mysqlCfg.Passwd, "Should use configured password")
		assert.Equal(t, "db.internal:3307", mysqlCfg.Addr, "Should use configured host/port")
		assert.Equal(t, "approval", mysqlCfg.DBName, "Should use configured database")
		assert.True(t, mysqlCfg.MultiStatements, "Should keep multi-statements enabled")
	})

	t.Run("SSLModeRequireEnablesTLS", func(t *testing.T) {
		mysqlCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Database: "vef_test",
			SSLMode:  config.SSLModeRequire,
		})
		require.NoError(t, err, "require mode should build without error")

		require.NotNil(t, mysqlCfg.TLS, "require mode should attach a TLS config")
		assert.True(t, mysqlCfg.TLS.InsecureSkipVerify, "require mode should skip verification")
	})

	t.Run("SSLModeVerifyFullPinsHost", func(t *testing.T) {
		mysqlCfg, err := provider.buildConfig(&config.DataSourceConfig{
			Host:     "db.internal",
			Database: "vef_test",
			SSLMode:  config.SSLModeVerifyFull,
		})
		require.NoError(t, err, "verify-full mode should build without error")

		require.NotNil(t, mysqlCfg.TLS, "verify-full mode should attach a TLS config")
		assert.Equal(t, "db.internal", mysqlCfg.TLS.ServerName, "verify-full should pin the configured host")
	})

	t.Run("UnknownSSLModeErrors", func(t *testing.T) {
		_, err := provider.buildConfig(&config.DataSourceConfig{
			Database: "vef_test",
			SSLMode:  "bogus",
		})
		require.Error(t, err, "An unknown ssl_mode should surface an error from the provider")
	})
}
