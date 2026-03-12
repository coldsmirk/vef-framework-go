package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// FindFlowVersionsQuery queries flow versions for a specific flow.
type FindFlowVersionsQuery struct {
	cqrs.BaseQuery

	FlowID   string
	TenantID *string
}

// FindFlowVersionsHandler handles the FindFlowVersionsQuery.
type FindFlowVersionsHandler struct {
	db orm.DB
}

// NewFindFlowVersionsHandler creates a new FindFlowVersionsHandler.
func NewFindFlowVersionsHandler(db orm.DB) *FindFlowVersionsHandler {
	return &FindFlowVersionsHandler{db: db}
}

func (h *FindFlowVersionsHandler) Handle(ctx context.Context, query FindFlowVersionsQuery) ([]approval.FlowVersion, error) {
	db := contextx.DB(ctx, h.db)

	if query.TenantID != nil {
		exists, err := db.NewSelect().
			Model((*approval.Flow)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(query.FlowID).
					Equals("tenant_id", *query.TenantID)
			}).
			Exists(ctx)
		if err != nil {
			return nil, fmt.Errorf("check flow tenant: %w", err)
		}

		if !exists {
			return nil, shared.ErrFlowNotFound
		}
	}

	var versions []approval.FlowVersion

	err := db.NewSelect().
		Model(&versions).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID)
		}).
		OrderByDesc("version").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("query flow versions: %w", err)
	}

	return versions, nil
}
