package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeUniqueIDs(t *testing.T) {
	t.Run("NilOrEmptyInput", func(t *testing.T) {
		assert.Nil(t, NormalizeUniqueIDs(nil), "Nil input should return nil")
		assert.Nil(t, NormalizeUniqueIDs([]string{}), "Empty input should return nil")
	})

	t.Run("TrimDeduplicateAndSkipEmpty", func(t *testing.T) {
		got := NormalizeUniqueIDs([]string{" user-1 ", "", "user-2", "user-1", "   ", " user-2 "})
		assert.Equal(t, []string{"user-1", "user-2"}, got, "Should trim, deduplicate, and keep first-seen order")
	})
}
