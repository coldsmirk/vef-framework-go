package tabular

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatRowError covers all four formatting branches of formatRowError:
// both column and field present, only column, only field, and neither.
func TestFormatRowError(t *testing.T) {
	cause := errors.New("boom")

	tests := []struct {
		name     string
		row      int
		column   string
		field    string
		expected string
	}{
		{"ColumnAndField", 3, "Name", "user_name", "row 3, column Name (field user_name): boom"},
		{"ColumnOnly", 4, "Name", "", "row 4, column Name: boom"},
		{"FieldOnly", 5, "", "user_name", "row 5, field user_name: boom"},
		{"Neither", 6, "", "", "row 6: boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRowError(tt.row, tt.column, tt.field, cause)
			assert.Equal(t, tt.expected, got, "formatRowError should produce the documented format")
		})
	}
}

// TestImportError exercises the public surface of ImportError.
func TestImportError(t *testing.T) {
	cause := errors.New("parse failed")
	importErr := ImportError{Row: 7, Column: "Age", Field: "age", Err: cause}

	t.Run("ErrorMessage", func(t *testing.T) {
		assert.Equal(t, "row 7, column Age (field age): parse failed", importErr.Error(),
			"Error should reuse formatRowError")
	})

	t.Run("UnwrapReturnsCause", func(t *testing.T) {
		assert.Same(t, cause, importErr.Unwrap(), "Unwrap should expose the wrapped cause")
	})

	t.Run("ErrorsIsTraversesUnwrap", func(t *testing.T) {
		require.ErrorIs(t, importErr, cause, "errors.Is should walk through Unwrap")
	})

	t.Run("ZeroValueDegradesGracefully", func(t *testing.T) {
		zero := ImportError{}
		assert.Equal(t, "row 0: <nil>", zero.Error(),
			"Zero ImportError should still format without panicking")
		assert.Nil(t, zero.Unwrap(), "Unwrap on zero ImportError should be nil")
	})
}

// TestExportError exercises the public surface of ExportError.
func TestExportError(t *testing.T) {
	cause := errors.New("format failed")
	exportErr := ExportError{Row: 2, Column: "Date", Field: "date", Err: cause}

	t.Run("ErrorMessage", func(t *testing.T) {
		assert.Equal(t, "row 2, column Date (field date): format failed", exportErr.Error(),
			"Error should reuse formatRowError")
	})

	t.Run("UnwrapReturnsCause", func(t *testing.T) {
		assert.Same(t, cause, exportErr.Unwrap(), "Unwrap should expose the wrapped cause")
	})

	t.Run("ErrorsIsTraversesUnwrap", func(t *testing.T) {
		require.ErrorIs(t, exportErr, cause, "errors.Is should walk through Unwrap")
	})
}
