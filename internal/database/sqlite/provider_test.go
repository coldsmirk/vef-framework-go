package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestBuildDsn(t *testing.T) {
	provider := NewProvider()

	t.Run("InMemory", func(t *testing.T) {
		dsn := provider.buildDsn(&config.DataSourceConfig{})
		assert.Contains(t, dsn, "mode=memory", "Should use in-memory mode when path is empty")
		assert.Contains(t, dsn, "cache=shared", "Should use shared cache for in-memory DB")
		assert.Contains(t, dsn, "_busy_timeout=5000", "Should set sqlite busy timeout")
		assert.Contains(t, dsn, "_pragma=busy_timeout(5000)", "Should set busy_timeout pragma for each connection")
		assert.Contains(t, dsn, "_pragma=foreign_keys(ON)", "Should enable foreign keys for in-memory sqlite")
	})

	t.Run("FilePath", func(t *testing.T) {
		dsn := provider.buildDsn(&config.DataSourceConfig{Path: "/tmp/test.db"})
		assert.Contains(t, dsn, "file:/tmp/test.db", "Should use file path as sqlite DSN")
		assert.Contains(t, dsn, "_busy_timeout=5000", "Should set sqlite busy timeout")
		assert.Contains(t, dsn, "_pragma=busy_timeout(5000)", "Should set busy_timeout pragma for each connection")
		assert.Contains(t, dsn, "_pragma=journal_mode(WAL)", "Should enable WAL mode for file-based sqlite to improve concurrency")
		assert.Contains(t, dsn, "_pragma=synchronous(NORMAL)", "Should set synchronous mode for WAL workload")
		assert.Contains(t, dsn, "_pragma=foreign_keys(ON)", "Should enable foreign keys for file-based sqlite")
	})
}
