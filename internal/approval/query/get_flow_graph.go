package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// GetFlowGraphQuery retrieves a flow's graph: the latest published version by
// default, or the version named by VersionID — the lane a designer uses to
// resume editing from the newest deployment, published or not.
type GetFlowGraphQuery struct {
	cqrs.BaseQuery

	FlowID   string
	TenantID string
	// VersionID selects an explicit version of the flow; empty resolves the
	// latest published version.
	VersionID string
	Caller    approval.CallerContext
}

// GetFlowGraphHandler handles the GetFlowGraphQuery.
type GetFlowGraphHandler struct {
	db orm.DB
}

// NewGetFlowGraphHandler creates a new GetFlowGraphHandler.
func NewGetFlowGraphHandler(db orm.DB) *GetFlowGraphHandler {
	return &GetFlowGraphHandler{db: db}
}

func (h *GetFlowGraphHandler) Handle(ctx context.Context, query GetFlowGraphQuery) (*shared.FlowGraph, error) {
	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = query.FlowID

	if err := db.NewSelect().
		Model(&flow).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(query.FlowID).
				ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", query.TenantID)
				})
		}).
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("query flow: %w", err)
	}

	if !query.Caller.Allows(flow.TenantID) {
		// Indistinguishable from "no such flow" on purpose — see opaque
		// response policy for query handlers (avoids cross-tenant
		// existence probing).
		return nil, shared.ErrFlowNotFound
	}

	var version approval.FlowVersion

	if query.VersionID != "" {
		version.ID = query.VersionID

		if err := db.NewSelect().
			Model(&version).
			WherePK().
			Scan(ctx); err != nil {
			if result.IsRecordNotFound(err) {
				return nil, shared.ErrVersionNotFound
			}

			return nil, fmt.Errorf("query version: %w", err)
		}

		// A version id under a different flow is indistinguishable from a
		// missing one, so a caller cannot probe versions across flows.
		if version.FlowID != flow.ID {
			return nil, shared.ErrVersionNotFound
		}
	} else if err := db.NewSelect().
		Model(&version).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID).
				Equals("status", string(approval.VersionPublished))
		}).
		OrderByDesc("version").
		Limit(1).
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrNoPublishedVersion
		}

		return nil, fmt.Errorf("query published version: %w", err)
	}

	var nodes []approval.FlowNode

	if err := db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}

	var edges []approval.FlowEdge

	if err := db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}

	return &shared.FlowGraph{
		Flow:    &flow,
		Version: &version,
		Nodes:   nodes,
		Edges:   edges,
	}, nil
}
