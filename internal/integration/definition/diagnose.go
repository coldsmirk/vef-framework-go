package definition

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// adapterKey identifies one enabled adapter binding.
type adapterKey struct {
	systemID   string
	contractID string
}

// routingState is the definition snapshot the diagnosis walks.
type routingState struct {
	routes    []integration.Route
	contracts map[string]*integration.Contract
	systems   map[string]*integration.System
	adapters  map[adapterKey]struct{}
}

// DiagnoseRoutes analyses the routing table against contracts, systems, and
// adapters, reporting the configuration gaps that would otherwise surface
// only as runtime errors. Disabled routes are skipped — they are
// intentionally off. Definitions are config-scale, so the analysis loads
// them wholesale and walks them in memory.
func DiagnoseRoutes(ctx context.Context, db orm.DB) (*integration.RouteDiagnostics, error) {
	state, err := loadRoutingState(ctx, db)
	if err != nil {
		return nil, err
	}

	report := &integration.RouteDiagnostics{Findings: []integration.RouteFinding{}}

	for i := range state.routes {
		diagnoseRoute(state, &state.routes[i], report)
	}

	diagnoseCoverage(state, report)

	return report, nil
}

// loadRoutingState fetches every definition the diagnosis needs: enabled
// routes, plus all contracts and systems (their enabled flags are part of
// the analysis) and the enabled adapters.
func loadRoutingState(ctx context.Context, db orm.DB) (*routingState, error) {
	state := &routingState{
		contracts: make(map[string]*integration.Contract),
		systems:   make(map[string]*integration.System),
		adapters:  make(map[adapterKey]struct{}),
	}

	err := db.NewSelect().
		Model(&state.routes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("is_enabled", true)
		}).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	var contracts []integration.Contract
	if err := db.NewSelect().Model(&contracts).Scan(ctx); err != nil {
		return nil, err
	}

	for i := range contracts {
		state.contracts[contracts[i].ID] = &contracts[i]
	}

	var systems []integration.System
	if err := db.NewSelect().Model(&systems).Scan(ctx); err != nil {
		return nil, err
	}

	for i := range systems {
		state.systems[systems[i].ID] = &systems[i]
	}

	var adapters []integration.Adapter

	// Routing serves the outbound flow only, so inbound adapters must not
	// satisfy a route's serving check.
	err = db.NewSelect().
		Model(&adapters).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("is_enabled", true).
				Equals("direction", integration.DirectionOutbound)
		}).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	for _, adapter := range adapters {
		state.adapters[adapterKey{systemID: adapter.SystemID, contractID: adapter.ContractID}] = struct{}{}
	}

	return state, nil
}

// diagnoseRoute reports the gaps of one enabled route: disabled targets,
// missing adapters behind a contract-scoped rule, and the per-contract
// serving gaps behind a wildcard rule.
func diagnoseRoute(state *routingState, route *integration.Route, report *integration.RouteDiagnostics) {
	system, ok := state.systems[route.SystemID]
	if !ok {
		return
	}

	var contract *integration.Contract

	if route.ContractID != "" {
		if contract, ok = state.contracts[route.ContractID]; !ok {
			return
		}
	}

	if !system.IsEnabled {
		report.Findings = append(report.Findings, finding(integration.RouteFindingDisabledSystem, route, contract, system))
	}

	if contract != nil {
		if !contract.IsEnabled {
			report.Findings = append(report.Findings, finding(integration.RouteFindingDisabledContract, route, contract, system))
		}

		if _, ok := state.adapters[adapterKey{systemID: system.ID, contractID: contract.ID}]; !ok {
			report.Findings = append(report.Findings, finding(integration.RouteFindingDanglingAdapter, route, contract, system))
		}

		return
	}

	for _, wildcardContract := range state.contracts {
		if !wildcardContract.IsEnabled {
			continue
		}

		if _, ok := state.adapters[adapterKey{systemID: system.ID, contractID: wildcardContract.ID}]; !ok {
			report.Findings = append(report.Findings, finding(integration.RouteFindingWildcardGap, route, wildcardContract, system))
		}
	}
}

// diagnoseCoverage reports, for every route key present in the table, the
// enabled contracts that resolve to no rule: an exact rule for the contract
// or a wildcard rule under the same key covers it.
func diagnoseCoverage(state *routingState, report *integration.RouteDiagnostics) {
	exact := make(map[string]map[string]struct{})
	wildcard := make(map[string]struct{})

	for i := range state.routes {
		route := &state.routes[i]

		if route.ContractID == "" {
			wildcard[route.RouteKey] = struct{}{}

			continue
		}

		if exact[route.RouteKey] == nil {
			exact[route.RouteKey] = make(map[string]struct{})
		}

		exact[route.RouteKey][route.ContractID] = struct{}{}
	}

	for key, contracts := range exact {
		if _, ok := wildcard[key]; ok {
			continue
		}

		for _, contract := range state.contracts {
			if !contract.IsEnabled {
				continue
			}

			if _, ok := contracts[contract.ID]; ok {
				continue
			}

			report.Findings = append(report.Findings, integration.RouteFinding{
				Kind:         integration.RouteFindingUncoveredContract,
				RouteKey:     key,
				ContractCode: contract.Code,
				ContractName: contract.Name,
			})
		}
	}
}

// finding assembles one route-anchored finding with its entity snapshots.
func finding(kind integration.RouteFindingKind, route *integration.Route, contract *integration.Contract, system *integration.System) integration.RouteFinding {
	f := integration.RouteFinding{
		Kind:     kind,
		RouteID:  route.ID,
		RouteKey: route.RouteKey,
	}

	if contract != nil {
		f.ContractCode = contract.Code
		f.ContractName = contract.Name
	}

	if system != nil {
		f.SystemCode = system.Code
		f.SystemName = system.Name
	}

	return f
}
