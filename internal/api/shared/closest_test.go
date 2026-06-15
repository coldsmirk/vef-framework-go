package shared

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClosest covers the generic nearest-match helper, including the returned
// distance (used by callers for a proximity threshold), the empty and tie
// (ambiguous) cases, and projection over a non-string element type.
func TestClosest(t *testing.T) {
	identity := func(s string) string { return s }

	t.Run("Empty", func(t *testing.T) {
		match, distance, ok := Closest("target", slices.Values([]string{}), identity)
		assert.False(t, ok, "empty input should not yield a match")
		assert.Empty(t, match, "match should be the zero value when not ok")
		assert.Zero(t, distance, "distance should be zero when not ok")
	})

	t.Run("ExactMatch", func(t *testing.T) {
		match, distance, ok := Closest("get_user", slices.Values([]string{"get_user", "get_users"}), identity)
		require.True(t, ok, "an exact match should be found")
		assert.Equal(t, "get_user", match, "exact candidate should be returned")
		assert.Equal(t, 0, distance, "exact match distance should be zero")
	})

	t.Run("ClosestReturnsDistance", func(t *testing.T) {
		match, distance, ok := Closest("get_cpu", slices.Values([]string{"get_cpus", "get_memory"}), identity)
		require.True(t, ok, "a closest match should be found")
		assert.Equal(t, "get_cpus", match, "the nearest candidate should be returned")
		assert.Equal(t, 1, distance, "distance to the nearest candidate should be reported")
	})

	t.Run("AmbiguousTie", func(t *testing.T) {
		_, _, ok := Closest("ab", slices.Values([]string{"ac", "ad"}), identity)
		assert.False(t, ok, "two equally close candidates should be reported as ambiguous")
	})

	t.Run("ProjectsNonStringElements", func(t *testing.T) {
		type entry struct {
			id   int
			name string
		}

		items := []entry{
			{id: 1, name: "alpha"},
			{id: 2, name: "alpine"},
		}

		match, distance, ok := Closest("alpha", slices.Values(items), func(e entry) string { return e.name })
		require.True(t, ok, "a closest entry should be found")
		assert.Equal(t, 1, match.id, "the entry whose projected key matches should be returned")
		assert.Equal(t, 0, distance, "the matching projection should have zero distance")
	})

	t.Run("SingleCandidate", func(t *testing.T) {
		match, distance, ok := Closest("xyz", slices.Values([]string{"abc"}), identity)
		require.True(t, ok, "a single candidate is unambiguous")
		assert.Equal(t, "abc", match, "the only candidate should be returned")
		assert.Equal(t, 3, distance, "distance to the only candidate should be reported")
	})
}
