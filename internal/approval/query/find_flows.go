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

	TenantID    *string
	CategoryID  *string
	Keyword     *string
	IsActive    *bool
	Labels      map[string]string
	BindingMode *approval.BindingMode
	Caller      approval.CallerContext
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

	// TenantScopeFilter is the single source of truth for the tenant filter:
	// super-admin keeps the caller-supplied override (possibly empty for
	// cross-tenant view); every other caller is pinned to their own tenant
	// regardless of what the client sent, and fails closed if it has none.
	// Mirrors the admin resource's resolveTenantFilter helper.
	override := ""
	if query.TenantID != nil {
		override = *query.TenantID
	}

	scope, err := query.Caller.TenantScopeFilter(override)
	if err != nil {
		return nil, err
	}

	query.TenantID = scope

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
