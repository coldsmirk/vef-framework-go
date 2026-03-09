package middleware

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:api:middleware",
	fx.Provide(
		fx.Private,
		fx.Annotate(
			NewAudit,
			fx.ResultTags(`group:"vef:api:middlewares"`),
		),
		fx.Annotate(
			NewAuth,
			fx.ResultTags(`group:"vef:api:middlewares"`),
		),
		fx.Annotate(
			NewContextual,
			fx.ResultTags(`group:"vef:api:middlewares"`),
		),
		fx.Annotate(
			NewDataPermission,
			fx.ResultTags(`group:"vef:api:middlewares"`),
		),
		fx.Annotate(
			NewRateLimit,
			fx.ResultTags(`group:"vef:api:middlewares"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewChain,
			fx.ParamTags(`group:"vef:api:middlewares"`),
		),
	),
)
