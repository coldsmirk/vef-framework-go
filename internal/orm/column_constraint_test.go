package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConstraintConstructors verifies that each constraint constructor produces the correct kind and fields.
func TestConstraintConstructors(t *testing.T) {
	t.Run("SimpleKinds", func(t *testing.T) {
		tests := []struct {
			name     string
			factory  func() ColumnConstraint
			expected ConstraintKind
		}{
			{"NotNull", NotNull, ConstraintNotNull},
			{"Nullable", Nullable, ConstraintNullable},
			{"PrimaryKey", PrimaryKey, ConstraintPrimaryKey},
			{"Unique", Unique, ConstraintUnique},
			{"AutoIncrement", AutoIncrement, ConstraintAutoIncrement},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := tt.factory()
				assert.Equal(t, tt.expected, c.kind, "Should produce the correct constraint kind")
			})
		}
	})

	t.Run("DefaultValues", func(t *testing.T) {
		tests := []struct {
			name     string
			value    any
			expected any
		}{
			{"IntValue", 42, 42},
			{"StringValue", "hello", "hello"},
			{"BoolValue", true, true},
			{"NilValue", nil, nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := Default(tt.value)
				assert.Equal(t, ConstraintDefault, c.kind, "Should produce Default constraint kind")
				assert.Equal(t, tt.expected, c.defaultValue, "Should store the default value")
			})
		}
	})

	t.Run("Check", func(t *testing.T) {
		builder := func(cb ConditionBuilder) { cb.GreaterThan("age", 0) }
		c := Check(builder)
		assert.Equal(t, ConstraintCheck, c.kind, "Should produce Check constraint kind")
		require.NotNil(t, c.checkBuilder, "Should store the check builder function")
	})

	t.Run("ReferencesSingleColumn", func(t *testing.T) {
		c := References("users", "id")
		assert.Equal(t, ConstraintReferences, c.kind, "Should produce References constraint kind")
		assert.Equal(t, "users", c.refTable, "Should store the referenced table")
		assert.Equal(t, []string{"id"}, c.refColumns, "Should store the referenced column")
	})

	t.Run("ReferencesMultiColumn", func(t *testing.T) {
		c := References("orders", "user_id", "order_id")
		assert.Equal(t, ConstraintReferences, c.kind, "Should produce References constraint kind")
		assert.Equal(t, "orders", c.refTable, "Should store the referenced table")
		assert.Equal(t, []string{"user_id", "order_id"}, c.refColumns, "Should store all referenced columns")
	})
}
