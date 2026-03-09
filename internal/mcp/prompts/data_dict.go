package prompts

import (
	"context"
	"strings"

	_ "embed"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

//go:embed data-dict-prompt.md
var dataDictPromptContent string

// DataDictPrompt provides the data dictionary management prompt.
type DataDictPrompt struct{}

// NewDataDictPrompt creates a new DataDictPrompt instance.
func NewDataDictPrompt() mcp.PromptProvider {
	return &DataDictPrompt{}
}

// Prompts implements mcp.PromptProvider.
func (p *DataDictPrompt) Prompts() []mcp.PromptDefinition {
	return []mcp.PromptDefinition{
		{
			Prompt: &mcp.Prompt{
				Name:        "data-dict-assistant",
				Description: "Data dictionary management assistant for VEF Framework applications. Helps manage system dictionaries (enumeration values, dropdown options, and configuration items) through safe, structured database operations. Table names default to 'sys_data_dict' and 'sys_data_dict_item' if not specified.",
				Arguments: []*mcp.PromptArgument{
					{
						Name:        "dict_table",
						Description: "The name of the dictionary table. Defaults to 'sys_data_dict' if not specified.",
						Required:    false,
					},
					{
						Name:        "dict_item_table",
						Description: "The name of the dictionary item table. Defaults to 'sys_data_dict_item' if not specified.",
						Required:    false,
					},
				},
			},
			Handler: p.handleDataDictPrompt,
		},
	}
}

// handleDataDictPrompt handles the data dictionary prompt request.
func (*DataDictPrompt) handleDataDictPrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	dictTable := getArgument(req.Params.Arguments, "dict_table", "sys_data_dict")
	dictItemTable := getArgument(req.Params.Arguments, "dict_item_table", "sys_data_dict_item")

	content := strings.ReplaceAll(dataDictPromptContent, "{{DICT_TABLE}}", dictTable)
	content = strings.ReplaceAll(content, "{{DICT_ITEM_TABLE}}", dictItemTable)

	return &mcp.GetPromptResult{
		Description: "Data dictionary management assistant prompt with configured table names",
		Messages: []*mcp.PromptMessage{
			{
				Role:    mcp.Role("user"),
				Content: &mcp.TextContent{Text: content},
			},
		},
	}, nil
}

func getArgument(args map[string]string, key, defaultValue string) string {
	if args == nil {
		return defaultValue
	}

	if v, ok := args[key]; ok && v != "" {
		return v
	}

	return defaultValue
}
