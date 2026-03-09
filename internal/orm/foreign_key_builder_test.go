package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForeignKeyBuilder verifies the fluent ForeignKeyDef builder API.
func TestForeignKeyBuilder(t *testing.T) {
	t.Run("BasicFK", func(t *testing.T) {
		fk := &ForeignKeyDef{}
		fk.Columns("user_id").References("users", "id")

		assert.Equal(t, []string{"user_id"}, fk.columns, "Should store the local column")
		assert.Equal(t, "users", fk.refTable, "Should store the referenced table")
		assert.Equal(t, []string{"id"}, fk.refColumns, "Should store the referenced column")
		assert.Nil(t, fk.onDelete, "OnDelete should be nil by default")
		assert.Nil(t, fk.onUpdate, "OnUpdate should be nil by default")
	})

	t.Run("WithReferentialActions", func(t *testing.T) {
		fk := &ForeignKeyDef{}
		fk.Columns("order_id").
			References("orders", "id").
			OnDelete(ReferenceCascade).
			OnUpdate(ReferenceRestrict)

		assert.Equal(t, []string{"order_id"}, fk.columns, "Should store the local column")
		assert.Equal(t, "orders", fk.refTable, "Should store the referenced table")
		assert.Equal(t, []string{"id"}, fk.refColumns, "Should store the referenced column")
		require.NotNil(t, fk.onDelete, "OnDelete should be set")
		assert.Equal(t, ReferenceCascade, *fk.onDelete, "OnDelete should be CASCADE")
		require.NotNil(t, fk.onUpdate, "OnUpdate should be set")
		assert.Equal(t, ReferenceRestrict, *fk.onUpdate, "OnUpdate should be RESTRICT")
	})

	t.Run("CompositeColumns", func(t *testing.T) {
		fk := &ForeignKeyDef{}
		fk.Columns("user_id", "order_id").References("user_orders", "uid", "oid")

		assert.Equal(t, []string{"user_id", "order_id"}, fk.columns, "Should store all local columns")
		assert.Equal(t, []string{"uid", "oid"}, fk.refColumns, "Should store all referenced columns")
	})

	t.Run("NamedConstraint", func(t *testing.T) {
		fk := &ForeignKeyDef{}
		fk.Name("fk_test").Columns("a", "b").References("other", "x", "y")

		assert.Equal(t, "fk_test", fk.name, "Should store the constraint name")
		assert.Equal(t, []string{"a", "b"}, fk.columns, "Should store all local columns")
		assert.Equal(t, "other", fk.refTable, "Should store the referenced table")
		assert.Equal(t, []string{"x", "y"}, fk.refColumns, "Should store all referenced columns")
	})
}

// TestReferenceActionString verifies the String() output for all ReferenceAction values.
func TestReferenceActionString(t *testing.T) {
	tests := []struct {
		name     string
		action   ReferenceAction
		expected string
	}{
		{"Cascade", ReferenceCascade, "CASCADE"},
		{"Restrict", ReferenceRestrict, "RESTRICT"},
		{"SetNull", ReferenceSetNull, "SET NULL"},
		{"SetDefault", ReferenceSetDefault, "SET DEFAULT"},
		{"NoAction", ReferenceNoAction, "NO ACTION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.String(),
				"Should return the correct SQL action string")
		})
	}
}
