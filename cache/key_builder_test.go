package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrefixKeyBuilder tests prefix key builder functionality.
func TestPrefixKeyBuilder(t *testing.T) {
	t.Run("BuildWithoutPrefix", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("")

		result := builder.Build("user", "123")
		assert.Equal(t, "user:123", result, "Should equal expected value")
	})

	t.Run("BuildWithPrefix", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("app")

		result := builder.Build("user", "123")
		assert.Equal(t, "app:user:123", result, "Should equal expected value")
	})

	t.Run("BuildWithEmptyKeyParts", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("app")

		result := builder.Build()
		assert.Equal(t, "app", result, "Should equal expected value")
	})

	t.Run("BuildWithEmptyKeyPartsNoPrefix", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("")

		result := builder.Build()
		assert.Equal(t, "", result, "Should equal expected value")
	})

	t.Run("BuildWithSingleKeyPart", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("app")

		result := builder.Build("user")
		assert.Equal(t, "app:user", result, "Should equal expected value")
	})

	t.Run("BuildWithMultipleKeyParts", func(t *testing.T) {
		builder := NewPrefixKeyBuilder("app")

		result := builder.Build("user", "123", "profile")
		assert.Equal(t, "app:user:123:profile", result, "Should equal expected value")
	})

	t.Run("BuildWithCustomSeparator", func(t *testing.T) {
		builder := NewPrefixKeyBuilderWithSeparator("app", "/")

		result := builder.Build("user", "123")
		assert.Equal(t, "app/user/123", result, "Should equal expected value")
	})

	t.Run("BuildWithCustomSeparatorEmptyKeyParts", func(t *testing.T) {
		builder := NewPrefixKeyBuilderWithSeparator("app", "/")

		result := builder.Build()
		assert.Equal(t, "app", result, "Should equal expected value")
	})

	t.Run("KeyHelperFunction", func(t *testing.T) {
		result := Key("user", "123")
		assert.Equal(t, "user:123", result, "Should equal expected value")
	})

	t.Run("KeyHelperFunctionEmptyParts", func(t *testing.T) {
		result := Key()
		assert.Equal(t, "", result, "Should equal expected value")
	})
}
