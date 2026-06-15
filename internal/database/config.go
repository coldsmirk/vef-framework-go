package database

import (
	"runtime"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
)

// ConnectionPoolConfig holds the database/sql pool knobs applied to a freshly
// opened *sql.DB. Profiles are dialect-aware (see poolConfigFor): server
// dialects get a large concurrent pool, while SQLite — effectively
// single-writer — is capped low to avoid burning file descriptors and
// amplifying SQLITE_BUSY contention.
type ConnectionPoolConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// Server-dialect (Postgres/MySQL) pool defaults. Sized relative to GOMAXPROCS
// with a generous floor so a busy service is not starved on small core counts.
const (
	ServerMaxIdleConnsMultiplier = 4
	ServerMaxOpenConnsMultiplier = 16
	ServerMinIdleConns           = 25
	ServerMinOpenConns           = 100
	ServerConnMaxIdleTime        = 5 * time.Minute
	ServerConnMaxLifetime        = 30 * time.Minute
)

// SQLite file-mode pool defaults. SQLite is effectively single-writer, so a
// large server-sized pool only burns file descriptors and amplifies
// SQLITE_BUSY contention. The cap is kept small but strictly above one: the
// schema inspector (and any nested-query code path) opens a second statement
// while a result set is still streaming, which would deadlock against a
// single-connection pool. WAL mode (enabled by the SQLite provider for files)
// lets the remaining headroom serve concurrent readers.
const (
	SQLiteFileMaxOpenConns    = 4
	SQLiteFileMaxIdleConns    = 4
	SQLiteFileConnMaxIdleTime = 5 * time.Minute
	SQLiteFileConnMaxLifetime = 30 * time.Minute
)

// poolConfigFor selects the connection-pool profile for a data source by
// dialect:
//
//   - File-mode SQLite uses a small pool sized to SQLite's single-writer model.
//   - In-memory SQLite keeps the server-sized pool. The shared-cache in-memory
//     database lives only as long as a connection is open, and the existing
//     defaults (generous idle floor, no aggressive eviction) keep it alive and
//     allow the concurrent connections nested queries require — so it is left
//     exactly as before to avoid any regression.
//   - Every other (server) dialect uses the large concurrent pool.
func poolConfigFor(cfg config.DataSourceConfig) ConnectionPoolConfig {
	if cfg.Kind == config.SQLite && cfg.Path != "" {
		return ConnectionPoolConfig{
			MaxIdleConns:    SQLiteFileMaxIdleConns,
			MaxOpenConns:    SQLiteFileMaxOpenConns,
			ConnMaxIdleTime: SQLiteFileConnMaxIdleTime,
			ConnMaxLifetime: SQLiteFileConnMaxLifetime,
		}
	}

	return serverPoolConfig()
}

func serverPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxIdleConns:    max(runtime.GOMAXPROCS(0)*ServerMaxIdleConnsMultiplier, ServerMinIdleConns),
		MaxOpenConns:    max(runtime.GOMAXPROCS(0)*ServerMaxOpenConnsMultiplier, ServerMinOpenConns),
		ConnMaxIdleTime: ServerConnMaxIdleTime,
		ConnMaxLifetime: ServerConnMaxLifetime,
	}
}

func (c ConnectionPoolConfig) ApplyToDB(db interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxIdleTime(time.Duration)
	SetConnMaxLifetime(time.Duration)
},
) {
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetConnMaxIdleTime(c.ConnMaxIdleTime)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
}
