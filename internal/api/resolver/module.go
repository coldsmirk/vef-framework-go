package resolver

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:api:resolver",
	fx.Provide(
		fx.Annotate(
			NewRest,
			fx.ResultTags(`group:"vef:api:handler_resolvers"`),
		),
		fx.Annotate(
			NewRPC,
			fx.ResultTags(`group:"vef:api:handler_resolvers"`),
		),
	),
)
