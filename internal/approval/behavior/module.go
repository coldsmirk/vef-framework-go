package behavior

import (
	"go.uber.org/fx"
)

// Module provides all CQRS behavior middlewares for the approval module.
var Module = fx.Module(
	"vef:approval:behavior",

	fx.Provide(
		fx.Annotate(
			NewTransactionBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
	),
)
