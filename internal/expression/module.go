package expression

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/expression/zen"
)

// Module wires the expression feature: the Zen-backed engine plus the API
// handler parameter resolver and the mold field transformer. The engine is
// provided only as the public expression.Engine contract, so consumers depend
// on the interface and the backend can be swapped without touching them.
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
