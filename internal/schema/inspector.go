package schema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ariga.io/atlas/sql/mysql"
	"ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/sqlite"
	"github.com/samber/lo"

	as "ariga.io/atlas/sql/schema"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Inspector wraps Atlas inspection capabilities for read-only schema inspection.
type Inspector interface {
	// InspectSchema inspects the current database schema.
	InspectSchema(ctx context.Context) (*as.Schema, error)
	// InspectTable inspects a specific table.
	InspectTable(ctx context.Context, name string) (*as.Table, error)
	// InspectViews inspects all views in the current database schema.
	InspectViews(ctx context.Context) ([]*as.View, error)
	// InspectTriggers inspects all triggers in the current database schema.
	InspectTriggers(ctx context.Context) ([]*as.Trigger, error)
}

var ErrUnsupportedDBKind = errors.New("unsupported database type")

type AtlasInspector struct {
	inspector as.Inspector
	schema    string
}

// NewInspector creates a new Atlas Inspector for the given database connection.
func NewInspector(db *sql.DB, kind config.DBKind, schemaName string) (Inspector, error) {
	var (
		inspector as.Inspector
		schema    string
		err       error
	)

	switch kind {
	case config.Postgres:
		inspector, err = postgres.Open(db)
		schema = lo.CoalesceOrEmpty(schemaName, "public")

	case config.MySQL:
		inspector, err = mysql.Open(db)
		// For MySQL, schema is the database name, which is already set in the connection
		schema = ""

	case config.SQLite:
		inspector, err = sqlite.Open(db)
		schema = "main"

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDBKind, kind)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open %s inspector: %w", kind, err)
	}

	return &AtlasInspector{
		inspector: inspector,
		schema:    schema,
	}, nil
}

func (i *AtlasInspector) InspectSchema(ctx context.Context) (*as.Schema, error) {
	return i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Mode: as.InspectTables,
	})
}

func (i *AtlasInspector) InspectTable(ctx context.Context, name string) (*as.Table, error) {
	schema, err := i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Tables: []string{name},
	})
	if err != nil {
		return nil, err
	}

	if len(schema.Tables) == 0 {
		return nil, ErrTableNotFound
	}

	return schema.Tables[0], nil
}

func (i *AtlasInspector) InspectViews(ctx context.Context) ([]*as.View, error) {
	schema, err := i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Mode: as.InspectViews,
	})
	if err != nil {
		return nil, err
	}

	return schema.Views, nil
}

func (i *AtlasInspector) InspectTriggers(ctx context.Context) ([]*as.Trigger, error) {
	// Triggers are attached to tables and views, so we need to inspect both
	schema, err := i.inspector.InspectSchema(ctx, i.schema, &as.InspectOptions{
		Mode: as.InspectTables | as.InspectViews | as.InspectTriggers,
	})
	if err != nil {
		return nil, err
	}

	var triggers []*as.Trigger

	// Collect triggers from tables
	for _, t := range schema.Tables {
		triggers = append(triggers, t.Triggers...)
	}

	// Collect triggers from views
	for _, v := range schema.Views {
		triggers = append(triggers, v.Triggers...)
	}

	return triggers, nil
}
