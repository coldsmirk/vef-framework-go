package resource

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("integration")

// Module provides all integration API resources.
var Module = fx.Module(
	"vef:integration:resource",

	fx.Provide(
		fx.Annotate(NewContractResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewSystemResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewAdapterResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewRouteResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewCodeMapResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewCodeSetResource, fx.ParamTags(`optional:"true"`, `optional:"true"`), fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewLogResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewOpsResource, fx.ResultTags(`group:"vef:api:resources"`)),
	),
)
