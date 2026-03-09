package engine

import "go.uber.org/fx"

// Module provides the flow engine and node processors.
var Module = fx.Module(
	"vef:approval:engine",

	// Node processors
	fx.Provide(
		fx.Annotate(NewStartProcessor, fx.ResultTags(`group:"vef:approval:node_processors"`)),
		fx.Annotate(NewEndProcessor, fx.ResultTags(`group:"vef:approval:node_processors"`)),
		fx.Annotate(NewConditionProcessor, fx.ResultTags(`group:"vef:approval:node_processors"`)),
		fx.Annotate(NewApprovalProcessor, fx.As(new(NodeProcessor)), fx.ResultTags(`group:"vef:approval:node_processors"`)),
		fx.Annotate(NewHandleProcessor, fx.As(new(NodeProcessor)), fx.ResultTags(`group:"vef:approval:node_processors"`)),
		fx.Annotate(NewCCProcessor, fx.As(new(NodeProcessor)), fx.ResultTags(`group:"vef:approval:node_processors"`)),

		// Flow engine
		fx.Annotate(
			NewFlowEngine,
			fx.ParamTags(``, `group:"vef:approval:node_processors"`, ``, ``),
		),
	),
)
