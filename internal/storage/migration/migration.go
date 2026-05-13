package migration

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var errUnsupportedDBKind = errors.New("unsupported database kind")

//go:embed scripts/*.sql
var scripts embed.FS

// expectedTables lists all tables the storage module requires.
// Used to check whether migration is needed.
var expectedTables = []string{
	"sys_storage_upload_claim",
	"sys_storage_upload_part",
	"sys_storage_pending_delete",
}

// Migrate runs the storage module's DDL migration for the given database kind.
// It checks whether all expected tables exist and skips if they do.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	needed, err := needsMigration(ctx, db, kind)
	if err != nil {
		return fmt.Errorf("check migration status: %w", err)
	}

	if !needed {
		return nil
	}

	sql, err := GetMigrationSQL(kind)
	if err != nil {
		return err
	}

	if _, err = db.NewRaw(sql).Exec(ctx); err != nil {
		return fmt.Errorf("execute storage migration: %w", err)
	}

	return nil
}

// GetMigrationSQL returns the migration SQL script for the given database kind.
func GetMigrationSQL(kind config.DBKind) (string, error) {
	filename := "scripts/" + string(kind) + ".sql"

	data, err := scripts.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("%w %q for storage migration", errUnsupportedDBKind, kind)
	}

	return string(data), nil
}

// needsMigration checks whether any expected table is missing from the database.
func needsMigration(ctx context.Context, db orm.DB, kind config.DBKind) (bool, error) {
	query := buildTableCountQuery(kind)
	if query == "" {
		return false, fmt.Errorf("%w %q", errUnsupportedDBKind, kind)
	}

	var count int
	if err := db.NewRaw(query, bun.Tuple(expectedTables)).Scan(ctx, &count); err != nil {
		return false, err
	}

	return count < len(expectedTables), nil
}

// buildTableCountQuery builds a SQL query that counts how many expected tables
// already exist in the database. Table names are hardcoded constants (no injection risk).
func buildTableCountQuery(kind config.DBKind) string {
	switch kind {
	case config.Postgres:
		return "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ?"
	case config.MySQL:
		return "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ?"
	case config.SQLite:
		return "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ?"
	default:
		return ""
	}
}
