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
	if !SupportsKind(kind) {
		return nil
	}

	version, err := Version(ctx, kind, db)
	if err != nil {
		return err
	}

	logger.Infof("Database type: %s | Database version: %s", kind, version)
	logger.Infof("Database client started successfully: %s", kind)

	return nil
}

// Version resolves the provider for kind and returns the connected server's
// version string. It returns an unsupported-kind error when no provider is
// registered for kind.
func Version(ctx context.Context, kind config.DBKind, db *sql.DB) (string, error) {
	provider, ok := registry.lookup(kind)
	if !ok {
		return "", newUnsupportedDBKindError(kind)
	}

	version, err := provider.Version(ctx, db)
	if err != nil {
		return "", wrapVersionQueryError(kind, err)
	}

	return version, nil
}
