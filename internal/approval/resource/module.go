package resource

import (
	"go.uber.org/fx"
)

// Module provides all approval API resources.
var Module = fx.Module(
	"vef:approval:resource",

	fx.Provide(
		fx.Annotate(NewFlowResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewInstanceResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewCategoryResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewDelegationResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewMyResource, fx.ResultTags(`group:"vef:api:resources"`)),
		fx.Annotate(NewAdminResource, fx.ResultTags(`group:"vef:api:resources"`)),
	),
)
