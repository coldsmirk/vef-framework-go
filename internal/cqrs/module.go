package cqrs

import "go.uber.org/fx"

// Module provides the CQRS Bus to the DI container.
var Module = fx.Module(
	"vef:cqrs",
	fx.Provide(
		fx.Annotate(
			NewBus,
			fx.ParamTags(`group:"vef:cqrs:behaviors"`),
		),
	),
)
