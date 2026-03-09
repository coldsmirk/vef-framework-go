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

// GetFlowGraphQuery retrieves the flow graph for a published flow.
type GetFlowGraphQuery struct {
	cqrs.BaseQuery

	FlowID   string
	TenantID string
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

	var version approval.FlowVersion

	if err := db.NewSelect().
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
