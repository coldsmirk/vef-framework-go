package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

// TestNewToolResultText tests NewToolResultText creates a valid text result.
func TestNewToolResultText(t *testing.T) {
	t.Run("NonEmptyText", func(t *testing.T) {
		result := mcp.NewToolResultText("hello world")

		require.NotNil(t, result, "Result should not be nil")
		assert.False(t, result.IsError, "IsError should be false")
		require.Len(t, result.Content, 1, "Should have exactly 1 content item")

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "Content should be TextContent")
		assert.Equal(t, "hello world", textContent.Text, "Text should match input")
	})

	t.Run("EmptyText", func(t *testing.T) {
		result := mcp.NewToolResultText("")

		require.NotNil(t, result, "Result should not be nil")
		assert.False(t, result.IsError, "IsError should be false")
		require.Len(t, result.Content, 1, "Should have exactly 1 content item")

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "Content should be TextContent")
		assert.Empty(t, textContent.Text, "Text should be empty")
	})
}

// TestNewToolResultError tests NewToolResultError creates a valid error result.
func TestNewToolResultError(t *testing.T) {
	t.Run("ErrorMessage", func(t *testing.T) {
		result := mcp.NewToolResultError("something went wrong")

		require.NotNil(t, result, "Result should not be nil")
		assert.True(t, result.IsError, "IsError should be true")
		require.Len(t, result.Content, 1, "Should have exactly 1 content item")

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "Content should be TextContent")
		assert.Equal(t, "something went wrong", textContent.Text, "Error message should match")
	})

	t.Run("EmptyErrorMessage", func(t *testing.T) {
		result := mcp.NewToolResultError("")

		require.NotNil(t, result, "Result should not be nil")
		assert.True(t, result.IsError, "IsError should be true")
	})
}
