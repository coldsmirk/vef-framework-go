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

func (h *FindFlowVersionsHandler) Handle(ctx context.Context, query FindFlowVersionsQuery) ([]shared.FlowVersionSummary, error) {
	db := contextx.DB(ctx, h.db)

	// Authorize before disclosing any data. An empty slice (rather than an
	// error) for a missing or out-of-tenant flow keeps the response uniform so
	// callers cannot distinguish "does not exist" from "exists in another tenant".
	ok, err := authorizeFlowDisclosure(ctx, db, query.FlowID, query.TenantID, query.Caller)
	if err != nil {
		return nil, err
	}

	if !ok {
		return []shared.FlowVersionSummary{}, nil
	}

	// The version list renders metadata only, so the definition payloads
	// (flow_schema / form_schema / form_fields) are never loaded — they grow
	// with the form and would be fetched once per version.
	var versions []approval.FlowVersion
	if err := db.NewSelect().
		Model(&versions).
		Select("id", "flow_id", "version", "status", "description", "storage_mode",
			"published_at", "published_by", "created_at", "created_by").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID)
		}).
		OrderByDesc("version").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow versions: %w", err)
	}

	summaries := make([]shared.FlowVersionSummary, len(versions))
	for i, version := range versions {
		summaries[i] = shared.FlowVersionSummary{
			ID:          version.ID,
			FlowID:      version.FlowID,
			Version:     version.Version,
			Status:      version.Status,
			Description: version.Description,
			StorageMode: version.StorageMode,
			PublishedAt: version.PublishedAt,
			PublishedBy: version.PublishedBy,
			CreatedAt:   version.CreatedAt,
			CreatedBy:   version.CreatedBy,
		}
	}

	return summaries, nil
}
