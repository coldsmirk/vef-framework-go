package integration

// RouteFindingKind classifies one routing diagnostic finding. Like
// FailureKind it is a typed vocabulary shared with management UIs, which
// render and translate findings by kind — extend it consciously and keep the
// React side in lockstep.
type RouteFindingKind string

const (
	// RouteFindingDanglingAdapter marks a contract-scoped route whose target
	// system has no enabled adapter for that contract: invoking the contract
	// through this rule fails with ErrAdapterNotFound.
	RouteFindingDanglingAdapter RouteFindingKind = "dangling_adapter"
	// RouteFindingWildcardGap marks an enabled contract a wildcard (or
	// default) route cannot serve because its target system has no enabled
	// adapter for it. Informational: not every contract is invoked through
	// every key.
	RouteFindingWildcardGap RouteFindingKind = "wildcard_gap"
	// RouteFindingDisabledSystem marks an enabled route targeting a disabled
	// system: invocations through it fail with ErrSystemDisabled.
	RouteFindingDisabledSystem RouteFindingKind = "disabled_system"
	// RouteFindingDisabledContract marks an enabled route scoped to a
	// disabled contract: the rule can never match a successful invocation.
	RouteFindingDisabledContract RouteFindingKind = "disabled_contract"
	// RouteFindingUncoveredContract marks an enabled contract that resolves
	// to no rule under a route key present in the routing table: invoking it
	// with that key fails with ErrRouteNotFound. Informational when the key
	// intentionally routes a subset of contracts.
	RouteFindingUncoveredContract RouteFindingKind = "uncovered_contract"
)

// RouteFinding is one diagnostic finding. RouteKey is always meaningful (""
// is the default route); the entity references identify the involved route,
// contract, and system by code and display name so a UI needs no extra
// lookups.
type RouteFinding struct {
	Kind         RouteFindingKind `json:"kind"`
	RouteID      string           `json:"routeId,omitempty"`
	RouteKey     string           `json:"routeKey"`
	ContractCode string           `json:"contractCode,omitempty"`
	ContractName string           `json:"contractName,omitempty"`
	SystemCode   string           `json:"systemCode,omitempty"`
	SystemName   string           `json:"systemName,omitempty"`
}

// RouteDiagnostics is the point-in-time report of the routing table's
// configuration gaps. It is computed on demand — a diagnostic pull, not a
// background monitor.
type RouteDiagnostics struct {
	Findings []RouteFinding `json:"findings"`
}
