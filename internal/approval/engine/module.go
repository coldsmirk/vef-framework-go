package engine

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/cache"
)

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

		// Lifecycle hooks aggregator: collects host-registered hooks via FX group.
		fx.Annotate(
			NewLifecycleHookRunner,
			fx.ParamTags(`group:"vef:approval:lifecycle_hooks"`),
		),

		// CompiledFlow cache backed by the in-process memory store. Hosts
		// that need cross-node sharing (rare for immutable flow versions)
		// can fx.Replace with a cache.NewRedis[*CompiledFlow](...).
		newDefaultCompiledFlowCache,
		NewFlowCache,

		// Flow engine
		fx.Annotate(
			NewFlowEngine,
			fx.ParamTags(``, `group:"vef:approval:node_processors"`, ``, ``, ``, ``),
		),
	),
)

func newDefaultCompiledFlowCache() cache.Cache[*CompiledFlow] {
	return cache.NewMemory[*CompiledFlow]()
}
