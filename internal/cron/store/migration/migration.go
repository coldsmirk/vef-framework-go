package migration

import (
	"context"
	"embed"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

//go:embed scripts/*.sql
var scripts embed.FS

// expectedTables lists all tables the cron store requires.
var expectedTables = []string{
	"crn_schedule",
	"crn_fire_request",
	"crn_run",
}

// Migrate provisions a fresh cron store schema and verifies its capabilities.
// Existing incompatible tables are never altered or repaired.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	if err := withMigrationLock(ctx, db, kind, func(ctx context.Context, lockedDB orm.DB) error {
		return provisionFresh(ctx, lockedDB, kind)
	}); err != nil {
		return err
	}

	return Verify(ctx, db, kind)
}

func provisionFresh(ctx context.Context, db orm.DB, kind config.DBKind) error {
	existing := 0
	for _, table := range expectedTables {
		exists, err := sqlmigration.TableExists(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("cron store: check table %s: %w", table, err)
		}

		if exists {
			existing++
		}
	}

	switch existing {
	case 0:
		sql, err := sqlmigration.LoadScript(scripts, kind)
		if err != nil {
			return fmt.Errorf("cron store: %w", err)
		}

		if _, err := db.NewRaw(sql).Exec(ctx); err != nil {
			return fmt.Errorf("cron store: execute migration: %w", err)
		}

	case len(expectedTables):
		// A complete existing schema is verification-only.

	default:
		return outdated(
			"partial schema contains %d of %d required tables",
			existing,
			len(expectedTables),
		)
	}

	return nil
}
