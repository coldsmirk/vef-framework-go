package expression

import (
	"go.uber.org/fx"
)

// Module wires the backend-agnostic expression integrations: an API handler
// parameter resolver and a mold field transformer. It requires an
// expression.Engine supplied by a backend module (e.g. expression/zen).
var Module = fx.Module(
	"vef:expression",
	fx.Provide(
		fx.Annotate(
			NewEngineResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewFieldTransformer,
			fx.ResultTags(`group:"vef:mold:field_transformers"`),
		),
	),
)
