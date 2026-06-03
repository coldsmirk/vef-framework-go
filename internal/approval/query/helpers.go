package query

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// instanceDetailBundle holds the full set of related records needed to build an
// instance-detail DTO. Both GetAdminInstanceDetailHandler and
// GetMyInstanceDetailHandler load the same five queries; this struct lets them
// share a single loadInstanceDetailBundle helper and apply their own auth gate
// and DTO projection on top.
type instanceDetailBundle struct {
	Instance    approval.Instance
	Flow        approval.Flow
	Tasks       []approval.Task
	ActionLogs  []approval.ActionLog
	FlowNodes   []approval.FlowNode
	NodeNameMap map[string]string
}

// loadInstanceDetailBundle loads the instance identified by instanceID together
// with its flow, tasks, action logs, flow nodes, and a node-name lookup map.
// It returns (nil, shared.ErrInstanceNotFound) when the instance does not exist,
// so callers can apply their own auth gate before or after this call.
func loadInstanceDetailBundle(ctx context.Context, db orm.DB, instanceID string) (*instanceDetailBundle, error) {
	var instance approval.Instance

	instance.ID = instanceID

	if err := db.NewSelect().Model(&instance).WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("query instance: %w", err)
	}

	var flow approval.Flow

	flow.ID = instance.FlowID
	if err := db.NewSelect().Model(&flow).WherePK().Scan(ctx); err != nil && !result.IsRecordNotFound(err) {
		return nil, fmt.Errorf("query flow: %w", err)
	}

	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	var actionLogs []approval.ActionLog
	if err := db.NewSelect().Model(&actionLogs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	var flowNodes []approval.FlowNode
	if err := db.NewSelect().Model(&flowNodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", instance.FlowVersionID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	nodeNameMap := make(map[string]string, len(flowNodes))
	for _, n := range flowNodes {
		nodeNameMap[n.ID] = n.Name
	}

	return &instanceDetailBundle{
		Instance:    instance,
		Flow:        flow,
		Tasks:       tasks,
		ActionLogs:  actionLogs,
		FlowNodes:   flowNodes,
		NodeNameMap: nodeNameMap,
	}, nil
}

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

// loadPublishedFlowIDs returns the subset of flowIDs that have at least one
// published version. Uses Distinct to avoid duplicates when a flow has multiple
// published versions.
func loadPublishedFlowIDs(ctx context.Context, db orm.DB, flowIDs []string) ([]string, error) {
	var publishedFlowIDs []string

	if err := db.NewSelect().
		Model((*approval.FlowVersion)(nil)).
		Distinct().
		Select("flow_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.In("flow_id", flowIDs).
				Equals("status", approval.VersionPublished)
		}).
		Scan(ctx, &publishedFlowIDs); err != nil {
		return nil, fmt.Errorf("query published flow versions: %w", err)
	}

	return publishedFlowIDs, nil
}

// loadCategoryMap loads flow categories by IDs and returns a map keyed by category ID.
func loadCategoryMap(ctx context.Context, db orm.DB, categoryIDs []string) (map[string]*approval.FlowCategory, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}

	var categories []approval.FlowCategory
	if err := db.NewSelect().Model(&categories).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", dedup(categoryIDs)) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow categories: %w", err)
	}

	m := make(map[string]*approval.FlowCategory, len(categories))
	for i := range categories {
		m[categories[i].ID] = &categories[i]
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
