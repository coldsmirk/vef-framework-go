package integration

import "context"

// RouteResolver maps a route key (tenant, branch, hospital area) to the code
// of the system that should serve a contract. The framework default resolves
// through the itg_route table — exact (key, contract) rules win over
// contract-wildcard rules, and the empty key is the default route; replace it
// via fx.Decorate when routing lives elsewhere (a tenant registry, a config
// center).
type RouteResolver interface {
	// Resolve returns the code of the system serving contract for routeKey.
	// A key matching no rule fails with ErrRouteNotFound.
	Resolve(ctx context.Context, contract, routeKey string) (string, error)
}
