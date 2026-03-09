package collector

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:api:collector",
	fx.Provide(
		fx.Annotate(
			NewResourceProviderCollector,
			fx.ResultTags(`group:"vef:api:operations_collectors"`),
		),
		fx.Annotate(
			NewEmbeddedProviderCollector,
			fx.ResultTags(`group:"vef:api:operations_collectors"`),
		),
	),
)
