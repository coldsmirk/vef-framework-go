package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrimaryKeyBuilder verifies the fluent PrimaryKeyDef builder API.
func TestPrimaryKeyBuilder(t *testing.T) {
	t.Run("ColumnsOnly", func(t *testing.T) {
		pk := &PrimaryKeyDef{}
		pk.Columns("user_id", "product_id")

		assert.Equal(t, []string{"user_id", "product_id"}, pk.columns, "Should store all specified columns")
		assert.Empty(t, pk.name, "Name should be empty by default")
	})

	t.Run("NamedConstraint", func(t *testing.T) {
		pk := &PrimaryKeyDef{}
		pk.Columns("user_id", "product_id").Name("pk_test")

		assert.Equal(t, "pk_test", pk.name, "Should store the constraint name")
		assert.Equal(t, []string{"user_id", "product_id"}, pk.columns, "Should store all specified columns")
	})
}
