package query

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/result"
)

// defaultListPageSize is the default page size for the approval list queries.
// They share one value (rather than orm.Paginate's framework default of 15)
// so admin / my list endpoints page uniformly.
const defaultListPageSize = 20

// applyPageable normalizes the pageable to defaultListPageSize and applies the
// resulting Limit/Offset to sq. It collapses the Normalize + Limit/Offset pair
// that every approval list handler repeats verbatim; each handler keeps its own
// ScanAndCount, empty-guard, and page.New since those legitimately differ by
// the result DTO type and error context.
func applyPageable(sq orm.SelectQuery, pageable *page.Pageable) orm.SelectQuery {
	pageable.Normalize(defaultListPageSize)

	return sq.Limit(pageable.Size).Offset(pageable.Offset())
}

// instanceDetailBundle holds the full set of related records needed to build an
// instance-detail DTO. Both GetAdminInstanceDetailHandler and
// GetMyInstanceDetailHandler load the same record set; this struct lets them
// share a single loadInstanceDetailBundle helper and apply their own auth gate
// and DTO projection on top.
type instanceDetailBundle struct {
	Instance    approval.Instance
	Flow        approval.Flow
	Tasks       []approval.Task
	ActionLogs  []approval.ActionLog
	FlowNodes   []approval.FlowNode
	NodeNameMap map[string]string
	// Visits is the engine-recorded traversal trail in sequence order — the
	// authoritative source for the timeline and flow-graph progress.
	Visits []approval.NodeVisit
	// CCRecords and UrgeRecords feed the timeline's CC recipient lists and
	// urge activities, both in chronological order.
	CCRecords   []approval.CCRecord
	UrgeRecords []approval.UrgeRecord
	// FormSchema is the form definition snapshot pinned to the instance's own
	// FlowVersionID, so a detail view renders form data against the exact
	// schema the instance was submitted under — not whatever version is
	// published now. Nil when the flow has no form or the version is missing.
	FormSchema *approval.FormDefinition
	// FlowSchema is the React Flow graph definition (node positions + edges)
	// pinned to the same version, used to build the read-only progress graph.
	// Nil when the version has no graph or is missing.
	FlowSchema *approval.FlowDefinition
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

	// Load the form + flow schema snapshots from the instance's own version;
	// the rest of the version row is not needed here.
	var version approval.FlowVersion

	version.ID = instance.FlowVersionID
	if err := db.NewSelect().Model(&version).Select("form_schema", "flow_schema").WherePK().Scan(ctx); err != nil && !result.IsRecordNotFound(err) {
		return nil, fmt.Errorf("query flow version: %w", err)
	}

	// Secondary "id" ordering keeps every list deterministic when the primary
	// key ties (same sort_order, or same-second timestamps on dialects with
	// second precision) — ids are XIDs, which sort by creation time.
	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sort_order", "id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	var actionLogs []approval.ActionLog
	if err := db.NewSelect().Model(&actionLogs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("created_at", "id").
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

	var visits []approval.NodeVisit
	if err := db.NewSelect().Model(&visits).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sequence").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query node visits: %w", err)
	}

	var ccRecords []approval.CCRecord
	if err := db.NewSelect().Model(&ccRecords).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("created_at", "id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query cc records: %w", err)
	}

	var urgeRecords []approval.UrgeRecord
	if err := db.NewSelect().Model(&urgeRecords).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("created_at", "id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query urge records: %w", err)
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
		Visits:      visits,
		CCRecords:   ccRecords,
		UrgeRecords: urgeRecords,
		FormSchema:  version.FormSchema,
		FlowSchema:  version.FlowSchema,
	}, nil
}

// scopeCCByTenant returns the CC→instance tenant-scoping closure shared by the
// cc-records and pending-counts queries: it joins apv_instance (alias "i") on
// instance_id and filters i.tenant_id. Apply it via ApplyIf so the nil guard
// stays at the call site (the closure dereferences tenantID). Keeping the join
// in one place stops the two queries' tenant-scoping semantics from drifting.
func scopeCCByTenant(tenantID *string) func(orm.SelectQuery) {
	return func(sq orm.SelectQuery) {
		sq.Join((*approval.Instance)(nil), func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("instance_id", "i.id")
		}, "i").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("i.tenant_id", *tenantID)
			})
	}
}

// dedup returns a deduplicated copy of the given string slice.
func dedup(ids []string) []string {
	s := slices.Clone(ids)
	slices.Sort(s)

	return slices.Compact(s)
}

// toAdminActionLog projects a persisted action log into the admin API shape.
// Shared by the admin instance-detail and the admin action-log list so the two
// projections cannot drift.
func toAdminActionLog(log approval.ActionLog) admin.ActionLog {
	return admin.ActionLog{
		LogID:            log.ID,
		Action:           string(log.Action),
		NodeID:           log.NodeID,
		TaskID:           log.TaskID,
		Operator:         log.Operator(),
		TransferTo:       log.TransferTo(),
		RollbackToNodeID: log.RollbackToNodeID,
		AddedAssignees:   log.AddedAssignees,
		RemovedAssignees: log.RemovedAssignees,
		CCUsers:          log.CCUsers,
		Opinion:          log.Opinion,
		Attachments:      log.Attachments,
		CreatedAt:        log.CreatedAt,
	}
}

// loadByIDs loads rows of model type M whose id is in ids (deduplicated) and
// returns them as a map keyed by keyOf. The id column is matched literally, so
// every model must store its identifier there (true for all approval models —
// the id lives in the embedded orm audit models). bun resolves the table from
// the slice element type, the same generic Model(&slice) pattern crud uses.
func loadByIDs[M any](ctx context.Context, db orm.DB, ids []string, keyOf func(*M) string) (map[string]*M, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []M
	if err := db.NewSelect().Model(&rows).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", dedup(ids)) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query %T by ids: %w", rows, err)
	}

	m := make(map[string]*M, len(rows))
	for i := range rows {
		row := &rows[i]
		m[keyOf(row)] = row
	}

	return m, nil
}

// loadFlowMap loads flows by IDs and returns a map keyed by flow ID.
func loadFlowMap(ctx context.Context, db orm.DB, flowIDs []string) (map[string]*approval.Flow, error) {
	return loadByIDs(ctx, db, flowIDs, func(f *approval.Flow) string { return f.ID })
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

// loadEnrichmentMaps performs the three batched loads that the task- and
// CC-list handlers all share: instances by ID, then the flows those instances
// reference, then node names. Callers keep their own ID-collection loop (which
// differs by source type — Task.NodeID is a string, CCRecord.NodeID a *string)
// and pass the collected slices here, sharing the assembly that an N+1 guard
// or batching change would otherwise have to edit in four places.
func loadEnrichmentMaps(ctx context.Context, db orm.DB, instanceIDs, nodeIDs []string) (
	instanceMap map[string]*approval.Instance,
	flowMap map[string]*approval.Flow,
	nodeMap map[string]string,
	err error,
) {
	instanceMap, err = loadInstanceMap(ctx, db, instanceIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	flowIDs := make([]string, 0, len(instanceMap))
	for _, inst := range instanceMap {
		flowIDs = append(flowIDs, inst.FlowID)
	}

	flowMap, err = loadFlowMap(ctx, db, flowIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	nodeMap, err = loadNodeNameMap(ctx, db, nodeIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	return instanceMap, flowMap, nodeMap, nil
}

// loadInstanceEnrichment hydrates an instance list: it collects the flow IDs
// and current-node IDs from instances, then loads the flow map and node-name
// map. Shared by the my-initiated and admin-instances list handlers, which
// resolve the same flow + current-node-name columns.
func loadInstanceEnrichment(ctx context.Context, db orm.DB, instances []approval.Instance) (
	flowMap map[string]*approval.Flow,
	nodeMap map[string]string,
	err error,
) {
	flowIDs := make([]string, 0, len(instances))

	nodeIDs := make([]string, 0, len(instances))
	for i := range instances {
		flowIDs = append(flowIDs, instances[i].FlowID)
		if instances[i].CurrentNodeID != nil {
			nodeIDs = append(nodeIDs, *instances[i].CurrentNodeID)
		}
	}

	flowMap, err = loadFlowMap(ctx, db, flowIDs)
	if err != nil {
		return nil, nil, err
	}

	nodeMap, err = loadNodeNameMap(ctx, db, nodeIDs)
	if err != nil {
		return nil, nil, err
	}

	return flowMap, nodeMap, nil
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
	return loadByIDs(ctx, db, categoryIDs, func(c *approval.FlowCategory) string { return c.ID })
}

// loadInstanceMap loads instances by IDs and returns a map keyed by instance ID.
func loadInstanceMap(ctx context.Context, db orm.DB, instanceIDs []string) (map[string]*approval.Instance, error) {
	return loadByIDs(ctx, db, instanceIDs, func(i *approval.Instance) string { return i.ID })
}
