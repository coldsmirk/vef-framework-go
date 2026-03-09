package database

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/log"
)

func logDBVersion(provider DatabaseProvider, db *bun.DB, logger log.Logger) error {
	version, err := provider.QueryVersion(db)
	if err != nil {
		return wrapVersionQueryError(provider.Kind(), err)
	}

	logger.Infof("Database type: %s | Database version: %s", provider.Kind(), version)

	return nil
}

func setupBunDB(sqlDB *sql.DB, dialect schema.Dialect, opts *databaseOptions) *bun.DB {
	db := bun.NewDB(sqlDB, dialect, opts.BunOptions...)

	if opts.EnableQueryHook {
		addQueryHook(db, opts.Logger, opts.SQLGuardConfig)
	}

	db = db.WithNamedArg("Operator", "system")

	return db
}

func New(cfg *config.DataSourceConfig, options ...Option) (*bun.DB, error) {
	provider, exists := registry.provider(cfg.Kind)
	if !exists {
		return nil, newUnsupportedDBKindError(cfg.Kind)
	}

	sqlDB, dialect, err := provider.Connect(cfg)
	if err != nil || sqlDB == nil {
		return nil, err
	}

	opts := newDefaultOptions(cfg)
	opts.apply(options...)

	if opts.PoolConfig != nil {
		opts.PoolConfig.ApplyToDB(sqlDB)
	}

	return setupBunDB(sqlDB, dialect, opts), nil
}
