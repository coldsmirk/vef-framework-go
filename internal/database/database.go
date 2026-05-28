package database

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/logx"
)

func logDBVersion(provider DatabaseProvider, db *bun.DB, logger logx.Logger) error {
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
