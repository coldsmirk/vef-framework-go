package schema

import (
	"go.uber.org/fx"
)

// Module is the FX module for schema inspection functionality.
var Module = fx.Module(
	"vef:schema",
	fx.Provide(
		NewService,
		fx.Annotate(
			NewResource,
			fx.ResultTags(`group:"vef:api:resources"`),
		),
	),
)
