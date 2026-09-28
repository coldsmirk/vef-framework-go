package sqlmigration

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AddedColumn is a column introduced after its table first shipped. The
// module's CREATE script still declares it, so a fresh schema gets it from
// there; Run adds it in place to an existing table that lacks it. Only a
// column every existing row can take by default qualifies — a type, key or
// constraint change to an existing table cannot be applied this way.
type AddedColumn struct {
	// Table is the table the column belongs to.
	Table string
	// Name is the column name.
	Name string
	// Definition is the type-and-constraints clause, portable across every
	// dialect the module ships a script for, e.g. "INTEGER NOT NULL DEFAULT 0".
	Definition string
	// Comment is the column comment the scripts declare, empty for none. It is
	// applied the way each dialect's script applies it, so an upgraded table
	// matches a freshly created one.
	Comment string
}

// Statements renders the DDL adding the column on the given dialect — also
// the remedy a module's schema verification can quote when the column is
// missing and automatic migration is off.
func (c AddedColumn) Statements(kind config.DBKind) []string {
	add := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.Table, c.Name, c.Definition)
	if c.Comment == "" {
		return []string{add}
	}

	comment := "'" + strings.ReplaceAll(c.Comment, "'", "''") + "'"

	switch kind {
	case config.MySQL:
		return []string{add + " COMMENT " + comment}
	case config.Postgres:
		return []string{add, fmt.Sprintf("COMMENT ON COLUMN %s.%s IS %s", c.Table, c.Name, comment)}
	default:
		return []string{add}
	}
}

// MissingColumns returns, in order, the columns absent from their table. A
// column whose table does not exist is not missing: the script creating the
// table declares it.
func MissingColumns(ctx context.Context, db orm.DB, kind config.DBKind, columns []AddedColumn) ([]AddedColumn, error) {
	var missing []AddedColumn

	for _, column := range columns {
		exists, err := TableExists(ctx, db, kind, column.Table)
		if err != nil {
			return nil, fmt.Errorf("check table %s: %w", column.Table, err)
		}

		if !exists {
			continue
		}

		present, err := LoadTableColumns(ctx, db, kind, column.Table)
		if err != nil {
			return nil, fmt.Errorf("load columns for %s: %w", column.Table, err)
		}

		if _, ok := present[column.Name]; !ok {
			missing = append(missing, column)
		}
	}

	return missing, nil
}

// addMissingColumns adds every Plan.AddedColumns entry an existing table
// lacks. It runs under the migration lock, so a replica booting concurrently
// finds the columns already present.
func addMissingColumns(ctx context.Context, db orm.DB, plan Plan) error {
	missing, err := MissingColumns(ctx, db, plan.Kind, plan.AddedColumns)
	if err != nil {
		return err
	}

	for _, column := range missing {
		for _, statement := range column.Statements(plan.Kind) {
			if _, err := db.NewRaw(statement).Exec(ctx); err != nil {
				return fmt.Errorf("add column %s.%s: %w", column.Table, column.Name, err)
			}
		}
	}

	return nil
}
