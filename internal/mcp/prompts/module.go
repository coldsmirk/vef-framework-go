package prompts

import (
	"go.uber.org/fx"
)

// Module provides MCP built-in prompts.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewNamingMasterPrompt,
			fx.ResultTags(`group:"vef:mcp:prompts"`),
		),
	),
)
