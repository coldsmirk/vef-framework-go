package middleware

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:middleware",
	fx.Provide(
		fx.Annotate(
			NewRequestIDMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewLoggerMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewRecoveryMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewRequestRecordMiddleware,
			fx.ParamTags(`group:"vef:spa"`),
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewCorsMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewContentTypeMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewCompressionMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewHeadersMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			NewSPAMiddleware,
			fx.ParamTags(`group:"vef:spa"`),
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
	),
)
