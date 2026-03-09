package query

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindAvailableFlowsQuery queries flows the current user is allowed to initiate.
type FindAvailableFlowsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID          string
	TenantID        string
	ApplicantDeptID *string
	Keyword         string
}

// FindAvailableFlowsHandler handles the FindAvailableFlowsQuery.
type FindAvailableFlowsHandler struct {
	db              orm.DB
	assigneeService approval.AssigneeService
}

// NewFindAvailableFlowsHandler creates a new FindAvailableFlowsHandler.
func NewFindAvailableFlowsHandler(db orm.DB, assigneeService approval.AssigneeService) *FindAvailableFlowsHandler {
	return &FindAvailableFlowsHandler{db: db, assigneeService: assigneeService}
}

func (h *FindAvailableFlowsHandler) Handle(ctx context.Context, query FindAvailableFlowsQuery) (*page.Page[my.AvailableFlow], error) {
	db := contextx.DB(ctx, h.db)

	// First, find flows that allow all initiation.
	var allAllowedIDs []string
	if err := db.NewSelect().
		Model((*approval.Flow)(nil)).
		Select("id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("is_active", true).
				Equals("is_all_initiation_allowed", true).
				ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", query.TenantID)
				}).
				ApplyIf(query.Keyword != "", func(cb orm.ConditionBuilder) {
					cb.Contains("name", query.Keyword)
				})
		}).
		Scan(ctx, &allAllowedIDs); err != nil {
		return nil, fmt.Errorf("query all-allowed flows: %w", err)
	}

	// Find flows with user/dept/role initiator rules.
	// Load all initiators and filter in Go since JSON/array containment varies across dialects.
	// Scope initiator query to tenant's flows when TenantID is specified.
	var tenantFlowIDs []string
	if query.TenantID != "" {
		if err := db.NewSelect().
			Model((*approval.Flow)(nil)).
			Select("id").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", query.TenantID).
					Equals("is_active", true)
			}).
			Scan(ctx, &tenantFlowIDs); err != nil {
			return nil, fmt.Errorf("query tenant flows: %w", err)
		}

		if len(tenantFlowIDs) == 0 {
			r := page.New(query.Pageable, 0, []my.AvailableFlow{})

			return &r, nil
		}
	}

	var initiators []approval.FlowInitiator
	if err := db.NewSelect().
		Model(&initiators).
		Select("flow_id", "kind", "ids").
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(len(tenantFlowIDs) > 0, func(cb orm.ConditionBuilder) {
				cb.In("flow_id", tenantFlowIDs)
			})
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow initiators: %w", err)
	}

	initiatorFlowIDs, err := h.matchInitiatorFlowIDs(ctx, initiators, query.UserID, query.ApplicantDeptID)
	if err != nil {
		return nil, err
	}

	// Merge IDs using HashSet.
	flowIDSet := collections.NewHashSetFrom(allAllowedIDs...)
	for _, id := range initiatorFlowIDs {
		flowIDSet.Add(id)
	}

	if flowIDSet.IsEmpty() {
		result := page.New(query.Pageable, 0, []my.AvailableFlow{})

		return &result, nil
	}

	flowIDs := flowIDSet.ToSlice()

	publishedFlowIDs, err := h.loadPublishedFlowIDs(ctx, db, flowIDs)
	if err != nil {
		return nil, err
	}

	if len(publishedFlowIDs) == 0 {
		result := page.New(query.Pageable, 0, []my.AvailableFlow{})

		return &result, nil
	}

	// Load active flows that have at least one published version.
	var flows []approval.Flow

	sq := db.NewSelect().Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", publishedFlowIDs).
				Equals("is_active", true).
				ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", query.TenantID)
				}).
				ApplyIf(query.Keyword != "", func(cb orm.ConditionBuilder) {
					cb.Contains("name", query.Keyword)
				})
		}).
		OrderBy("name")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

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

func (h *FindAvailableFlowsHandler) matchInitiatorFlowIDs(
	ctx context.Context,
	initiators []approval.FlowInitiator,
	userID string,
	applicantDeptID *string,
) ([]string, error) {
	matchedFlowIDs := collections.NewHashSet[string]()
	roleMembershipCache := make(map[string]bool)

	for _, initiator := range initiators {
		switch initiator.Kind {
		case approval.InitiatorUser:
			if slices.Contains(initiator.IDs, userID) {
				matchedFlowIDs.Add(initiator.FlowID)
			}

		case approval.InitiatorDept:
			if applicantDeptID != nil && slices.Contains(initiator.IDs, *applicantDeptID) {
				matchedFlowIDs.Add(initiator.FlowID)
			}

		case approval.InitiatorRole:
			matched, err := h.matchesAnyRole(ctx, initiator.IDs, userID, roleMembershipCache)
			if err != nil {
				return nil, err
			}

			if matched {
				matchedFlowIDs.Add(initiator.FlowID)
			}
		}
	}

	return matchedFlowIDs.ToSlice(), nil
}

func (h *FindAvailableFlowsHandler) matchesAnyRole(
	ctx context.Context,
	roleIDs []string,
	userID string,
	cache map[string]bool,
) (bool, error) {
	if h.assigneeService == nil {
		return false, nil
	}

	for _, roleID := range roleIDs {
		matched, ok := cache[roleID]
		if !ok {
			users, err := h.assigneeService.GetRoleUsers(ctx, roleID)
			if err != nil {
				return false, fmt.Errorf("get users by role %s: %w", roleID, err)
			}

			matched = slices.ContainsFunc(users, func(u approval.UserInfo) bool { return u.ID == userID })
			cache[roleID] = matched
		}

		if matched {
			return true, nil
		}
	}

	return false, nil
}

func (*FindAvailableFlowsHandler) loadPublishedFlowIDs(ctx context.Context, db orm.DB, flowIDs []string) ([]string, error) {
	var publishedFlowIDs []string

	if err := db.NewSelect().
		Model((*approval.FlowVersion)(nil)).
		Distinct().
		Select("flow_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.In("flow_id", flowIDs).
				Equals("status", approval.VersionPublished)
		}).
		Scan(ctx, &publishedFlowIDs); err != nil {
		return nil, fmt.Errorf("query published flow versions: %w", err)
	}

	return publishedFlowIDs, nil
}

// loadCategoryMap loads flow categories by IDs and returns a map keyed by category ID.
func loadCategoryMap(ctx context.Context, db orm.DB, categoryIDs []string) (map[string]*approval.FlowCategory, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}

	var categories []approval.FlowCategory
	if err := db.NewSelect().Model(&categories).
		Where(func(cb orm.ConditionBuilder) { cb.In("id", categoryIDs) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow categories: %w", err)
	}

	m := make(map[string]*approval.FlowCategory, len(categories))
	for i := range categories {
		m[categories[i].ID] = &categories[i]
	}

	return m, nil
}
