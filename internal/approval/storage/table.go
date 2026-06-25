package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// TableStorage is the StorageTable strategy. At publish it generates a
// dedicated physical table for the version's form schema and records the
// generated structure in apv_form_table / apv_form_table_column. At write it
// projects an instance's form data into that table as exactly one row.
//
// The strategy is dialect-aware: DDL and DML are generated for the primary
// data source's dialect (the same dialect the migration scripts target), mirrored
// from how migration.go selects scripts by dataSources.Primary().Kind.
//
// Security: every physical table and column name is a validated SQL identifier
// (approval.ValidateBusinessIdentifier) before it reaches a DDL/DML string, and
// every form value is bound as a `?` argument — never interpolated. See ddl.go
// for the identifier pipeline.
type TableStorage struct {
	kind config.DBKind
}

// NewTableStorage constructs a TableStorage for the given database dialect.
func NewTableStorage(kind config.DBKind) *TableStorage {
	return &TableStorage{kind: kind}
}

// ProvisionTable creates the version's physical form table. It runs outside the
// publish transaction (DDL implicitly commits the in-flight transaction on
// MySQL, which would break the publish's atomicity) and is idempotent: CREATE
// TABLE IF NOT EXISTS makes a republish — or a retry after a rolled-back publish
// that left the table behind — a no-op at the DDL level. The table name and
// columns are derived from the version's form schema; see ddl.go.
func (s *TableStorage) ProvisionTable(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error {
	tableName, err := buildPhysicalTableName(flow.Code, version.ID)
	if err != nil {
		return err
	}

	specs, err := buildColumnSpecs(s.kind, version.FormSchema)
	if err != nil {
		return err
	}

	createSQL, err := renderCreateTable(s.kind, tableName, specs)
	if err != nil {
		return err
	}

	if _, err := db.NewRaw(createSQL).Exec(ctx); err != nil {
		return fmt.Errorf("create form table %q: %w", tableName, err)
	}

	return nil
}

// RecordMetadata records the generated table's structure in apv_form_table /
// apv_form_table_column. It runs inside the publish transaction so the metadata
// commits or rolls back with the version's published state. It is idempotent: if
// metadata already exists for the version (a republish or a retry), it returns
// without re-inserting. The table name and column specs are recomputed from the
// version's form schema — the same deterministic derivation ProvisionTable used,
// so the recorded metadata always describes the physical table.
func (s *TableStorage) RecordMetadata(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error {
	exists, err := db.NewSelect().
		Model((*approval.FormTable)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("version_id", version.ID)
		}).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("check existing form table metadata: %w", err)
	}

	if exists {
		return nil
	}

	tableName, err := buildPhysicalTableName(flow.Code, version.ID)
	if err != nil {
		return err
	}

	specs, err := buildColumnSpecs(s.kind, version.FormSchema)
	if err != nil {
		return err
	}

	formTable := &approval.FormTable{
		FlowID:            version.FlowID,
		VersionID:         version.ID,
		PhysicalTableName: tableName,
	}
	if _, err := db.NewInsert().Model(formTable).Exec(ctx); err != nil {
		return fmt.Errorf("insert form table metadata: %w", err)
	}

	columns := make([]approval.FormTableColumn, 0, len(specs))
	for _, spec := range specs {
		columns = append(columns, approval.FormTableColumn{
			FormTableID:    formTable.ID,
			ColumnName:     spec.Name,
			ColumnType:     spec.Type,
			IsNullable:     spec.IsNullable,
			SourceFieldKey: spec.SourceFieldKey,
			SortOrder:      spec.SortOrder,
		})
	}

	if _, err := db.NewInsert().Model(&columns).Exec(ctx); err != nil {
		return fmt.Errorf("insert form table column metadata: %w", err)
	}

	return nil
}

// Write projects an instance's form data into the version's physical form table
// as exactly one row. It is idempotent per instance: any existing row for the
// instance is deleted before the fresh row is inserted, so the first projection
// (at start) and every later one (at resubmit) leave the table reflecting the
// instance's current form data — never an accumulating history. The instance_id
// UNIQUE constraint backs this contract at the database level.
//
// The physical table name and the set of generated columns are read from the
// metadata recorded at publish (the single source of truth), so Write and
// OnVersionPublished can never disagree on identifiers. Only columns sourced
// from a form field are populated from formData; instance_id is set from the
// argument and created_at defaults at the database. Every value is bound as a
// `?` argument.
func (*TableStorage) Write(ctx context.Context, db orm.DB, _ *approval.Flow, version *approval.FlowVersion, instanceID string, formData map[string]any) error {
	var formTable approval.FormTable
	if err := db.NewSelect().
		Model(&formTable).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("version_id", version.ID)
		}).
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			// In table mode the metadata must exist (publish creates it). Its
			// absence means an instance is being created against a version that
			// was never (successfully) published in table mode — fail loudly
			// rather than silently dropping the projection.
			return fmt.Errorf("%w: version %q", ErrFormTableMetadataMissing, version.ID)
		}

		return fmt.Errorf("load form table metadata: %w", err)
	}

	var columns []approval.FormTableColumn
	if err := db.NewSelect().
		Model(&columns).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("form_table_id", formTable.ID)
		}).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return fmt.Errorf("load form table columns: %w", err)
	}

	// Replace, never append: clear any prior projection for this instance so a
	// resubmit refreshes the single row instead of stacking duplicates. On the
	// first write (start) this deletes nothing.
	deleteSQL, err := buildDelete(formTable.PhysicalTableName)
	if err != nil {
		return err
	}

	if _, err := db.NewRaw(deleteSQL, instanceID).Exec(ctx); err != nil {
		return fmt.Errorf("clear existing form row in %q: %w", formTable.PhysicalTableName, err)
	}

	insertSQL, args, err := buildInsert(formTable.PhysicalTableName, instanceID, columns, formData)
	if err != nil {
		return err
	}

	if _, err := db.NewRaw(insertSQL, args...).Exec(ctx); err != nil {
		return fmt.Errorf("insert form row into %q: %w", formTable.PhysicalTableName, err)
	}

	return nil
}

// buildDelete composes the DELETE that clears an instance's existing projection
// row before a fresh one is written. The table name comes from metadata
// (validated when written) and is re-validated here as defense-in-depth before
// interpolation; instance_id is returned to the caller as a bind argument.
func buildDelete(tableName string) (string, error) {
	if err := approval.ValidateBusinessIdentifier(tableName); err != nil {
		return "", fmt.Errorf("%w: %q: %w", ErrInvalidGeneratedIdentifier, tableName, err)
	}

	return fmt.Sprintf("DELETE FROM %s WHERE instance_id = ?", tableName), nil
}

// buildInsert composes the INSERT for one instance row from the recorded column
// metadata. The table name and column names come from metadata (validated when
// it was written) and are re-validated here as defense-in-depth before being
// interpolated; all values are returned as positional bind arguments. The raw
// INSERT bypasses the ORM's PK default hook, so id is populated explicitly from
// a generated identifier; created_at is omitted so its database DEFAULT applies.
func buildInsert(tableName, instanceID string, columns []approval.FormTableColumn, formData map[string]any) (string, []any, error) {
	if err := approval.ValidateBusinessIdentifier(tableName); err != nil {
		return "", nil, fmt.Errorf("%w: %q: %w", ErrInvalidGeneratedIdentifier, tableName, err)
	}

	names := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))

	for _, column := range columns {
		// created_at is database-defaulted; skip it so the DEFAULT fires.
		if column.ColumnName == "created_at" {
			continue
		}

		if err := approval.ValidateBusinessIdentifier(column.ColumnName); err != nil {
			return "", nil, fmt.Errorf("%w: column %q: %w", ErrInvalidGeneratedIdentifier, column.ColumnName, err)
		}

		var value any

		switch {
		case column.ColumnName == "id":
			value = id.Generate()
		case column.ColumnName == "instance_id":
			value = instanceID
		case column.SourceFieldKey != nil:
			raw, present := formData[*column.SourceFieldKey]
			if !present {
				value = nil
			} else {
				coerced, err := coerceValue(raw)
				if err != nil {
					return "", nil, fmt.Errorf("encode form field %q: %w", *column.SourceFieldKey, err)
				}

				value = nullEmptyForNonText(coerced, column.ColumnType)
				value = coerceIntegerColumnValue(value, column.ColumnType)
			}

		default:
			// A generated column with neither a source field nor a recognized
			// built-in role: insert NULL. This cannot happen for metadata the
			// generator wrote, but keeps Write total over arbitrary metadata.
			value = nil
		}

		names = append(names, column.ColumnName)
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(names, ", "),
		strings.Join(placeholders, ", "),
	)

	return insertSQL, args, nil
}

// coerceValue converts a decoded JSON form value into something the database
// driver accepts as a bind argument. Scalars (string/number/bool/nil) pass
// through; composite values (slices/maps from multi-value fields such as select
// or upload) are JSON-encoded to a string so they land losslessly in the TEXT
// column. The value is always a bound parameter, so this is purely about driver
// compatibility, never SQL safety.
func coerceValue(raw any) (any, error) {
	switch v := raw.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return v, nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		return string(encoded), nil
	}
}

// nullEmptyForNonText maps an empty string to NULL for a non-text column. An
// optional, unfilled date/number/boolean/json field arrives as "" (the form's
// empty value), but a real DATE / NUMERIC / BOOLEAN / JSONB column rejects the
// empty string. Text columns (TEXT/VARCHAR/CHAR) keep "" as a value distinct
// from NULL; a column with no recorded type (legacy metadata) is treated as text.
func nullEmptyForNonText(value any, columnType string) any {
	s, ok := value.(string)
	if !ok || s != "" {
		return value
	}

	if columnType == "" || isTextColumnType(columnType) {
		return value
	}

	return nil
}

// isTextColumnType reports whether a generated column's SQL type is a text type
// (TEXT / VARCHAR(n) / CHAR(n)), where an empty string is a legitimate value.
func isTextColumnType(columnType string) bool {
	upper := strings.ToUpper(columnType)

	return strings.HasPrefix(upper, "TEXT") ||
		strings.HasPrefix(upper, "VARCHAR") ||
		strings.HasPrefix(upper, "CHAR")
}

// isIntegerColumnType reports whether a generated column's SQL type is an
// integer type (BIGINT on Postgres/MySQL, INTEGER on SQLite), where the bound
// value must be an exact integer rather than a float.
func isIntegerColumnType(columnType string) bool {
	upper := strings.ToUpper(columnType)

	return strings.HasPrefix(upper, "BIGINT") || strings.HasPrefix(upper, "INTEGER")
}

// coerceIntegerColumnValue binds an integer column's value as an int64. A JSON
// number decodes to float64, which a driver can render as a float literal (e.g.
// "3.0") that a BIGINT/INTEGER column rejects; converting to int64 makes the
// bind exact. Submission validation (ColumnInteger) has already rejected
// fractional values, so the conversion is lossless. Non-float values (nil, or a
// string left after nullEmptyForNonText) pass through unchanged.
func coerceIntegerColumnValue(value any, columnType string) any {
	if !isIntegerColumnType(columnType) {
		return value
	}

	if f, ok := value.(float64); ok {
		return int64(f)
	}

	return value
}
