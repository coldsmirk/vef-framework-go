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

// expectedTables lists all tables the approval module requires.
// Used to check whether migration is needed.
var expectedTables = []string{
	"apv_flow_category",
	"apv_flow",
	"apv_flow_initiator",
	"apv_flow_version",
	"apv_flow_node",
	"apv_flow_node_assignee",
	"apv_flow_node_cc",
	"apv_flow_edge",
	"apv_flow_form_field",
	"apv_instance",
	"apv_task",
	"apv_action_log",
	"apv_parallel_record",
	"apv_cc_record",
	"apv_delegation",
	"apv_form_snapshot",
	"apv_event_outbox",
	"apv_urge_record",
}

// Migrate runs the approval module's DDL migration for the given database kind.
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
		return fmt.Errorf("execute approval migration: %w", err)
	}

	return nil
}

// GetMigrationSQL returns the migration SQL script for the given database kind.
func GetMigrationSQL(kind config.DBKind) (string, error) {
	filename := "scripts/" + string(kind) + ".sql"

	data, err := scripts.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("%w %q for approval migration", errUnsupportedDBKind, kind)
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
