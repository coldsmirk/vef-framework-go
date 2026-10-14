package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindFlowsQuery queries flows for admin management.
type FindFlowsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	// TenantID narrows the list to one tenant's flows —
	// approval.SharedTenantID for the shared ones — and lists every tenant's
	// when nil. Managing flow definitions is gated by permission, not by the
	// caller's own tenant.
	TenantID    *string
	CategoryID  *string
	Keyword     *string
	IsActive    *bool
	Labels      map[string]string
	BindingMode *approval.BindingMode
}

// FindFlowsHandler handles the FindFlowsQuery.
type FindFlowsHandler struct {
	db orm.DB
}

// NewFindFlowsHandler creates a new FindFlowsHandler.
func NewFindFlowsHandler(db orm.DB) *FindFlowsHandler {
	return &FindFlowsHandler{db: db}
}

func (h *FindFlowsHandler) Handle(ctx context.Context, query FindFlowsQuery) (*page.Page[approval.Flow], error) {
	db := contextx.DB(ctx, h.db)

	var flows []approval.Flow

	sq := db.NewSelect().Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", *query.TenantID)
			}).
				ApplyIf(query.CategoryID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("category_id", *query.CategoryID)
				}).
				ApplyIf(query.IsActive != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("is_active", *query.IsActive)
				}).
				ApplyIf(query.Keyword != nil, func(cb orm.ConditionBuilder) {
					cb.Contains("name", *query.Keyword)
				}).
				ApplyIf(query.BindingMode != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("binding_mode", *query.BindingMode)
				})

			applyLabelsFilter(cb, query.Labels)
		}).
		OrderBy("name")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query flows: %w", err)
	}

	result := page.New(query.Pageable, count, flows)

	return &result, nil
}
