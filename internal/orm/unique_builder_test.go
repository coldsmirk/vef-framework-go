package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUniqueBuilder verifies the fluent UniqueDef builder API.
func TestUniqueBuilder(t *testing.T) {
	t.Run("ColumnsOnly", func(t *testing.T) {
		u := &UniqueDef{}
		u.Columns("email", "tenant_id")

		assert.Equal(t, []string{"email", "tenant_id"}, u.columns, "Should store all specified columns")
		assert.Empty(t, u.name, "Name should be empty by default")
	})

	t.Run("NamedConstraint", func(t *testing.T) {
		u := &UniqueDef{}
		u.Columns("email", "tenant_id").Name("uq_test")

		assert.Equal(t, "uq_test", u.name, "Should store the constraint name")
		assert.Equal(t, []string{"email", "tenant_id"}, u.columns, "Should store all specified columns")
	})
}
