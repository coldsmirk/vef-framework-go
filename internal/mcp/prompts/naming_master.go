package prompts

import (
	"context"

	_ "embed"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

//go:embed naming-master.md
var namingMasterPromptContent string

// NamingMasterPrompt provides the naming master prompt for professional IT naming conventions.
type NamingMasterPrompt struct{}

// NewNamingMasterPrompt creates a new NamingMasterPrompt instance.
func NewNamingMasterPrompt() mcp.PromptProvider {
	return &NamingMasterPrompt{}
}

// Prompts implements mcp.PromptProvider.
func (p *NamingMasterPrompt) Prompts() []mcp.PromptDefinition {
	return []mcp.PromptDefinition{
		{
			Prompt: &mcp.Prompt{
				Name:        "naming-master",
				Description: "Senior IT naming expert for code identifiers and database objects. Provides professional naming schemes following industry standards for multiple languages (Java, TypeScript, Go, Rust, Python) and databases (PostgreSQL, MySQL, SQLite). Includes database design guidance on audit fields, indexes, constraints, and foreign key strategies.",
			},
			Handler: p.handleNamingMasterPrompt,
		},
	}
}

// handleNamingMasterPrompt handles the naming master prompt request.
func (*NamingMasterPrompt) handleNamingMasterPrompt(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "Professional IT naming expert for code and database naming conventions",
		Messages: []*mcp.PromptMessage{
			{
				Role:    mcp.Role("user"),
				Content: &mcp.TextContent{Text: namingMasterPromptContent},
			},
		},
	}, nil
}
