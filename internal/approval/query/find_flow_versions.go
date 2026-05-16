package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FindFlowVersionsQuery queries flow versions for a specific flow.
type FindFlowVersionsQuery struct {
	cqrs.BaseQuery

	FlowID   string
	TenantID *string
	Caller   approval.CallerContext
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

	// Authorize before disclosing any data. Returning an empty slice (rather
	// than ErrFlowNotFound) for both missing and out-of-tenant flows keeps
	// the response shape uniform so callers can't distinguish "does not
	// exist" from "exists in another tenant".
	var flow approval.Flow

	flow.ID = query.FlowID

	err := db.NewSelect().
		Model(&flow).
		Select("tenant_id").
		WherePK().
		Scan(ctx)
	switch {
	case result.IsRecordNotFound(err):
		// Indistinguishable response for "no such flow" and "exists but
		// outside caller tenant" so the API does not reveal cross-tenant
		// existence to a probing caller.
		return []approval.FlowVersion{}, nil

	case err != nil:
		return nil, fmt.Errorf("load flow for tenant check: %w", err)
	}

	if authErr := query.Caller.Authorize(flow.TenantID); authErr != nil {
		// Indistinguishable from "no such flow" on purpose — see comment
		// above. The auth failure is intentionally swallowed.
		return []approval.FlowVersion{}, nil //nolint:nilerr // tenant isolation requires opaque response
	}

	if query.TenantID != nil && *query.TenantID != flow.TenantID {
		return []approval.FlowVersion{}, nil
	}

	var versions []approval.FlowVersion
	if err := db.NewSelect().
		Model(&versions).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID)
		}).
		OrderByDesc("version").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow versions: %w", err)
	}

	return versions, nil
}
