package vef

import (
	"github.com/coldsmirk/go-streams"
	"go.uber.org/fx"

	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/mcp"
	"github.com/coldsmirk/vef-framework-go/middleware"
)

// ApprovalModule enables the optional approval (workflow) feature: pass it to
// vef.Run(...) to register the approval API resources, CQRS handlers, engine,
// binding listener, and timeout scanner. It is intentionally absent from the
// default boot sequence (bootmodules.Core) so applications that do not need
// workflows pay nothing. Approval events publish with event.WithTx and the
// binding listener subscribes, so the host must route approval.* to a
// transactional transport with a subscribable sink (see the approval docs).
var ApprovalModule = iapproval.Module

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

// ProvideAuthStrategy provides a custom API authentication strategy to the
// dependency injection container. The strategy will be registered in the
// "vef:api:auth_strategies" group and is selected per resource through
// api.AuthConfig.Strategy by the name it reports from Name().
// The constructor must return api.AuthStrategy (not a concrete type).
//
// Example:
//
//	type apiKeyStrategy struct{ /* ... */ }
//
//	func (*apiKeyStrategy) Name() string { return "api-key" }
//
//	func (s *apiKeyStrategy) Authenticate(ctx fiber.Ctx, options map[string]any) (*security.Principal, error) {
//	    /* validate the credential and return the principal */
//	}
//
//	func newAPIKeyStrategy(db orm.DB) api.AuthStrategy { return &apiKeyStrategy{ /* ... */ } }
//
//	fx.New(
//	    vef.Module,
//	    vef.ProvideAuthStrategy(newAPIKeyStrategy),
//	)
func ProvideAuthStrategy(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:api:auth_strategies"`),
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

// ProvideEventTransport registers a custom event Transport. The
// constructor must return event/transport.Transport (or a type that
// satisfies it).
func ProvideEventTransport(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:event:transports"`),
		),
	)
}

// ProvideEventPublishMiddleware registers a publish-side event
// middleware. The constructor must return event/middleware.PublishMiddleware.
func ProvideEventPublishMiddleware(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:event:publish-middlewares"`),
		),
	)
}

// ProvideEventConsumeMiddleware registers a consume-side event
// middleware. The constructor must return event/middleware.ConsumeMiddleware.
func ProvideEventConsumeMiddleware(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:event:consume-middlewares"`),
		),
	)
}

// ProvideEventMetricsRecorder overrides the framework's default
// (expvar-backed) event.MetricsRecorder. The constructor must return
// event.MetricsRecorder. Use this when forwarding publish/consume
// observations to Prometheus, OpenTelemetry, or a vendor SDK.
//
// Example:
//
//	fx.New(
//	    vef.Module,
//	    vef.ProvideEventMetricsRecorder(newPrometheusRecorder),
//	)
func ProvideEventMetricsRecorder(constructor any, paramTags ...string) fx.Option {
	return fx.Decorate(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
		),
	)
}

// ProvideEventErrorSink overrides the framework's default async error
// sink. The constructor must return event.ErrorSink. Useful when
// async-publish failures need to flow to a metrics or alerting system
// rather than just the logger.
func ProvideEventErrorSink(constructor any, paramTags ...string) fx.Option {
	return fx.Decorate(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
		),
	)
}

// SupplyBusinessRefProvider replaces the default no-op
// approval.BusinessRefProvider with a host implementation that resolves or
// allocates the business row during start_instance, inside the same
// transaction. Register one when the business record must come into
// existence together with the instance; callers that already know the
// reference can simply pass businessRef in the start parameters instead.
//
// The engine-owned write-back of approval outcomes is not affected by this
// hook and cannot be replaced — hosts extend around it with
// ProvideApprovalLifecycleHook or event subscriptions.
//
// constructor is an fx-style factory that returns approval.BusinessRefProvider
// (or a type implementing it). It may declare any dependencies already
// registered in the fx graph.
//
// Example:
//
//	fx.New(
//	    vef.Module,
//	    vef.SupplyBusinessRefProvider(newOrderRefProvider),
//	)
func SupplyBusinessRefProvider(constructor any) fx.Option {
	return fx.Decorate(constructor)
}

// SupplyBusinessRefResolver replaces the default identity
// approval.BusinessRefResolver. Register one when Instance.BusinessRef is
// not the bare business primary key — e.g. a composite key encoded as JSON —
// so the engine-owned write-back can still resolve the row to target in
// `WHERE pk_field = ?`.
//
// constructor is an fx-style factory that returns approval.BusinessRefResolver
// (or a type implementing it). It may declare any dependencies already
// registered in the fx graph.
//
// Example:
//
//	fx.New(
//	    vef.Module,
//	    vef.SupplyBusinessRefResolver(newCompositeRefResolver),
//	)
func SupplyBusinessRefResolver(constructor any) fx.Option {
	return fx.Decorate(constructor)
}

// ProvideApprovalLifecycleHook registers a synchronous
// approval.InstanceLifecycleHook into the FX container. Hooks run inside
// the engine transaction for OnInstanceCreated / OnInstanceCompleted, so
// returning an error rolls back the surrounding business operation.
//
// The constructor must return approval.InstanceLifecycleHook (not a
// concrete type). Multiple hooks compose via the
// `vef:approval:lifecycle_hooks` group.
func ProvideApprovalLifecycleHook(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:approval:lifecycle_hooks"`),
		),
	)
}

// ProvideApprovalAggregator registers a custom detail-table aggregator for
// approval field conditions alongside the built-in sum / count / avg. The
// constructor must return an approval.Aggregator; the condition evaluator
// picks it up by its AggregateKind with no changes to existing code.
//
//	vef.ProvideApprovalAggregator(func() approval.Aggregator { return myMedian{} })
func ProvideApprovalAggregator(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:approval:aggregators"`),
		),
	)
}

// ProvideApprovalFormSchemaParser overrides the framework's default
// approval.FormSchemaParser — the built-in vef-framework-react form-editor
// parser — with a host implementation. The replacement is wholesale, not
// additive: every deployed form schema goes through it, so it must understand
// every designer document the host submits. The constructor must return
// approval.FormSchemaParser. Parsing runs once at flow deploy; versions
// deployed earlier keep the form_fields they were persisted with.
//
// Example:
//
//	fx.New(
//	    vef.Module,
//	    vef.ProvideApprovalFormSchemaParser(newMyDesignerParser),
//	)
func ProvideApprovalFormSchemaParser(constructor any, paramTags ...string) fx.Option {
	return fx.Decorate(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
		),
	)
}

// SupplyURLKeyMapper replaces the framework-provided default
// storage.URLKeyMapper (storage.ProxyURLKeyMapper) with a
// business-specific implementation. The default mapper strips and
// prepends the framework's proxy prefix ("/storage/files/"), so it
// resolves the proxy URLs served by the built-in proxy middleware back
// to storage keys during meta:"rich_text" / "markdown" reconciliation.
// Applications that embed a different URL convention — external CDN
// URLs, or bare storage keys — register their own mapper here so those
// URLs can be resolved back to storage keys before consuming claims or
// scheduling deletions.
//
// constructor is an fx-style factory that returns storage.URLKeyMapper
// (or a type implementing it). It may declare any dependencies already
// registered in the fx graph.
//
// Example: serving files from an external CDN whose URLs embed bare
// storage keys after a fixed host prefix.
//
//	type cdnURLMapper struct{}
//
//	func (cdnURLMapper) URLToKey(u string) (key string, ok bool) {
//	    const prefix = "https://cdn.example.com/"
//	    if !strings.HasPrefix(u, prefix) {
//	        return "", false
//	    }
//	    return strings.TrimPrefix(u, prefix), true
//	}
//
//	func (cdnURLMapper) KeyToURL(k string) string {
//	    return "https://cdn.example.com/" + k
//	}
//
//	fx.New(
//	    vef.Module,
//	    vef.SupplyURLKeyMapper(func() storage.URLKeyMapper { return cdnURLMapper{} }),
//	)
func SupplyURLKeyMapper(constructor any) fx.Option {
	return fx.Decorate(constructor)
}

// ProvideDataSourceProvider registers a datasource.Provider that the
// framework consults during boot, after the primary and static (TOML)
// data sources have been registered. Every spec returned by the provider's
// Load method is passed to datasource.Registry.Register; a name collision with
// any existing source (TOML or another provider) fails boot.
//
// Typical use case: a tenant table in the primary database whose rows
// describe additional data sources. The provider reads the table during
// Load and returns one datasource.Spec per row. For periodic re-sync of
// the same table, register a cron job that calls
// datasource.Registry.Reconcile instead.
//
// constructor is an fx-style factory that returns datasource.Provider
// (or a type that implements it).
func ProvideDataSourceProvider(constructor any, paramTags ...string) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			fx.ParamTags(paramTags...),
			fx.ResultTags(`group:"vef:datasource:providers"`),
		),
	)
}
