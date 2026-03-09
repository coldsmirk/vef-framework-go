package tools

import (
	"go.uber.org/fx"
)

// Module provides MCP built-in tools.
var Module = fx.Options(
	fx.Provide(
		fx.Annotate(
			NewQueryTool,
			fx.ResultTags(`group:"vef:mcp:tools"`),
		),
	),
)
