package auth

import "go.uber.org/fx"

var Module = fx.Module(
	"vef:api:auth",
	fx.Provide(
		fx.Private,
		fx.Annotate(
			NewAccessTokenAuthenticator,
			fx.ResultTags(`group:"vef:api:bearer_authenticators"`),
		),
		fx.Annotate(
			NewNone,
			fx.ResultTags(`group:"vef:api:auth_strategies"`),
		),
		fx.Annotate(
			NewBearer,
			fx.ParamTags(`group:"vef:api:bearer_authenticators"`),
			fx.ResultTags(`group:"vef:api:auth_strategies"`),
		),
		fx.Annotate(
			NewSignature,
			fx.ParamTags(`optional:"true"`),
			fx.ResultTags(`group:"vef:api:auth_strategies"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewRegistry,
			fx.ParamTags(`group:"vef:api:auth_strategies"`),
		),
	),
)
