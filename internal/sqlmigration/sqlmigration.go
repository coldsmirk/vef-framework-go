package sqlmigration

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrUnsupportedDBKind indicates the configured database dialect has
// no migration script or schema lookup query for the supplied Plan.
var ErrUnsupportedDBKind = errors.New("sqlmigration: unsupported database kind")

// Plan describes one module's migration. Label appears in error
// messages so logs identify which module is being migrated.
type Plan struct {
	// Label is a short, human-readable identifier ("storage",
	// "event outbox", ...). Used as the error prefix.
	Label string
	// Kind is the database dialect.
	Kind config.DBKind
	// Scripts is the embedded SQL bundle. Run looks up
	// "scripts/<kind>.sql" within this FS.
	Scripts embed.FS
	// ExpectedTables names every table the migration must end up with.
	// The migration is skipped when all of them already exist.
	ExpectedTables []string
	// Pre is an optional list of steps that run before the
	// needs-migration probe (e.g. dropping obsolete tables left over
	// from earlier schema revisions). Each hook should be idempotent.
	Pre []func(ctx context.Context, db orm.DB) error
}

// Run executes the supplied Plan. It is a no-op when every expected
// table is already present and Pre hooks have completed without error.
func Run(ctx context.Context, db orm.DB, plan Plan) error {
	for _, hook := range plan.Pre {
		if err := hook(ctx, db); err != nil {
			return fmt.Errorf("%s pre-migration: %w", plan.Label, err)
		}
	}

	needed, err := needsMigration(ctx, db, plan)
	if err != nil {
		return fmt.Errorf("%s: check migration status: %w", plan.Label, err)
	}

	if !needed {
		return nil
	}

	sql, err := LoadScript(plan.Scripts, plan.Kind)
	if err != nil {
		return fmt.Errorf("%s: %w", plan.Label, err)
	}

	if _, err := db.NewRaw(sql).Exec(ctx); err != nil {
		return fmt.Errorf("%s: execute migration: %w", plan.Label, err)
	}

	return nil
}

// LoadScript returns the DDL script for the given dialect from the
// supplied embedded FS. Exported so callers that need the SQL text
// (e.g. integration tests) can re-use the lookup convention.
func LoadScript(scripts embed.FS, kind config.DBKind) (string, error) {
	filename := "scripts/" + string(kind) + ".sql"

	data, err := scripts.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}

	return string(data), nil
}

func needsMigration(ctx context.Context, db orm.DB, plan Plan) (bool, error) {
	query := tableCountQuery(plan.Kind)
	if query == "" {
		return false, fmt.Errorf("%w %q", ErrUnsupportedDBKind, plan.Kind)
	}

	var count int
	if err := db.NewRaw(query, bun.Tuple(plan.ExpectedTables)).Scan(ctx, &count); err != nil {
		return false, err
	}

	return count < len(plan.ExpectedTables), nil
}

// tableCountQuery returns the dialect-specific COUNT query used to
// determine whether the migration is needed. Table names are bound via
// bun.Tuple so there is no injection risk.
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
