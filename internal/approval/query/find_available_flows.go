package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/go-collections"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindAvailableFlowsQuery queries the flows the current user may initiate.
// They are the flows of the tenant the user would start in plus the shared
// flows that tenant has not replaced with one of its own — exactly the flows a
// start by code would find there (service.FindStartFlow).
type FindAvailableFlowsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID                string
	ApplicantDepartmentID *string
	Keyword               *string
	Labels                map[string]string
	// Caller decides the tenant the flows are offered in: the one it acts in
	// (service.StartTenant).
	Caller approval.CallerContext
}

// FindAvailableFlowsHandler handles the FindAvailableFlowsQuery.
type FindAvailableFlowsHandler struct {
	db            orm.DB
	validationSvc *service.ValidationService
}

// NewFindAvailableFlowsHandler creates a new FindAvailableFlowsHandler.
func NewFindAvailableFlowsHandler(db orm.DB, validationSvc *service.ValidationService) *FindAvailableFlowsHandler {
	return &FindAvailableFlowsHandler{db: db, validationSvc: validationSvc}
}

func (h *FindAvailableFlowsHandler) Handle(ctx context.Context, query FindAvailableFlowsQuery) (*page.Page[my.AvailableFlow], error) {
	db := contextx.DB(ctx, h.db)

	tenantID, err := service.StartTenant(query.Caller, "")
	if err != nil {
		return nil, err
	}

	candidates, err := loadStartableFlows(ctx, db, tenantID, query.Keyword, query.Labels)
	if err != nil {
		return nil, err
	}

	// The initiation gate a start applies (ValidationService), evaluated for
	// the tenant the start would run in, so the list offers exactly the flows
	// a start would accept — host initiator kinds included.
	flowIDs, err := h.validationSvc.InitiableFlowIDs(ctx, db, candidates, tenantID, approval.UserInfo{
		ID:           query.UserID,
		DepartmentID: query.ApplicantDepartmentID,
	})
	if err != nil {
		return nil, err
	}

	if len(flowIDs) == 0 {
		result := page.New(query.Pageable, 0, []my.AvailableFlow{})

		return &result, nil
	}

	var flows []approval.Flow

	sq := db.NewSelect().Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", flowIDs)
		}).
		OrderBy("name")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query available flows: %w", err)
	}

	if len(flows) == 0 {
		result := page.New(query.Pageable, count, []my.AvailableFlow{})

		return &result, nil
	}

	// Load categories.
	categoryIDs := make([]string, 0, len(flows))
	for _, f := range flows {
		categoryIDs = append(categoryIDs, f.CategoryID)
	}

	categoryMap, err := loadCategoryMap(ctx, db, categoryIDs)
	if err != nil {
		return nil, err
	}

	items := make([]my.AvailableFlow, len(flows))
	for i, f := range flows {
		item := my.AvailableFlow{
			FlowID:      f.ID,
			FlowCode:    f.Code,
			FlowName:    f.Name,
			FlowIcon:    f.Icon,
			Description: f.Description,
			Labels:      f.Labels,
			CategoryID:  f.CategoryID,
		}
		if cat := categoryMap[f.CategoryID]; cat != nil {
			item.CategoryName = cat.Name
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}

// loadStartableFlows returns the flows a start in tenantID could run, narrowed
// by keyword and labels: active flows with a published version, owned by the
// tenant or shared, minus every shared flow whose code the tenant has taken for
// a flow of its own — in any state, since that flow is what a start by the
// code finds.
func loadStartableFlows(
	ctx context.Context,
	db orm.DB,
	tenantID string,
	keyword *string,
	labels map[string]string,
) ([]approval.Flow, error) {
	var flows []approval.Flow

	if err := db.NewSelect().
		Model(&flows).
		Select("id", "tenant_id", "code", "is_all_initiation_allowed").
		Where(func(cb orm.ConditionBuilder) {
			cb.In("tenant_id", []string{tenantID, approval.SharedTenantID}).
				Equals("is_active", true).
				ApplyIf(keyword != nil, func(cb orm.ConditionBuilder) {
					cb.Contains("name", *keyword)
				})

			applyLabelsFilter(cb, labels)
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query candidate flows: %w", err)
	}

	sharedCodes := make([]string, 0, len(flows))
	for _, flow := range flows {
		if flow.TenantID == approval.SharedTenantID {
			sharedCodes = append(sharedCodes, flow.Code)
		}
	}

	var takenCodes []string

	if len(sharedCodes) > 0 {
		if err := db.NewSelect().
			Model((*approval.Flow)(nil)).
			Select("code").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", tenantID).
					In("code", sharedCodes)
			}).
			Scan(ctx, &takenCodes); err != nil {
			return nil, fmt.Errorf("query tenant flow codes: %w", err)
		}
	}

	taken := collections.NewHashSetFrom(takenCodes...)
	flows = lo.Reject(flows, func(flow approval.Flow, _ int) bool {
		return flow.TenantID == approval.SharedTenantID && taken.Contains(flow.Code)
	})

	if len(flows) == 0 {
		return nil, nil
	}

	publishedIDs, err := loadPublishedFlowIDs(ctx, db, lo.Map(flows, func(flow approval.Flow, _ int) string {
		return flow.ID
	}))
	if err != nil {
		return nil, err
	}

	published := collections.NewHashSetFrom(publishedIDs...)

	return lo.Filter(flows, func(flow approval.Flow, _ int) bool {
		return published.Contains(flow.ID)
	}), nil
}
