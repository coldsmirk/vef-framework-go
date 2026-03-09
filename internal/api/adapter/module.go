package adapter

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:api:adapter",
	fx.Provide(
		fx.Annotate(
			NewFuncHandler,
			fx.ResultTags(`group:"vef:api:handler_adapters"`),
		),
		fx.Annotate(
			NewFiberHandler,
			fx.ResultTags(`group:"vef:api:handler_adapters"`),
		),
	),
)
