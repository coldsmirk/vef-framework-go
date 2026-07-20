package exec

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// tableRouteResolver is the default RouteResolver, backed by the itg_route
// table: an exact (route key, contract) rule wins over a contract-wildcard
// rule, and the empty route key serves as the default route.
type tableRouteResolver struct {
	db orm.DB
}

// NewTableRouteResolver builds the table-backed route resolver. Replace it
// via fx.Decorate when routing lives elsewhere.
func NewTableRouteResolver(db orm.DB) integration.RouteResolver {
	return &tableRouteResolver{db: db}
}

func (r *tableRouteResolver) Resolve(ctx context.Context, contract, routeKey string) (string, error) {
	contractID, err := r.contractID(ctx, contract)
	if err != nil {
		return "", err
	}

	var routes []integration.Route

	err = r.db.NewSelect().
		Model(&routes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("route_key", routeKey).
				In("contract_id", []string{contractID, ""}).
				Equals("is_enabled", true)
		}).
		Scan(ctx)
	if err != nil {
		return "", err
	}

	route := pickRoute(routes, contractID)
	if route == nil {
		return "", integration.ErrRouteNotFound
	}

	return r.systemCode(ctx, route.SystemID)
}

// contractID resolves the contract code to its ID for the route match; a
// missing contract degrades to wildcard-only matching and surfaces as
// contract-not-found later in the pipeline.
func (r *tableRouteResolver) contractID(ctx context.Context, contract string) (string, error) {
	found, err := definition.FindOne[integration.Contract](ctx, r.db, result.ErrRecordNotFound,
		func(cb orm.ConditionBuilder) {
			cb.Equals("code", contract)
		})
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return "", nil
		}

		return "", err
	}

	return found.ID, nil
}

// pickRoute selects the winning rule: exact contract scope over wildcard.
func pickRoute(routes []integration.Route, contractID string) *integration.Route {
	var wildcard *integration.Route

	for i := range routes {
		route := &routes[i]

		if contractID != "" && route.ContractID == contractID {
			return route
		}

		if route.ContractID == "" {
			wildcard = route
		}
	}

	return wildcard
}

// systemCode resolves the routed system's code, which the invoker loads by
// code afterwards.
func (r *tableRouteResolver) systemCode(ctx context.Context, systemID string) (string, error) {
	system, err := definition.FindOne[integration.System](ctx, r.db, integration.ErrRouteNotFound,
		func(cb orm.ConditionBuilder) {
			cb.Equals("id", systemID)
		})
	if err != nil {
		return "", err
	}

	return system.Code, nil
}
