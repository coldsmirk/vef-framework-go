package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckBuilder verifies the fluent CheckDef builder API.
func TestCheckBuilder(t *testing.T) {
	t.Run("NameAndCondition", func(t *testing.T) {
		ck := &CheckDef{}
		called := false
		ck.Name("ck_test").Condition(func(ConditionBuilder) { called = true })

		assert.Equal(t, "ck_test", ck.name, "Should store the constraint name")
		require.NotNil(t, ck.conditionBuilder, "Should store the condition builder function")
		ck.conditionBuilder(nil)
		assert.True(t, called, "Condition builder should be invocable")
	})

	t.Run("ConditionOnly", func(t *testing.T) {
		ck := &CheckDef{}
		ck.Condition(func(ConditionBuilder) {})

		assert.Empty(t, ck.name, "Name should be empty when not set")
		require.NotNil(t, ck.conditionBuilder, "Should store the condition builder function")
	})
}
