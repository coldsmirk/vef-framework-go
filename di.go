package vef

import (
	"github.com/coldsmirk/go-streams"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/mcp"
	"github.com/coldsmirk/vef-framework-go/middleware"
)

var (
	Provide    = fx.Provide
	Supply     = fx.Supply
	Annotate   = fx.Annotate
	As         = fx.As
	ParamTags  = fx.ParamTags
	ResultTags = fx.ResultTags
	Self       = fx.Self
	Invoke     = fx.Invoke
	Decorate   = fx.Decorate
	Module     = fx.Module
	Private    = fx.Private
	OnStart    = fx.OnStart
	OnStop     = fx.OnStop
)

type (
	Hook     = fx.Hook
	HookFunc = fx.HookFunc
)

var (
	From     = fx.From
	Replace  = fx.Replace
	Populate = fx.Populate
)

type (
	In        = fx.In
	Out       = fx.Out
	Lifecycle = fx.Lifecycle
)

func StartHook[T HookFunc](start T) Hook {
	return fx.StartHook(start)
}

func StopHook[T HookFunc](stop T) Hook {
	return fx.StopHook(stop)
}

func StartStopHook[T1, T2 HookFunc](start T1, stop T2) Hook {
	return fx.StartStopHook(start, stop)
}

// ProvideAPIResource provides an API resource to the dependency injection container.
// The resource will be registered in the "vef:api:resources" group.
// The constructor must return api.Resource (not a concrete type).
func ProvideAPIResource(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:api:resources"`),
		),
	)
}

// ProvideMiddleware provides a middleware to the dependency injection container.
// The middleware will be registered in the "vef:app:middlewares" group.
// The constructor must return app.Middleware (not a concrete type).
func ProvideMiddleware(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
	)
}

// ProvideSPAConfig provides a Single Page Application configuration to the dependency injection container.
// The config will be registered in the "vef:spa" group.
func ProvideSPAConfig(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:spa"`),
		),
	)
}

// SupplySPAConfigs supplies multiple Single Page Application configurations to the dependency injection container.
// All configs will be registered in the "vef:spa" group.
func SupplySPAConfigs(config *middleware.SPAConfig, configs ...*middleware.SPAConfig) fx.Option {
	spaConfigs := make([]any, 0, len(configs)+1)

	spaConfigs = append(
		spaConfigs,
		fx.Annotate(
			config,
			fx.ResultTags(`group:"vef:spa"`),
		),
	)
	if len(configs) > 0 {
		spaConfigs = append(
			spaConfigs,
			streams.MapTo(
				streams.FromSlice(configs),
				func(cfg *middleware.SPAConfig) any {
					return fx.Annotate(
						cfg,
						fx.ResultTags(`group:"vef:spa"`),
					)
				},
			).Collect()...,
		)
	}

	return fx.Supply(spaConfigs...)
}

// ProvideCQRSBehavior provides a CQRS behavior middleware to the dependency injection container.
// The constructor must return cqrs.Behavior (not a concrete type).
func ProvideCQRSBehavior(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
	)
}

// ProvideChallengeProvider provides a login challenge provider to the dependency injection container.
// The provider will be registered in the "vef:security:challenge_providers" group.
// The constructor must return security.ChallengeProvider (not a concrete type).
func ProvideChallengeProvider(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:security:challenge_providers"`),
		),
	)
}

// ProvideMCPTools provides an MCP tool provider.
// The constructor must return mcp.ToolProvider (not a concrete type).
func ProvideMCPTools(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:mcp:tools"`),
		),
	)
}

// ProvideMCPResources provides an MCP resource provider.
// The constructor must return mcp.ResourceProvider (not a concrete type).
func ProvideMCPResources(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:mcp:resources"`),
		),
	)
}

// ProvideMCPResourceTemplates provides an MCP resource template provider.
// The constructor must return mcp.ResourceTemplateProvider (not a concrete type).
func ProvideMCPResourceTemplates(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:mcp:templates"`),
		),
	)
}

// ProvideMCPPrompts provides an MCP prompt provider.
// The constructor must return mcp.PromptProvider (not a concrete type).
func ProvideMCPPrompts(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:mcp:prompts"`),
		),
	)
}

// SupplyMCPServerInfo supplies MCP server info.
func SupplyMCPServerInfo(info *mcp.ServerInfo) fx.Option {
	return fx.Supply(info)
}

// SupplyFileACL replaces the framework-provided default storage.FileACL
// with a business-specific implementation. The default ACL is pub-only
// (reads of keys under storage.PublicPrefix are allowed; everything
// else is denied), so any application that stores private files MUST
// register its own implementation through this helper.
//
// constructor is an fx-style factory that returns storage.FileACL (or a
// type implementing it). It may declare any dependencies already
// registered in the fx graph — typically orm.DB plus any business
// services that own the reverse index from object key to owning row.
//
// Example:
//
//	type myACL struct{ db orm.DB }
//
//	func newMyACL(db orm.DB) storage.FileACL {
//	    return &myACL{db: db}
//	}
//
//	fx.New(
//	    vef.Module,
//	    vef.SupplyFileACL(newMyACL),
//	)
func SupplyFileACL(constructor any) fx.Option {
	return fx.Decorate(constructor)
}

// SupplyURLKeyMapper replaces the framework-provided default
// storage.URLKeyMapper (identity) with a business-specific
// implementation. The default mapper assumes the frontend embeds bare
// storage keys verbatim in <img src> / ![](...) constructs; applications
// that embed proxy paths (e.g. "/storage/files/<key>"), CDN URLs, or any
// other URL convention MUST register their own mapper here so meta:
// "richtext" / "markdown" reconciliation can resolve those URLs back to
// storage keys before consuming claims or scheduling deletions.
//
// constructor is an fx-style factory that returns storage.URLKeyMapper
// (or a type implementing it). It may declare any dependencies already
// registered in the fx graph.
//
// Example: stripping the framework's default proxy prefix.
//
//	type proxyURLMapper struct{}
//
//	func (proxyURLMapper) URLToKey(u string) string {
//	    return strings.TrimPrefix(u, "/storage/files/")
//	}
//
//	func (proxyURLMapper) KeyToURL(k string) string {
//	    return "/storage/files/" + k
//	}
//
//	fx.New(
//	    vef.Module,
//	    vef.SupplyURLKeyMapper(func() storage.URLKeyMapper { return proxyURLMapper{} }),
//	)
func SupplyURLKeyMapper(constructor any) fx.Option {
	return fx.Decorate(constructor)
}
