package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

// TestDataDictPrompt tests data dict prompt functionality.
func TestDataDictPrompt(t *testing.T) {
	provider := NewDataDictPrompt()
	require.NotNil(t, provider, "Should not be nil")

	prompts := provider.Prompts()
	require.Len(t, prompts, 1, "Should have exactly one prompt definition")

	def := prompts[0]
	assert.NotNil(t, def.Prompt, "Should not be nil")
	assert.NotNil(t, def.Handler, "Should not be nil")

	assert.Equal(t, "data-dict-assistant", def.Prompt.Name, "Should equal expected value")
	assert.Contains(t, def.Prompt.Description, "Data dictionary", "Should contain expected value")
	assert.Len(t, def.Prompt.Arguments, 2, "Should have two arguments")

	assert.Equal(t, "dict_table", def.Prompt.Arguments[0].Name, "Should equal expected value")
	assert.False(t, def.Prompt.Arguments[0].Required, "dict_table should be optional")

	assert.Equal(t, "dict_item_table", def.Prompt.Arguments[1].Name, "Should equal expected value")
	assert.False(t, def.Prompt.Arguments[1].Required, "dict_item_table should be optional")
}

// TestDataDictPromptHandler tests data dict prompt handler functionality.
func TestDataDictPromptHandler(t *testing.T) {
	provider := NewDataDictPrompt()
	def := provider.Prompts()[0]
	ctx := context.Background()

	tests := []struct {
		name              string
		arguments         map[string]string
		expectedDictTable string
		expectedItemTable string
	}{
		{
			name:              "DefaultTableNames",
			arguments:         nil,
			expectedDictTable: "sys_data_dict",
			expectedItemTable: "sys_data_dict_item",
		},
		{
			name: "CustomDictTable",
			arguments: map[string]string{
				"dict_table": "custom_dict",
			},
			expectedDictTable: "custom_dict",
			expectedItemTable: "sys_data_dict_item",
		},
		{
			name: "CustomItemTable",
			arguments: map[string]string{
				"dict_item_table": "custom_item",
			},
			expectedDictTable: "sys_data_dict",
			expectedItemTable: "custom_item",
		},
		{
			name: "BothCustomTables",
			arguments: map[string]string{
				"dict_table":      "my_dict",
				"dict_item_table": "my_item",
			},
			expectedDictTable: "my_dict",
			expectedItemTable: "my_item",
		},
		{
			name: "EmptyArgumentValues",
			arguments: map[string]string{
				"dict_table":      "",
				"dict_item_table": "",
			},
			expectedDictTable: "sys_data_dict",
			expectedItemTable: "sys_data_dict_item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mcp.GetPromptRequest{
				Params: &mcp.GetPromptParams{
					Arguments: tt.arguments,
				},
			}

			result, err := def.Handler(ctx, req)
			require.NoError(t, err, "Handler should not return error")
			require.NotNil(t, result, "Result should not be nil")

			assert.NotEmpty(t, result.Description, "Description should not be empty")
			assert.Len(t, result.Messages, 1, "Should have exactly one message")

			msg := result.Messages[0]
			assert.Equal(t, mcp.Role("user"), msg.Role, "Message role should be user")

			textContent, ok := msg.Content.(*mcp.TextContent)
			require.True(t, ok, "Message content should be TextContent")
			assert.NotEmpty(t, textContent.Text, "Message text should not be empty")

			assert.Contains(t, textContent.Text, tt.expectedDictTable,
				"Content should contain expected dict table name")
			assert.Contains(t, textContent.Text, tt.expectedItemTable,
				"Content should contain expected item table name")

			assert.NotContains(t, textContent.Text, "{{DICT_TABLE}}",
				"Content should not contain unreplaced placeholder")
			assert.NotContains(t, textContent.Text, "{{DICT_ITEM_TABLE}}",
				"Content should not contain unreplaced placeholder")
		})
	}
}

// TestDataDictPromptContent tests data dict prompt content functionality.
func TestDataDictPromptContent(t *testing.T) {
	assert.NotEmpty(t, dataDictPromptContent, "Embedded data-dict-prompt.md content should not be empty")

	assert.Contains(t, dataDictPromptContent, "{{DICT_TABLE}}", "Content should contain dict table placeholder")
	assert.Contains(t, dataDictPromptContent, "{{DICT_ITEM_TABLE}}", "Content should contain item table placeholder")
}

// TestDataDictPromptPlaceholderReplacement tests data dict prompt placeholder replacement functionality.
func TestDataDictPromptPlaceholderReplacement(t *testing.T) {
	provider := NewDataDictPrompt()
	def := provider.Prompts()[0]
	ctx := context.Background()

	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Arguments: map[string]string{
				"dict_table":      "test_dict",
				"dict_item_table": "test_item",
			},
		},
	}

	result, err := def.Handler(ctx, req)
	require.NoError(t, err, "Should not return error")

	textContent := result.Messages[0].Content.(*mcp.TextContent)

	dictTableCount := strings.Count(textContent.Text, "test_dict")
	itemTableCount := strings.Count(textContent.Text, "test_item")

	assert.Greater(t, dictTableCount, 0, "Custom dict table name should appear at least once")
	assert.Greater(t, itemTableCount, 0, "Custom item table name should appear at least once")

	originalDictCount := strings.Count(dataDictPromptContent, "{{DICT_TABLE}}")
	originalItemCount := strings.Count(dataDictPromptContent, "{{DICT_ITEM_TABLE}}")

	assert.Equal(t, originalDictCount, dictTableCount,
		"Number of dict table replacements should match placeholder count")
	assert.Equal(t, originalItemCount, itemTableCount,
		"Number of item table replacements should match placeholder count")
}
