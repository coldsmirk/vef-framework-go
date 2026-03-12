package query

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// dedup returns a deduplicated copy of the given string slice.
func dedup(ids []string) []string {
	s := slices.Clone(ids)
	slices.Sort(s)

	return slices.Compact(s)
}

// loadFlowMap loads flows by IDs and returns a map keyed by flow ID.
func loadFlowMap(ctx context.Context, db orm.DB, flowIDs []string) (map[string]*approval.Flow, error) {
	if len(flowIDs) == 0 {
		return nil, nil
	}

	var flows []approval.Flow
	if err := db.NewSelect().Model(&flows).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", dedup(flowIDs)) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flows: %w", err)
	}

	m := make(map[string]*approval.Flow, len(flows))
	for i := range flows {
		m[flows[i].ID] = &flows[i]
	}

	return m, nil
}

// loadNodeNameMap loads flow node names by IDs and returns a map keyed by node ID.
func loadNodeNameMap(ctx context.Context, db orm.DB, nodeIDs []string) (map[string]string, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	var nodes []approval.FlowNode
	if err := db.NewSelect().Model(&nodes).
		Select("id", "name").
		Where(func(cb orm.ConditionBuilder) { cb.In("id", dedup(nodeIDs)) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		m[n.ID] = n.Name
	}

	return m, nil
}

// loadInstanceMap loads instances by IDs and returns a map keyed by instance ID.
func loadInstanceMap(ctx context.Context, db orm.DB, instanceIDs []string) (map[string]*approval.Instance, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}

	var instances []approval.Instance
	if err := db.NewSelect().Model(&instances).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", dedup(instanceIDs)) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query instances: %w", err)
	}

	m := make(map[string]*approval.Instance, len(instances))
	for i := range instances {
		m[instances[i].ID] = &instances[i]
	}

	return m, nil
}
