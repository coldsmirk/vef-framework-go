package database

import (
	"context"
	"database/sql"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// LogVersion resolves the provider for kind, logs the connected server version,
// and emits a ready line. It no-ops when kind has no registered provider. The
// orm data source start hook calls it so version introspection stays inside the
// database layer.
func LogVersion(ctx context.Context, kind config.DBKind, db *sql.DB, logger logx.Logger) error {
	provider, ok := registry.lookup(kind)
	if !ok {
		return nil
	}

	if err := logDBVersion(ctx, provider, db, logger); err != nil {
		return err
	}

	logger.Infof("Database client started successfully: %s", provider.Kind())

	return nil
}

func logDBVersion(ctx context.Context, provider Provider, db *sql.DB, logger logx.Logger) error {
	version, err := provider.Version(ctx, db)
	if err != nil {
		return wrapVersionQueryError(provider.Kind(), err)
	}

	logger.Infof("Database type: %s | Database version: %s", provider.Kind(), version)

	return nil
}
