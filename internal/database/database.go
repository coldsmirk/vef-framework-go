package database

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/config"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// logger is the database package's default logger, used as the query-hook
// logger when a caller does not supply one via WithLogger.
var logger = ilogx.Named("database")

// LogVersion resolves the dialect provider for kind, logs the connected server
// version, and emits a ready line. It no-ops when kind has no registered
// provider. The orm data source start hook calls it so dialect introspection
// stays inside the database layer.
func LogVersion(kind config.DBKind, db *bun.DB, logger logx.Logger) error {
	provider, ok := registry.provider(kind)
	if !ok {
		return nil
	}

	if err := logDBVersion(provider, db, logger); err != nil {
		return err
	}

	logger.Infof("Database client started successfully: %s", provider.Kind())

	return nil
}

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
