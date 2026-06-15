package schema

import (
	"context"
	"database/sql"
	"fmt"

	"ariga.io/atlas/sql/mysql"
	"ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/sqlite"
	"github.com/samber/lo"

	as "ariga.io/atlas/sql/schema"

	"github.com/coldsmirk/vef-framework-go/config"
)

// AtlasInspector performs read-only schema inspection backed by Atlas.
type AtlasInspector struct {
	inspector    as.Inspector
	db           *sql.DB
	schema       string
	inspectViews func(ctx context.Context) ([]*as.View, error)
}

// NewInspector creates a new Atlas Inspector for the given database connection.
func NewInspector(db *sql.DB, kind config.DBKind, schemaName string) (*AtlasInspector, error) {
	var (
		inspector as.Inspector
		schema    string
		err       error
	)

	i := &AtlasInspector{db: db}

	switch kind {
	case config.Postgres:
		inspector, err = postgres.Open(db)
		schema = lo.CoalesceOrEmpty(schemaName, "public")
		i.inspectViews = i.inspectPostgresViews

	case config.MySQL:
		inspector, err = mysql.Open(db)
		// For MySQL, schema is the database name, which is already set in the connection
		schema = ""
		i.inspectViews = i.inspectMySQLViews

	case config.SQLite:
		inspector, err = sqlite.Open(db)
		schema = "main"
		i.inspectViews = i.inspectSQLiteViews

	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedDBKind, kind)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open %s inspector: %w", kind, err)
	}

	i.inspector = inspector
	i.schema = schema

	return i, nil
}

// InspectSchema inspects the current database schema.
func (i *AtlasInspector) InspectSchema(ctx context.Context) (*as.Schema, error) {
	return i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Mode: as.InspectTables,
	})
}

// InspectTable inspects a specific table, returning ErrTableMissing if it does not exist.
func (i *AtlasInspector) InspectTable(ctx context.Context, name string) (*as.Table, error) {
	schema, err := i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Tables: []string{name},
	})
	if err != nil {
		return nil, err
	}

	if len(schema.Tables) == 0 {
		return nil, ErrTableMissing
	}

	return schema.Tables[0], nil
}

// InspectViews inspects all views in the current database schema.
func (i *AtlasInspector) InspectViews(ctx context.Context) ([]*as.View, error) {
	return i.inspectViews(ctx)
}

func (i *AtlasInspector) inspectPostgresViews(ctx context.Context) ([]*as.View, error) {
	return scanViews(ctx, i.db, "postgres", `
SELECT
	v.table_schema,
	v.table_name,
	COALESCE(v.view_definition, ''),
	COALESCE(pg_catalog.obj_description(c.oid, 'pg_class'), '')
FROM
	information_schema.views AS v
	JOIN pg_catalog.pg_namespace AS n ON n.nspname = v.table_schema
	JOIN pg_catalog.pg_class AS c ON c.relnamespace = n.oid AND c.relname = v.table_name
WHERE
	v.table_schema = $1
ORDER BY
	v.table_name`, []any{i.schema}, func(rows *sql.Rows) (*as.View, error) {
		var schemaName, name, definition, comment string
		if err := rows.Scan(&schemaName, &name, &definition, &comment); err != nil {
			return nil, fmt.Errorf("scan postgres view: %w", err)
		}

		columns, err := i.inspectPostgresViewColumns(ctx, schemaName, name)
		if err != nil {
			return nil, err
		}

		return newAtlasView(schemaName, name, definition, comment, columns), nil
	})
}

func (i *AtlasInspector) inspectPostgresViewColumns(ctx context.Context, schemaName, viewName string) ([]string, error) {
	return queryColumnNames(ctx, i.db, `
SELECT
	column_name
FROM
	information_schema.columns
WHERE
	table_schema = $1
	AND table_name = $2
ORDER BY
	ordinal_position`, schemaName, viewName)
}

func (i *AtlasInspector) inspectMySQLViews(ctx context.Context) ([]*as.View, error) {
	return scanViews(ctx, i.db, "mysql", `
SELECT
	v.table_schema,
	v.table_name,
	COALESCE(v.view_definition, ''),
	COALESCE(t.table_comment, '')
FROM
	information_schema.views AS v
	LEFT JOIN information_schema.tables AS t
		ON t.table_schema = v.table_schema AND t.table_name = v.table_name
WHERE
	v.table_schema = DATABASE()
ORDER BY
	v.table_name`, nil, func(rows *sql.Rows) (*as.View, error) {
		var schemaName, name, definition, comment string
		if err := rows.Scan(&schemaName, &name, &definition, &comment); err != nil {
			return nil, fmt.Errorf("scan mysql view: %w", err)
		}

		columns, err := i.inspectMySQLViewColumns(ctx, schemaName, name)
		if err != nil {
			return nil, err
		}

		return newAtlasView(schemaName, name, definition, comment, columns), nil
	})
}

func (i *AtlasInspector) inspectMySQLViewColumns(ctx context.Context, schemaName, viewName string) ([]string, error) {
	return queryColumnNames(ctx, i.db, `
SELECT
	column_name
FROM
	information_schema.columns
WHERE
	table_schema = ?
	AND table_name = ?
ORDER BY
	ordinal_position`, schemaName, viewName)
}

func (i *AtlasInspector) inspectSQLiteViews(ctx context.Context) ([]*as.View, error) {
	return scanViews(ctx, i.db, "sqlite", `
SELECT
	name,
	COALESCE(sql, '')
FROM
	sqlite_schema
WHERE
	type = 'view'
	AND name NOT LIKE 'sqlite_%'
ORDER BY
	name`, nil, func(rows *sql.Rows) (*as.View, error) {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			return nil, fmt.Errorf("scan sqlite view: %w", err)
		}

		columns, err := i.inspectSQLiteViewColumns(ctx, name)
		if err != nil {
			return nil, err
		}

		return newAtlasView(i.schema, name, definition, "", columns), nil
	})
}

func (i *AtlasInspector) inspectSQLiteViewColumns(ctx context.Context, viewName string) ([]string, error) {
	return queryColumnNames(ctx, i.db, `
SELECT
	name
FROM
	pragma_table_info(?)
ORDER BY
	cid`, viewName)
}

// scanViews runs the dialect view query and drives the shared iteration loop: it
// owns the *sql.Rows lifecycle (open and close), invokes the per-dialect build
// closure once per row, and wraps query/iteration errors with the dialect label.
// Each dialect supplies only its SQL, args, and a build closure that scans the
// row, resolves columns, and assembles the view.
func scanViews(ctx context.Context, db *sql.DB, dialect, query string, args []any, build func(*sql.Rows) (*as.View, error)) ([]*as.View, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s views: %w", dialect, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var views []*as.View
	for rows.Next() {
		view, err := build(rows)
		if err != nil {
			return nil, err
		}

		views = append(views, view)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s views: %w", dialect, err)
	}

	return views, nil
}

func queryColumnNames(ctx context.Context, db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query view columns: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan view column: %w", err)
		}

		columns = append(columns, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate view columns: %w", err)
	}

	return columns, nil
}

func newAtlasView(schemaName, name, definition, comment string, columns []string) *as.View {
	view := as.NewView(name, definition)
	if schemaName != "" {
		view.Schema = as.New(schemaName)
	}

	if comment != "" {
		view.Attrs = append(view.Attrs, &as.Comment{Text: comment})
	}

	for _, column := range columns {
		view.Columns = append(view.Columns, as.NewColumn(column))
	}

	return view
}
