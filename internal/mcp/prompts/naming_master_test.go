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
	require.NotNil(t, provider, "TestNamingMasterPrompt should return a non-nil value")

	prompts := provider.Prompts()
	require.Len(t, prompts, 1, "Should have exactly one prompt definition")

	def := prompts[0]
	assert.NotNil(t, def.Prompt, "TestNamingMasterPrompt should return a non-nil value")
	assert.NotNil(t, def.Handler, "TestNamingMasterPrompt should return a non-nil value")

	assert.Equal(t, "naming-master", def.Prompt.Name, "TestNamingMasterPrompt should match expected value")
	assert.Contains(t, def.Prompt.Description, "naming expert", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, def.Prompt.Description, "database", "TestNamingMasterPrompt should include expected value")

	ctx := context.Background()
	req := &mcp.GetPromptRequest{}

	result, err := def.Handler(ctx, req)
	require.NoError(t, err, "TestNamingMasterPrompt should complete without error")
	require.NotNil(t, result, "TestNamingMasterPrompt should return a non-nil value")

	assert.NotEmpty(t, result.Description, "TestNamingMasterPrompt should return non-empty value")
	assert.Len(t, result.Messages, 1, "Should have exactly one message")

	msg := result.Messages[0]
	assert.Equal(t, mcp.Role("user"), msg.Role, "TestNamingMasterPrompt should match expected value")

	textContent, ok := msg.Content.(*mcp.TextContent)
	require.True(t, ok, "Message content should be TextContent")
	assert.NotEmpty(t, textContent.Text, "TestNamingMasterPrompt should return non-empty value")

	content := textContent.Text
	assert.Contains(t, content, "Naming Master", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Core Principles", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Code Naming Conventions", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Database Naming Conventions", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Reserved Word", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Interaction Protocol", "TestNamingMasterPrompt should include expected value")
	assert.Contains(t, content, "Self-Check Checklist", "TestNamingMasterPrompt should include expected value")
}

// TestNamingMasterPromptContent tests naming master prompt content functionality.
func TestNamingMasterPromptContent(t *testing.T) {
	assert.NotEmpty(t, namingMasterPromptContent, "Embedded naming-master.md content should not be empty")

	assert.Contains(t, namingMasterPromptContent, "# Naming Master", "TestNamingMasterPromptContent should include expected value")
	assert.Contains(t, namingMasterPromptContent, "## Style Matrix", "TestNamingMasterPromptContent should include expected value")
	assert.Contains(t, namingMasterPromptContent, "## Standard Audit Fields", "TestNamingMasterPromptContent should include expected value")
	assert.Contains(t, namingMasterPromptContent, "## Foreign Key Strategy Matrix", "TestNamingMasterPromptContent should include expected value")
	assert.Contains(t, namingMasterPromptContent, "## Index Design Considerations", "TestNamingMasterPromptContent should include expected value")
}
