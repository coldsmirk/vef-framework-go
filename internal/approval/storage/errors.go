package storage

import "errors"

var (
	// ErrInvalidDetailValue indicates a detail-table field's runtime value is
	// not the list-of-row-objects shape the projection expects. Submit
	// validation rejects this earlier; hitting it here means corrupt data.
	ErrInvalidDetailValue = errors.New("approval/storage: invalid detail-table value")

	// ErrUnsupportedDialect indicates the configured database dialect has no
	// type mapping for generating dynamic form tables.
	ErrUnsupportedDialect = errors.New("approval/storage: unsupported database dialect")

	// ErrInvalidGeneratedIdentifier indicates a generated table or column name
	// failed approval.ValidateBusinessIdentifier. It signals a form schema whose
	// field keys cannot be turned into safe SQL identifiers; the publish that
	// triggered generation is failed rather than emitting unsafe DDL.
	ErrInvalidGeneratedIdentifier = errors.New("approval/storage: generated identifier is not SQL-safe")

	// ErrReservedColumnName indicates a form field key maps to one of the
	// built-in physical columns (id / instance_id / row_index / created_at).
	ErrReservedColumnName = errors.New("approval/storage: field key collides with a reserved column")

	// ErrDuplicateColumnName indicates two distinct field keys sanitized to the
	// same physical column name.
	ErrDuplicateColumnName = errors.New("approval/storage: duplicate generated column name")

	// ErrFormTableMetadataMissing indicates a table-mode instance write found no
	// apv_form_table row for the version. Publish provisions it, so its absence
	// is a real inconsistency rather than an expected branch.
	ErrFormTableMetadataMissing = errors.New("approval/storage: form table metadata missing for version")

	// ErrInvalidColumnType indicates a form field declares an explicit ColumnType
	// that is not a defined approval.ColumnDataType. Caught at deploy so a bad
	// column-type override surfaces when the admin saves, not opaquely at publish.
	ErrInvalidColumnType = errors.New("approval/storage: field has an invalid column data type")
)
