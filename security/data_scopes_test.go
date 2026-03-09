package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAllDataScope tests AllDataScope functionality.
func TestAllDataScope(t *testing.T) {
	scope := NewAllDataScope()

	t.Run("Key", func(t *testing.T) {
		assert.Equal(t, "all", scope.Key(), "Should return 'all'")
	})

	t.Run("Priority", func(t *testing.T) {
		assert.Equal(t, PriorityAll, scope.Priority(), "Should return PriorityAll")
	})

	t.Run("SupportsAlwaysTrue", func(t *testing.T) {
		assert.True(t, scope.Supports(nil, nil), "Should always return true")
		assert.True(t, scope.Supports(NewUser("u1", "Alice"), nil), "Should return true for any principal")
	})

	t.Run("ApplyAlwaysNil", func(t *testing.T) {
		assert.NoError(t, scope.Apply(nil, nil), "Should always return nil")
	})
}

// TestNewAllDataScope tests NewAllDataScope constructor.
func TestNewAllDataScope(t *testing.T) {
	scope := NewAllDataScope()

	_, ok := scope.(*AllDataScope)
	assert.True(t, ok, "Should return *AllDataScope")
}

// TestSelfDataScope tests SelfDataScope Key and Priority.
func TestSelfDataScope(t *testing.T) {
	t.Run("Key", func(t *testing.T) {
		scope := NewSelfDataScope("")
		assert.Equal(t, "self", scope.Key(), "Should return 'self'")
	})

	t.Run("Priority", func(t *testing.T) {
		scope := NewSelfDataScope("")
		assert.Equal(t, PrioritySelf, scope.Priority(), "Should return PrioritySelf")
	})
}

// TestNewSelfDataScope tests NewSelfDataScope constructor defaults.
func TestNewSelfDataScope(t *testing.T) {
	t.Run("EmptyColumnUsesDefault", func(t *testing.T) {
		scope := NewSelfDataScope("").(*SelfDataScope)
		assert.Equal(t, "created_by", scope.createdByColumn, "Should default to 'created_by'")
	})

	t.Run("CustomColumn", func(t *testing.T) {
		scope := NewSelfDataScope("creator_id").(*SelfDataScope)
		assert.Equal(t, "creator_id", scope.createdByColumn, "Should use custom column")
	})
}
