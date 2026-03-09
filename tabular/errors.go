package tabular

import (
	"errors"
	"fmt"
)

// ErrUnsupportedType indicates the target type is not supported by the parser.
var ErrUnsupportedType = errors.New("unsupported type")

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
