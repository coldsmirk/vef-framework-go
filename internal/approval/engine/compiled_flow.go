package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// CompiledFlow is the in-memory, query-free representation of a published
// flow version: every FlowNode keyed by ID, and the outgoing edges grouped
// by source node so engine traversal becomes a series of map lookups.
//
// FlowVersion rows are immutable once published (archive only flips status;
// nodes & edges never change), so a CompiledFlow can be cached for the
// lifetime of the version with no invalidation beyond explicit drops on
// version replacement.
type CompiledFlow struct {
	// FlowVersionID is the key under which this compilation was cached.
	FlowVersionID string
	// Nodes indexes FlowNode by ID.
	Nodes map[string]*approval.FlowNode
	// StartNode points to the unique start node of the flow (engine
	// traversal entry point).
	StartNode *approval.FlowNode
	// EdgesBySource groups outgoing edges by source node ID. Slice order
	// is undefined; callers needing branching look up via SourceHandle.
	EdgesBySource map[string][]*approval.FlowEdge
}

// FindOutgoing returns the first edge from sourceNodeID matching branchID
// (when branchID is nil, the unique unguarded out-edge). Returns
// ErrNoMatchingEdge / errAmbiguousEdges to keep behavior identical with
// the legacy DB-driven path so callers don't change their error handling.
func (c *CompiledFlow) FindOutgoing(sourceNodeID string, branchID *string) (*approval.FlowEdge, error) {
	candidates := c.EdgesBySource[sourceNodeID]
	if len(candidates) == 0 {
		return nil, ErrNoMatchingEdge
	}

	var matches []*approval.FlowEdge

	for _, edge := range candidates {
		if branchID == nil {
			matches = append(matches, edge)

			continue
		}

		if edge.SourceHandle != nil && *edge.SourceHandle == *branchID {
			matches = append(matches, edge)
		}
	}

	if len(matches) == 0 {
		return nil, ErrNoMatchingEdge
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("%w: %d edges from node %q", errAmbiguousEdges, len(matches), sourceNodeID)
	}

	return matches[0], nil
}

// FlowCache provides cached access to CompiledFlow keyed by FlowVersionID.
// Hosts can swap the underlying cache.Cache (memory or Redis); the
// approval module ships memory by default.
type FlowCache struct {
	db    orm.DB
	cache cache.Cache[*CompiledFlow]
}

// NewFlowCache constructs a FlowCache backed by the given cache.Cache.
// Pass cache.NewMemory[*CompiledFlow]() for a single-node deployment.
func NewFlowCache(db orm.DB, c cache.Cache[*CompiledFlow]) *FlowCache {
	return &FlowCache{db: db, cache: c}
}

// Get returns the compiled flow for versionID, hitting cache first and
// compiling from the database on miss. Concurrent misses for the same
// version coalesce into a single load via cache.GetOrLoad's singleflight.
func (c *FlowCache) Get(ctx context.Context, versionID string) (*CompiledFlow, error) {
	return c.cache.GetOrLoad(ctx, versionID, func(ctx context.Context) (*CompiledFlow, error) {
		return c.compile(ctx, versionID)
	})
}

// Invalidate evicts the cached compilation for a version. Called by
// publish_version when superseding a previously-published version (its
// status flips to archived, but in-flight instances keep their own
// FlowVersionID so no eviction is strictly required for them — the entry
// just becomes a one-off historical reference).
func (c *FlowCache) Invalidate(ctx context.Context, versionID string) error {
	return c.cache.Delete(ctx, versionID)
}

func (c *FlowCache) compile(ctx context.Context, versionID string) (*CompiledFlow, error) {
	var nodes []approval.FlowNode

	if err := c.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", versionID)
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load flow nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrFlowNoNodes, versionID)
	}

	var edges []approval.FlowEdge

	if err := c.db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", versionID)
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load flow edges: %w", err)
	}

	compiled := &CompiledFlow{
		FlowVersionID: versionID,
		Nodes:         make(map[string]*approval.FlowNode, len(nodes)),
		EdgesBySource: make(map[string][]*approval.FlowEdge),
	}

	for i := range nodes {
		node := &nodes[i]
		compiled.Nodes[node.ID] = node

		if node.Kind == approval.NodeStart {
			compiled.StartNode = node
		}
	}

	for i := range edges {
		edge := &edges[i]
		compiled.EdgesBySource[edge.SourceNodeID] = append(compiled.EdgesBySource[edge.SourceNodeID], edge)
	}

	return compiled, nil
}
