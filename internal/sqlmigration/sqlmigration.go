package sqlmigration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

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
	// "event inbox", ...). Used as the error prefix and as the
	// migration lock name.
	Label string
	// Kind is the database dialect.
	Kind config.DBKind
	// Scripts is the SQL bundle — typically a //go:embed FS. Run looks up
	// "scripts/<kind>.sql" within it.
	Scripts fs.FS
	// ExpectedTables names every table the migration must end up with.
	// The migration is skipped when all of them already exist.
	ExpectedTables []string
	// Pre is an optional list of steps that run before the
	// needs-migration probe (e.g. dropping obsolete tables left over
	// from earlier schema revisions). Each hook should be idempotent.
	Pre []func(ctx context.Context, db orm.DB) error
}

// Run executes the supplied Plan under the module's migration lock, so
// concurrently booting nodes provision a schema exactly once instead of
// racing the probe. It is a no-op when every expected table is already
// present and Pre hooks have completed without error.
func Run(ctx context.Context, db orm.DB, plan Plan) error {
	return WithLock(ctx, db, plan.Kind, plan.Label, func(ctx context.Context, db orm.DB) error {
		return runLocked(ctx, db, plan)
	})
}

func runLocked(ctx context.Context, db orm.DB, plan Plan) error {
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

// LoadScript returns the DDL script for the given dialect from the supplied
// script bundle. Exported so callers that need the SQL text (e.g.
// integration tests) can re-use the lookup convention.
func LoadScript(scripts fs.FS, kind config.DBKind) (string, error) {
	filename := "scripts/" + string(kind) + ".sql"

	data, err := fs.ReadFile(scripts, filename)
	if err != nil {
		return "", fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}

	return string(data), nil
}

func needsMigration(ctx context.Context, db orm.DB, plan Plan) (bool, error) {
	count, err := CountTables(ctx, db, plan.Kind, plan.ExpectedTables)
	if err != nil {
		return false, err
	}

	return count < len(plan.ExpectedTables), nil
}
