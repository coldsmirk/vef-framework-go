package tabular

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedType indicates the target type is not supported by the parser.
	ErrUnsupportedType = errors.New("unsupported type")
	// ErrDataMustBeSlice indicates the input data for export is not a slice.
	ErrDataMustBeSlice = errors.New("data must be a slice")
	// ErrNoDataRowsFound indicates the source contains no usable data rows.
	ErrNoDataRowsFound = errors.New("no data rows found")
	// ErrDuplicateColumnKey indicates a duplicate Key in a []ColumnSpec.
	ErrDuplicateColumnKey = errors.New("duplicate column key")
	// ErrDuplicateHeaderName indicates a duplicate header name during mapping.
	ErrDuplicateHeaderName = errors.New("duplicate header name")
	// ErrUnsetField indicates a struct field cannot be set (usually unexported).
	ErrUnsetField = errors.New("field is not settable")
	// ErrRequiredMissing indicates a required cell is empty.
	ErrRequiredMissing = errors.New("required value is missing")
	// ErrUnknownColumn indicates a column key is not present in the schema.
	ErrUnknownColumn = errors.New("unknown column")
	// ErrSchemaMismatch indicates the provided data does not match the schema
	// required by the adapter (e.g. wrong element type).
	ErrSchemaMismatch = errors.New("schema mismatch")
	// ErrMissingColumnType indicates a dynamic column spec has no target Type.
	ErrMissingColumnType = errors.New("column type is required")
	// ErrMissingColumnKey indicates a dynamic column spec has no Key.
	ErrMissingColumnKey = errors.New("column key is required")
	// ErrTypedRowMismatch indicates a TypedImporter received rows whose element
	// type does not match the requested generic parameter.
	ErrTypedRowMismatch = errors.New("importer returned unexpected row type")
)

func formatRowError(row int, column, field string, err error) string {
	switch {
	case column != "" && field != "":
		return fmt.Sprintf("row %d, column %s (field %s): %v", row, column, field, err)
	case column != "":
		return fmt.Sprintf("row %d, column %s: %v", row, column, err)
	case field != "":
		return fmt.Sprintf("row %d, field %s: %v", row, field, err)
	default:
		return fmt.Sprintf("row %d: %v", row, err)
	}
}

// ImportError represents an error that occurred during data import.
// Row is 1-based and includes the header row.
//
// Err may carry multiple leaf errors joined via errors.Join when a single
// row produces several failures (e.g. multiple Required misses or both a
// cell validator and a row validator failing). Use errors.Is on the
// ImportError to match a specific cause; to enumerate all leaves, assert
// Err against the interface{ Unwrap() []error } interface.
type ImportError struct {
	Row    int
	Column string
	Field  string
	Err    error
}

// Error implements the error interface.
func (e ImportError) Error() string {
	return formatRowError(e.Row, e.Column, e.Field, e.Err)
}

// Unwrap returns the underlying error.
func (e ImportError) Unwrap() error {
	return e.Err
}

// ExportError represents an error that occurred during data export.
// Row is 0-based data row index.
type ExportError struct {
	Row    int
	Column string
	Field  string
	Err    error
}

// Error implements the error interface.
func (e ExportError) Error() string {
	return formatRowError(e.Row, e.Column, e.Field, e.Err)
}

// Unwrap returns the underlying error.
func (e ExportError) Unwrap() error {
	return e.Err
}
