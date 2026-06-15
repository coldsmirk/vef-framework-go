package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestPoolConfigFor(t *testing.T) {
	t.Run("ServerDialectsUseLargePool", func(t *testing.T) {
		for _, kind := range []config.DBKind{config.Postgres, config.MySQL} {
			got := poolConfigFor(config.DataSourceConfig{Kind: kind})

			assert.GreaterOrEqual(t, got.MaxOpenConns, ServerMinOpenConns,
				"server dialect %s should keep the large open-connection floor", kind)
			assert.GreaterOrEqual(t, got.MaxIdleConns, ServerMinIdleConns,
				"server dialect %s should keep the large idle-connection floor", kind)
			assert.Equal(t, ServerConnMaxLifetime, got.ConnMaxLifetime,
				"server dialect %s should recycle connections on the server lifetime", kind)
		}
	})

	t.Run("FileSQLiteCapsConnections", func(t *testing.T) {
		got := poolConfigFor(config.DataSourceConfig{Kind: config.SQLite, Path: "/tmp/app.db"})

		assert.Equal(t, SQLiteFileMaxOpenConns, got.MaxOpenConns,
			"file SQLite should cap open connections to a single writer")
		assert.Equal(t, SQLiteFileMaxIdleConns, got.MaxIdleConns,
			"file SQLite should keep a single idle connection")
		assert.Equal(t, SQLiteFileConnMaxLifetime, got.ConnMaxLifetime,
			"file SQLite should recycle its lone connection on the file lifetime")
	})

	t.Run("InMemorySQLiteKeepsServerPool", func(t *testing.T) {
		// In-memory SQLite stays on the server pool: the shared-cache database
		// needs a connection kept alive, and nested queries (e.g. the schema
		// inspector) require more than one connection to avoid deadlock.
		got := poolConfigFor(config.DataSourceConfig{Kind: config.SQLite})

		assert.Equal(t, serverPoolConfig(), got,
			"in-memory SQLite should keep the server pool unchanged to preserve concurrency and connection liveness")
		assert.Greater(t, got.MaxOpenConns, 1,
			"in-memory SQLite must allow more than one connection so nested queries do not deadlock")
	})
}

func TestApplyToDB(t *testing.T) {
	cfg := ConnectionPoolConfig{
		MaxIdleConns:    3,
		MaxOpenConns:    7,
		ConnMaxIdleTime: 11 * time.Second,
		ConnMaxLifetime: 13 * time.Second,
	}

	spy := new(poolSpy)
	cfg.ApplyToDB(spy)

	assert.Equal(t, 3, spy.maxIdle, "MaxIdleConns should pass through verbatim")
	assert.Equal(t, 7, spy.maxOpen, "MaxOpenConns should pass through verbatim")
	assert.Equal(t, 11*time.Second, spy.maxIdleTime, "ConnMaxIdleTime should pass through verbatim")
	assert.Equal(t, 13*time.Second, spy.maxLifetime, "ConnMaxLifetime should pass through verbatim")
}

type poolSpy struct {
	maxIdle     int
	maxOpen     int
	maxIdleTime time.Duration
	maxLifetime time.Duration
}

func (s *poolSpy) SetMaxIdleConns(n int)              { s.maxIdle = n }
func (s *poolSpy) SetMaxOpenConns(n int)              { s.maxOpen = n }
func (s *poolSpy) SetConnMaxIdleTime(d time.Duration) { s.maxIdleTime = d }
func (s *poolSpy) SetConnMaxLifetime(d time.Duration) { s.maxLifetime = d }
