package param

import (
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// Handler param resolver constructors

func NewCtxResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) fiber.Ctx { return ctx })
}

func NewDBResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) orm.DB { return contextx.DB(ctx) })
}

func NewLoggerResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) log.Logger { return contextx.Logger(ctx) })
}

func NewPrincipalResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) *security.Principal { return contextx.Principal(ctx) })
}

func NewSchedulerResolver(scheduler cron.Scheduler) api.HandlerParamResolver {
	return newHandlerValueResolver(scheduler)
}

func NewPublisherResolver(publisher event.Publisher) api.HandlerParamResolver {
	return newHandlerValueResolver(publisher)
}

func NewTransformerResolver(transformer mold.Transformer) api.HandlerParamResolver {
	return newHandlerValueResolver(transformer)
}

func NewStorageResolver(service storage.Service) api.HandlerParamResolver {
	return newHandlerValueResolver(service)
}

func NewParamsResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) api.Params {
		if req := shared.Request(ctx); req != nil && req.Params != nil {
			return req.Params
		}

		return api.Params{}
	})
}

func NewMetaResolver() api.HandlerParamResolver {
	return newContextResolver(func(ctx fiber.Ctx) api.Meta {
		if req := shared.Request(ctx); req != nil && req.Meta != nil {
			return req.Meta
		}

		return api.Meta{}
	})
}

// Factory param resolver constructors

func NewDBFactoryResolver(db orm.DB) api.FactoryParamResolver {
	return newFactoryValueResolver(db)
}

func NewSchedulerFactoryResolver(scheduler cron.Scheduler) api.FactoryParamResolver {
	return newFactoryValueResolver(scheduler)
}

func NewPublisherFactoryResolver(publisher event.Publisher) api.FactoryParamResolver {
	return newFactoryValueResolver(publisher)
}

func NewTransformerFactoryResolver(transformer mold.Transformer) api.FactoryParamResolver {
	return newFactoryValueResolver(transformer)
}

func NewStorageFactoryResolver(service storage.Service) api.FactoryParamResolver {
	return newFactoryValueResolver(service)
}

var Module = fx.Module(
	"vef:api:param",
	fx.Provide(
		fx.Private,
		// Handler param resolvers
		fx.Annotate(
			NewCtxResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewDBResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewLoggerResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewPrincipalResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewSchedulerResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewPublisherResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewTransformerResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewStorageResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewParamsResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewMetaResolver,
			fx.ResultTags(`group:"vef:api:handler_param_resolvers"`),
		),
		// Factory param resolvers
		fx.Annotate(
			NewDBFactoryResolver,
			fx.ResultTags(`group:"vef:api:factory_param_resolvers"`),
		),
		fx.Annotate(
			NewSchedulerFactoryResolver,
			fx.ResultTags(`group:"vef:api:factory_param_resolvers"`),
		),
		fx.Annotate(
			NewPublisherFactoryResolver,
			fx.ResultTags(`group:"vef:api:factory_param_resolvers"`),
		),
		fx.Annotate(
			NewTransformerFactoryResolver,
			fx.ResultTags(`group:"vef:api:factory_param_resolvers"`),
		),
		fx.Annotate(
			NewStorageFactoryResolver,
			fx.ResultTags(`group:"vef:api:factory_param_resolvers"`),
		),
	),
	fx.Provide(
		fx.Annotate(
			NewHandlerParamResolverManager,
			fx.ParamTags(`group:"vef:api:handler_param_resolvers"`),
		),
		fx.Annotate(
			NewFactoryParamResolverManager,
			fx.ParamTags(`group:"vef:api:factory_param_resolvers"`),
		),
	),
)
