package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestGetMigrationSQL(t *testing.T) {
	t.Run("Postgres", func(t *testing.T) {
		sql, err := GetMigrationSQL(config.Postgres)
		require.NoError(t, err, "Should load Postgres migration SQL")
		assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS apv_flow", "Should contain flow table DDL")
		assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS apv_instance", "Should contain instance table DDL")
		assert.Contains(t, sql, "CREATE TABLE IF NOT EXISTS apv_task", "Should contain task table DDL")
	})

	t.Run("UnsupportedKind", func(t *testing.T) {
		_, err := GetMigrationSQL("unknown")
		require.Error(t, err, "Should error for unsupported database kind")
		assert.Contains(t, err.Error(), "unsupported database kind", "Should include kind info in error")
	})
}
