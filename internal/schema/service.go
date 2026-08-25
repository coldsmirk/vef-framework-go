package schema

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"ariga.io/atlas/sql/mysql"
	"ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/sqlite"

	as "ariga.io/atlas/sql/schema"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// DefaultService is the default implementation of schema.Service.
type DefaultService struct {
	db     *sql.DB
	kind   config.DBKind
	schema string

	mu        sync.Mutex
	inspector *AtlasInspector
}

// NewService creates a new schema service backed by the primary data source.
// Multi-data-source schema reflection is intentionally out of scope for v1;
// callers that need to introspect a non-primary source should inject
// datasource.Registry, fetch the connection, and drive atlas inspection
// themselves.
//
// The dialect is validated here but the Atlas inspector is opened on first
// use, because opening it queries the server. A constructor that dials makes
// the whole graph unbuildable without a reachable database — which is the same
// reason the data source module opens without blocking and pings from a start
// hook, and what lets the API surface be described offline.
func NewService(db *sql.DB, dataSources *config.DataSourcesConfig) (schema.Service, error) {
	primary := dataSources.Primary()
	if !isSupportedDBKind(primary.Kind) {
		return nil, fmt.Errorf("%w: %s", errUnsupportedDBKind, primary.Kind)
	}

	return &DefaultService{db: db, kind: primary.Kind, schema: primary.Schema}, nil
}

// isSupportedDBKind reports whether schema inspection can run against a kind.
func isSupportedDBKind(kind config.DBKind) bool {
	switch kind {
	case config.Postgres, config.MySQL, config.SQLite:
		return true
	default:
		return false
	}
}

// resolve opens the Atlas inspector on the first inspection and reuses it
// afterwards.
//
// Only success is remembered. Opening queries the server for its version, so it
// can fail for reasons that pass — a restart during a rolling deploy, a moment
// of pool exhaustion — and caching that failure would leave schema inspection
// permanently broken for the life of the process, recoverable only by a
// restart. That is strictly worse than the eager construction this replaced,
// where a failure at least took the boot down and let the supervisor retry.
func (s *DefaultService) resolve() (*AtlasInspector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inspector != nil {
		return s.inspector, nil
	}

	inspector, err := NewInspector(s.db, s.kind, s.schema)
	if err != nil {
		return nil, err
	}

	s.inspector = inspector

	return inspector, nil
}

// ListTables returns all tables in the current database/schema.
func (s *DefaultService) ListTables(ctx context.Context) ([]schema.Table, error) {
	inspector, err := s.resolve()
	if err != nil {
		return nil, err
	}

	inspected, err := inspector.InspectSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect schema: %w", err)
	}

	tables := make([]schema.Table, len(inspected.Tables))
	for i, t := range inspected.Tables {
		table := schema.Table{
			Name:    t.Name,
			Comment: extractComment(t.Attrs),
		}
		if t.Schema != nil {
			table.Schema = t.Schema.Name
		}

		tables[i] = table
	}

	return tables, nil
}

// GetTableSchema returns detailed structure information about a specific table.
func (s *DefaultService) GetTableSchema(ctx context.Context, name string) (*schema.TableSchema, error) {
	inspector, err := s.resolve()
	if err != nil {
		return nil, err
	}

	table, err := inspector.InspectTable(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect table: %w", err)
	}

	return convertTable(table), nil
}

// convertTable converts an Atlas table to a schema.TableSchema.
func convertTable(t *as.Table) *schema.TableSchema {
	info := schema.TableSchema{
		Name:    t.Name,
		Columns: make([]schema.Column, len(t.Columns)),
	}

	if t.Schema != nil {
		info.Schema = t.Schema.Name
	}

	pkColumns := extractPrimaryKey(t, &info)
	convertColumns(t, &info, pkColumns)
	convertIndexes(t, &info)
	convertForeignKeys(t, &info)
	convertTableAttributes(t, &info)

	return &info
}

// extractPrimaryKey extracts primary key information and returns a set of primary key column names.
func extractPrimaryKey(t *as.Table, info *schema.TableSchema) map[string]bool {
	pkColumns := make(map[string]bool)

	if t.PrimaryKey == nil {
		return pkColumns
	}

	pkCols := make([]string, len(t.PrimaryKey.Parts))
	for i, part := range t.PrimaryKey.Parts {
		if part.C != nil {
			pkColumns[part.C.Name] = true
			pkCols[i] = part.C.Name
		}
	}

	if len(pkCols) > 0 {
		info.PrimaryKey = &schema.PrimaryKey{
			Name:    t.PrimaryKey.Name,
			Columns: pkCols,
		}
	}

	return pkColumns
}

// convertColumns converts Atlas columns to schema columns.
func convertColumns(t *as.Table, info *schema.TableSchema, pkColumns map[string]bool) {
	for i, col := range t.Columns {
		colInfo := schema.Column{
			Name:            col.Name,
			Type:            col.Type.Raw,
			Nullable:        col.Type.Null,
			MaxLength:       characterMaxLength(col.Type.Type),
			IsPrimaryKey:    pkColumns[col.Name],
			IsAutoIncrement: hasAutoIncrement(col),
			Comment:         extractComment(col.Attrs),
		}

		if col.Default != nil {
			if raw, ok := col.Default.(*as.RawExpr); ok {
				colInfo.Default = raw.X
			}
		}

		info.Columns[i] = colInfo
	}
}

// characterMaxLength reports a character column's declared bound, or zero for
// any other type. Atlas normalizes the length out of the raw type name on some
// dialects, so the typed column is the only place it is reliably available.
func characterMaxLength(columnType as.Type) int {
	if text, ok := columnType.(*as.StringType); ok {
		return text.Size
	}

	return 0
}

// convertIndexes converts Atlas indexes to schema indexes and unique keys.
func convertIndexes(t *as.Table, info *schema.TableSchema) {
	for _, idx := range t.Indexes {
		columns, hasExpressions := extractIndexColumns(idx.Parts)

		if idx.Unique {
			info.UniqueKeys = append(info.UniqueKeys, schema.UniqueKey{
				Name:           idx.Name,
				Columns:        columns,
				Predicate:      extractIndexPredicate(idx.Attrs),
				HasExpressions: hasExpressions,
			})
		} else {
			info.Indexes = append(info.Indexes, schema.Index{
				Name:    idx.Name,
				Columns: columns,
			})
		}
	}
}

// convertForeignKeys converts Atlas foreign keys to schema foreign keys.
func convertForeignKeys(t *as.Table, info *schema.TableSchema) {
	for _, fk := range t.ForeignKeys {
		fkInfo := schema.ForeignKey{
			Name:       fk.Symbol,
			Columns:    make([]string, len(fk.Columns)),
			RefColumns: make([]string, len(fk.RefColumns)),
			OnUpdate:   referentialActionToString(fk.OnUpdate),
			OnDelete:   referentialActionToString(fk.OnDelete),
		}

		if fk.RefTable != nil {
			fkInfo.RefTable = fk.RefTable.Name
		}

		for i, col := range fk.Columns {
			fkInfo.Columns[i] = col.Name
		}

		for i, col := range fk.RefColumns {
			fkInfo.RefColumns[i] = col.Name
		}

		info.ForeignKeys = append(info.ForeignKeys, fkInfo)
	}
}

// convertTableAttributes converts Atlas table attributes to schema attributes.
func convertTableAttributes(t *as.Table, info *schema.TableSchema) {
	for _, attr := range t.Attrs {
		switch a := attr.(type) {
		case *as.Comment:
			info.Comment = a.Text
		case *as.Check:
			info.Checks = append(info.Checks, schema.Check{
				Name: a.Name,
				Expr: a.Expr,
			})
		}
	}
}

// extractComment extracts comment text from a slice of attributes.
func extractComment(attrs []as.Attr) string {
	for _, attr := range attrs {
		if comment, ok := attr.(*as.Comment); ok {
			return comment.Text
		}
	}

	return ""
}

// extractIndexColumns extracts column names from index parts.
func extractIndexColumns(parts []*as.IndexPart) ([]string, bool) {
	columns := make([]string, len(parts))
	hasExpressions := false

	for i, part := range parts {
		if part.C != nil {
			columns[i] = part.C.Name
		} else {
			hasExpressions = true
		}
	}

	return columns, hasExpressions
}

func extractIndexPredicate(attrs []as.Attr) string {
	for _, attr := range attrs {
		switch predicate := attr.(type) {
		case *postgres.IndexPredicate:
			return predicate.P
		case *sqlite.IndexPredicate:
			return predicate.P
		}
	}

	return ""
}

// referentialActionToString converts a referential action to string.
func referentialActionToString(action as.ReferenceOption) string {
	switch action {
	case as.Cascade:
		return "CASCADE"
	case as.SetNull:
		return "SET NULL"
	case as.SetDefault:
		return "SET DEFAULT"
	case as.Restrict:
		return "RESTRICT"
	case as.NoAction:
		return "NO ACTION"
	default:
		return ""
	}
}

// hasAutoIncrement reports whether a column auto-generates its value: MySQL
// AUTO_INCREMENT, SQLite AUTOINCREMENT, or PostgreSQL GENERATED ... AS IDENTITY.
func hasAutoIncrement(col *as.Column) bool {
	for _, attr := range col.Attrs {
		switch attr.(type) {
		case *mysql.AutoIncrement, *sqlite.AutoIncrement, *postgres.Identity:
			return true
		}
	}

	// PostgreSQL SERIAL pseudo-types surface only in the raw type string.
	// Atlas normalizes these to the lowercase driver constants, so only the
	// lowercase forms can ever appear here.
	switch col.Type.Raw {
	case "serial", "bigserial", "smallserial":
		return true
	default:
		return false
	}
}

// ListViews returns all views in the current database/schema.
func (s *DefaultService) ListViews(ctx context.Context) ([]schema.View, error) {
	inspector, err := s.resolve()
	if err != nil {
		return nil, err
	}

	views, err := inspector.InspectViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect views: %w", err)
	}

	converted := make([]schema.View, len(views))
	for i, v := range views {
		view := schema.View{
			Name:       v.Name,
			Definition: v.Def,
			Columns:    extractColumnNames(v.Columns),
			Comment:    extractComment(v.Attrs),
		}
		if v.Schema != nil {
			view.Schema = v.Schema.Name
		}

		converted[i] = view
	}

	return converted, nil
}

// extractColumnNames extracts column names from a slice of columns.
func extractColumnNames(columns []*as.Column) []string {
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = col.Name
	}

	return names
}
