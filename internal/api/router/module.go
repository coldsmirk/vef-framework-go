package router

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/middleware"
)

var Module = fx.Module(
	"vef:api:router",
	fx.Provide(
		fx.Annotate(
			func(chain *middleware.Chain) api.RouterStrategy {
				return NewRPC(DefaultRPCEndpoint, chain)
			},
			fx.ResultTags(`group:"vef:api:router_strategies"`),
		),
		fx.Annotate(
			func(chain *middleware.Chain) api.RouterStrategy {
				return NewREST(DefaultRESTBasePath, chain)
			},
			fx.ResultTags(`group:"vef:api:router_strategies"`),
		),
	),
)
