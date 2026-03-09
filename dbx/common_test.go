package dbx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestColumnWithAlias tests column with alias functionality.
func TestColumnWithAlias(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		alias    []string
		expected string
	}{
		{
			name:     "ColumnWithoutAlias",
			column:   "name",
			alias:    []string{},
			expected: "name",
		},
		{
			name:     "ColumnWithAlias",
			column:   "name",
			alias:    []string{"u"},
			expected: "u.name",
		},
		{
			name:     "ColumnWithEmptyAlias",
			column:   "age",
			alias:    []string{""},
			expected: "age",
		},
		{
			name:     "ColumnWithMultipleAliasValues",
			column:   "email",
			alias:    []string{"user", "profile"},
			expected: "user.email",
		},
		{
			name:     "EmptyColumnWithAlias",
			column:   "",
			alias:    []string{"t"},
			expected: "t.",
		},
		{
			name:     "EmptyColumnWithoutAlias",
			column:   "",
			alias:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColumnWithAlias(tt.column, tt.alias...)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}
