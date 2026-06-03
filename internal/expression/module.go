package expression

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/expression/zen"
)

// Module wires the expression feature: the Zen-backed engine plus the API
// handler parameter resolver and the mold field transformer. It is re-exported
// through the public expression/zen package.
var Module = fx.Module(
	"vef:expression",
	fx.Provide(
		zen.New,
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
