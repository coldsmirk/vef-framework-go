//nolint:revive // package name is intentional
package api

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/adapter"
	"github.com/coldsmirk/vef-framework-go/internal/api/auth"
	"github.com/coldsmirk/vef-framework-go/internal/api/collector"
	"github.com/coldsmirk/vef-framework-go/internal/api/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/api/param"
	"github.com/coldsmirk/vef-framework-go/internal/api/resolver"
	"github.com/coldsmirk/vef-framework-go/internal/api/router"
)

var Module = fx.Module(
	"vef:api",
	auth.Module,
	middleware.Module,
	router.Module,
	collector.Module,
	resolver.Module,
	adapter.Module,
	param.Module,
	fx.Provide(provideEngine),
)

type EngineParams struct {
	fx.In

	Resources        []api.Resource       `group:"vef:api:resources"`
	RouterStrategies []api.RouterStrategy `group:"vef:api:router_strategies"`

	OperationsCollectors []api.OperationsCollector `group:"vef:api:operations_collectors"`
	HandlerResolvers     []api.HandlerResolver     `group:"vef:api:handler_resolvers"`
	HandlerAdapters      []api.HandlerAdapter      `group:"vef:api:handler_adapters"`
}

// provideEngine creates the API engine.
func provideEngine(p EngineParams) (api.Engine, error) {
	eng, err := NewEngine(
		WithRouters(p.RouterStrategies...),
		WithHandlerAdapters(p.HandlerAdapters...),
		WithHandlerResolvers(p.HandlerResolvers...),
		WithOperationCollectors(p.OperationsCollectors...),
	)
	if err != nil {
		return nil, err
	}

	if err := eng.Register(p.Resources...); err != nil {
		return nil, err
	}

	return eng, nil
}
