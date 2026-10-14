package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// FindFlowInitiatorsQuery queries the initiator configurations of a flow.
type FindFlowInitiatorsQuery struct {
	cqrs.BaseQuery

	FlowID string
}

// FindFlowInitiatorsHandler handles the FindFlowInitiatorsQuery.
type FindFlowInitiatorsHandler struct {
	db orm.DB
}

// NewFindFlowInitiatorsHandler creates a new FindFlowInitiatorsHandler.
func NewFindFlowInitiatorsHandler(db orm.DB) *FindFlowInitiatorsHandler {
	return &FindFlowInitiatorsHandler{db: db}
}

func (h *FindFlowInitiatorsHandler) Handle(ctx context.Context, query FindFlowInitiatorsQuery) ([]approval.FlowInitiator, error) {
	db := contextx.DB(ctx, h.db)

	// Start from an empty slice, never nil: an absent rule set is a meaningful
	// answer here — it says the flow is open to everyone, which save-time
	// validation guarantees is the only way to store none — and it must
	// serialize as [] rather than null so callers can read it as one.
	initiators := make([]approval.FlowInitiator, 0)
	if err := db.NewSelect().
		Model(&initiators).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID)
		}).
		// Deterministic order so the same flow always returns its initiators in a
		// stable sequence across calls and dialects (mirrors the determinism
		// find_flow_versions provides via OrderByDesc).
		OrderBy("id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow initiators: %w", err)
	}

	return initiators, nil
}
