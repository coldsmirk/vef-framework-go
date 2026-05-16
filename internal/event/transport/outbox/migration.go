package outbox

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

// expectedTables lists the tables the outbox transport requires.
var expectedTables = []string{"sys_event_outbox"}

// Migrate runs the outbox transport's DDL migration for the supplied
// database kind. The migration is idempotent and a no-op when every
// expected table already exists.
func Migrate(ctx context.Context, db orm.DB, kind config.DBKind) error {
	needed, err := needsMigration(ctx, db, kind)
	if err != nil {
		return fmt.Errorf("event outbox: check migration status: %w", err)
	}

	if !needed {
		return nil
	}

	sql, err := loadScript(kind)
	if err != nil {
		return err
	}

	if _, err := db.NewRaw(sql).Exec(ctx); err != nil {
		return fmt.Errorf("event outbox: execute migration: %w", err)
	}

	return nil
}

func loadScript(kind config.DBKind) (string, error) {
	filename := "scripts/" + string(kind) + ".sql"

	data, err := scripts.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("%w %q for sys_event_outbox", errUnsupportedDBKind, kind)
	}

	return string(data), nil
}

func needsMigration(ctx context.Context, db orm.DB, kind config.DBKind) (bool, error) {
	query := tableCountQuery(kind)
	if query == "" {
		return false, fmt.Errorf("%w %q", errUnsupportedDBKind, kind)
	}

	var count int
	if err := db.NewRaw(query, bun.Tuple(expectedTables)).Scan(ctx, &count); err != nil {
		return false, err
	}

	return count < len(expectedTables), nil
}

func tableCountQuery(kind config.DBKind) string {
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
