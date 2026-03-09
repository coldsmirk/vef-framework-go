package schema

import (
	"context"
	"database/sql"
	"fmt"

	as "ariga.io/atlas/sql/schema"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// DefaultService is the default implementation of schema.Service.
type DefaultService struct {
	inspector Inspector
}

// NewService creates a new schema service.
func NewService(db *sql.DB, ds *config.DataSourceConfig) (schema.Service, error) {
	inspector, err := NewInspector(db, ds.Kind, ds.Schema)
	if err != nil {
		return nil, err
	}

	return &DefaultService{
		inspector: inspector,
	}, nil
}

// ListTables returns all tables in the current database/schema.
func (s *DefaultService) ListTables(ctx context.Context) ([]schema.Table, error) {
	result, err := s.inspector.InspectSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect schema: %w", err)
	}

	tables := make([]schema.Table, len(result.Tables))
	for i, t := range result.Tables {
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
	table, err := s.inspector.InspectTable(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect table: %w", err)
	}

	return s.convertTable(table), nil
}

// convertTable converts an Atlas table to a schema.TableSchema.
func (s *DefaultService) convertTable(t *as.Table) *schema.TableSchema {
	info := schema.TableSchema{
		Name:    t.Name,
		Columns: make([]schema.Column, len(t.Columns)),
	}

	if t.Schema != nil {
		info.Schema = t.Schema.Name
	}

	pkColumns := s.extractPrimaryKey(t, &info)
	s.convertColumns(t, &info, pkColumns)
	s.convertIndexes(t, &info)
	s.convertForeignKeys(t, &info)
	s.convertTableAttributes(t, &info)

	return &info
}

// extractPrimaryKey extracts primary key information and returns a set of primary key column names.
func (*DefaultService) extractPrimaryKey(t *as.Table, info *schema.TableSchema) map[string]bool {
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
func (*DefaultService) convertColumns(t *as.Table, info *schema.TableSchema, pkColumns map[string]bool) {
	for i, col := range t.Columns {
		colInfo := schema.Column{
			Name:            col.Name,
			Type:            col.Type.Raw,
			Nullable:        col.Type.Null,
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

// convertIndexes converts Atlas indexes to schema indexes and unique keys.
func (*DefaultService) convertIndexes(t *as.Table, info *schema.TableSchema) {
	for _, idx := range t.Indexes {
		columns := extractIndexColumns(idx.Parts)

		if idx.Unique {
			info.UniqueKeys = append(info.UniqueKeys, schema.UniqueKey{
				Name:    idx.Name,
				Columns: columns,
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
func (*DefaultService) convertForeignKeys(t *as.Table, info *schema.TableSchema) {
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
func (*DefaultService) convertTableAttributes(t *as.Table, info *schema.TableSchema) {
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
func extractIndexColumns(parts []*as.IndexPart) []string {
	columns := make([]string, len(parts))
	for i, part := range parts {
		if part.C != nil {
			columns[i] = part.C.Name
		}
	}

	return columns
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

// hasAutoIncrement checks if a column has auto-increment attribute.
func hasAutoIncrement(col *as.Column) bool {
	for _, attr := range col.Attrs {
		typeName := fmt.Sprintf("%T", attr)
		if typeName == "*mysql.AutoIncrement" || typeName == "*sqlite.AutoIncrement" {
			return true
		}
	}

	if col.Type == nil || col.Type.Raw == "" {
		return false
	}

	// PostgreSQL uses SERIAL types which show up in the type raw string
	switch col.Type.Raw {
	case "serial", "bigserial", "smallserial", "SERIAL", "BIGSERIAL", "SMALLSERIAL":
		return true
	default:
		return false
	}
}

// ListViews returns all views in the current database/schema.
func (s *DefaultService) ListViews(ctx context.Context) ([]schema.View, error) {
	views, err := s.inspector.InspectViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect views: %w", err)
	}

	result := make([]schema.View, len(views))
	for i, v := range views {
		view := schema.View{
			Name:         v.Name,
			Definition:   v.Def,
			Materialized: v.Materialized(),
			Columns:      extractColumnNames(v.Columns),
			Comment:      extractComment(v.Attrs),
		}
		if v.Schema != nil {
			view.Schema = v.Schema.Name
		}

		result[i] = view
	}

	return result, nil
}

// extractColumnNames extracts column names from a slice of columns.
func extractColumnNames(columns []*as.Column) []string {
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = col.Name
	}

	return names
}

// ListTriggers returns all triggers in the current database/schema.
func (s *DefaultService) ListTriggers(ctx context.Context) ([]schema.Trigger, error) {
	triggers, err := s.inspector.InspectTriggers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect triggers: %w", err)
	}

	result := make([]schema.Trigger, len(triggers))
	for i, t := range triggers {
		trigger := schema.Trigger{
			Name:       t.Name,
			ActionTime: string(t.ActionTime),
			ForEachRow: t.For == as.TriggerForRow,
			Body:       t.Body,
			Events:     extractEventNames(t.Events),
		}
		if t.Table != nil {
			trigger.Table = t.Table.Name
		}

		if t.View != nil {
			trigger.View = t.View.Name
		}

		result[i] = trigger
	}

	return result, nil
}

// extractEventNames extracts event names from a slice of trigger events.
func extractEventNames(events []as.TriggerEvent) []string {
	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}

	return names
}
