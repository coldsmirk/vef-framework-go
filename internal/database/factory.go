package database

import (
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Open constructs a *bun.DB for the supplied data source configuration. name
// is only used for logging context (so callers can tell which source produced
// an error) and may be empty in tests or one-off constructions.
//
// Open is the building block the Registry uses for every data source it
// manages; it is also exported for tests and the migration helpers that need
// to spin up a connection outside the FX lifecycle.
func Open(name string, cfg config.DataSourceConfig, options ...Option) (*bun.DB, error) {
	provider, exists := registry.provider(cfg.Kind)
	if !exists {
		return nil, newUnsupportedDBKindError(cfg.Kind)
	}

	sqlDB, dialect, err := provider.Connect(&cfg)
	if err != nil || sqlDB == nil {
		return nil, err
	}

	opts := newDefaultOptions(&cfg)
	opts.apply(options...)

	if opts.PoolConfig != nil {
		opts.PoolConfig.ApplyToDB(sqlDB)
	}

	_ = name // reserved for future per-source logging context

	return setupBunDB(sqlDB, dialect, opts), nil
}
