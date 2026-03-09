package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderedUniqueMaintainsFirstSeenOrder(t *testing.T) {
	t.Run("MaintainFirstSeenOrder", func(t *testing.T) {
		u := NewOrderedUnique[string](0)
		added := u.AddAll("u2", "u1", "u2", "u3", "u1")

		assert.Equal(t, 3, added, "AddAll should return the number of newly added unique values")
		assert.Equal(t, 3, u.Len(), "Len should equal the count of unique values")

		got := u.ToSlice()
		want := []string{"u2", "u1", "u3"}
		require.Len(t, got, len(want), "ToSlice should return all unique values")
		assert.Equal(t, want, got, "ToSlice should keep first-seen order while removing duplicates")
	})
}

func TestOrderedUniqueAddReturnValue(t *testing.T) {
	t.Run("FirstAddReturnsTrueAndDuplicateReturnsFalse", func(t *testing.T) {
		u := NewOrderedUnique[string](1)

		assert.True(t, u.Add("u1"), "First Add should return true")
		assert.False(t, u.Add("u1"), "Adding duplicate value should return false")

		got := u.ToSlice()
		require.Len(t, got, 1, "ToSlice should contain one element after duplicate add attempt")
		assert.Equal(t, "u1", got[0], "Stored value should be the first added unique element")
		assert.Equal(t, 1, u.Len(), "Len should remain one after duplicate add attempt")
	})
}

func TestOrderedUniqueContains(t *testing.T) {
	t.Run("ContainsShouldReflectMembership", func(t *testing.T) {
		u := NewOrderedUnique[string](2)

		assert.False(t, u.Contains("u1"), "Contains should be false before adding value")
		assert.True(t, u.Add("u1"), "Add should insert new value")
		assert.True(t, u.Contains("u1"), "Contains should be true after value is added")
	})
}
