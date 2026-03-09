package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestBuildConfig(t *testing.T) {
	provider := NewProvider()

	t.Run("UseDefaults", func(t *testing.T) {
		mysqlCfg := provider.buildConfig(&config.DataSourceConfig{
			Database: "vef_test",
		})

		assert.Equal(t, "root", mysqlCfg.User, "Should default user to root")
		assert.Equal(t, "127.0.0.1:3306", mysqlCfg.Addr, "Should default host/port")
		assert.Equal(t, "vef_test", mysqlCfg.DBName, "Should set database name")
		assert.True(t, mysqlCfg.ParseTime, "Should enable ParseTime")
		assert.Equal(t, "utf8mb4_unicode_ci", mysqlCfg.Collation, "Should set collation")
		assert.True(t, mysqlCfg.MultiStatements, "Should enable multi-statements for migration scripts")
	})

	t.Run("UseProvidedValues", func(t *testing.T) {
		mysqlCfg := provider.buildConfig(&config.DataSourceConfig{
			Host:     "db.internal",
			Port:     3307,
			User:     "vef",
			Password: "secret",
			Database: "approval",
		})

		assert.Equal(t, "vef", mysqlCfg.User, "Should use configured user")
		assert.Equal(t, "secret", mysqlCfg.Passwd, "Should use configured password")
		assert.Equal(t, "db.internal:3307", mysqlCfg.Addr, "Should use configured host/port")
		assert.Equal(t, "approval", mysqlCfg.DBName, "Should use configured database")
		assert.True(t, mysqlCfg.MultiStatements, "Should keep multi-statements enabled")
	})
}
