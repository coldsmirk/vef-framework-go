package prompts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

// TestNamingMasterPrompt tests naming master prompt functionality.
func TestNamingMasterPrompt(t *testing.T) {
	provider := NewNamingMasterPrompt()
	require.NotNil(t, provider, "Should not be nil")

	prompts := provider.Prompts()
	require.Len(t, prompts, 1, "Should have exactly one prompt definition")

	def := prompts[0]
	assert.NotNil(t, def.Prompt, "Should not be nil")
	assert.NotNil(t, def.Handler, "Should not be nil")

	assert.Equal(t, "naming-master", def.Prompt.Name, "Should equal expected value")
	assert.Contains(t, def.Prompt.Description, "naming expert", "Should contain expected value")
	assert.Contains(t, def.Prompt.Description, "database", "Should contain expected value")

	ctx := context.Background()
	req := &mcp.GetPromptRequest{}

	result, err := def.Handler(ctx, req)
	require.NoError(t, err, "Should not return error")
	require.NotNil(t, result, "Should not be nil")

	assert.NotEmpty(t, result.Description, "Should not be empty")
	assert.Len(t, result.Messages, 1, "Should have exactly one message")

	msg := result.Messages[0]
	assert.Equal(t, mcp.Role("user"), msg.Role, "Should equal expected value")

	textContent, ok := msg.Content.(*mcp.TextContent)
	require.True(t, ok, "Message content should be TextContent")
	assert.NotEmpty(t, textContent.Text, "Should not be empty")

	content := textContent.Text
	assert.Contains(t, content, "Naming Master", "Should contain expected value")
	assert.Contains(t, content, "Core Principles", "Should contain expected value")
	assert.Contains(t, content, "Code Naming Conventions", "Should contain expected value")
	assert.Contains(t, content, "Database Naming Conventions", "Should contain expected value")
	assert.Contains(t, content, "Reserved Word", "Should contain expected value")
	assert.Contains(t, content, "Interaction Protocol", "Should contain expected value")
	assert.Contains(t, content, "Self-Check Checklist", "Should contain expected value")
}

// TestNamingMasterPromptContent tests naming master prompt content functionality.
func TestNamingMasterPromptContent(t *testing.T) {
	assert.NotEmpty(t, namingMasterPromptContent, "Embedded naming-master.md content should not be empty")

	assert.Contains(t, namingMasterPromptContent, "# Naming Master", "Should contain expected value")
	assert.Contains(t, namingMasterPromptContent, "## Style Matrix", "Should contain expected value")
	assert.Contains(t, namingMasterPromptContent, "## Standard Audit Fields", "Should contain expected value")
	assert.Contains(t, namingMasterPromptContent, "## Foreign Key Strategy Matrix", "Should contain expected value")
	assert.Contains(t, namingMasterPromptContent, "## Index Design Considerations", "Should contain expected value")
}
