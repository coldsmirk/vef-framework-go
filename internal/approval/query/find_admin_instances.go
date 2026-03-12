package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindAdminInstancesQuery queries instances for admin management.
type FindAdminInstancesQuery struct {
	cqrs.BaseQuery
	page.Pageable

	TenantID    *string
	ApplicantID *string
	Status      *approval.InstanceStatus
	FlowID      *string
	Keyword     *string
}

// FindAdminInstancesHandler handles the FindAdminInstancesQuery.
type FindAdminInstancesHandler struct {
	db orm.DB
}

// NewFindAdminInstancesHandler creates a new FindAdminInstancesHandler.
func NewFindAdminInstancesHandler(db orm.DB) *FindAdminInstancesHandler {
	return &FindAdminInstancesHandler{db: db}
}

func (h *FindAdminInstancesHandler) Handle(ctx context.Context, query FindAdminInstancesQuery) (*page.Page[admin.Instance], error) {
	db := contextx.DB(ctx, h.db)

	var instances []approval.Instance

	sq := db.NewSelect().Model(&instances).
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", *query.TenantID)
			}).
				ApplyIf(query.ApplicantID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("applicant_id", *query.ApplicantID)
				}).
				ApplyIf(query.Status != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("status", *query.Status)
				}).
				ApplyIf(query.FlowID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("flow_id", *query.FlowID)
				}).
				ApplyIf(query.Keyword != nil, func(cb orm.ConditionBuilder) {
					cb.Contains("title", *query.Keyword)
				})
		}).
		OrderByDesc("created_at")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query admin instances: %w", err)
	}

	if len(instances) == 0 {
		result := page.New(query.Pageable, count, []admin.Instance{})

		return &result, nil
	}

	// Collect flow IDs and current node IDs for batch lookup.
	flowIDs := make([]string, 0, len(instances))

	nodeIDs := make([]string, 0, len(instances))
	for _, inst := range instances {
		flowIDs = append(flowIDs, inst.FlowID)
		if inst.CurrentNodeID != nil {
			nodeIDs = append(nodeIDs, *inst.CurrentNodeID)
		}
	}

	flowMap, err := loadFlowMap(ctx, db, flowIDs)
	if err != nil {
		return nil, err
	}

	nodeMap, err := loadNodeNameMap(ctx, db, nodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]admin.Instance, len(instances))
	for i, inst := range instances {
		flow := flowMap[inst.FlowID]

		item := admin.Instance{
			InstanceID:    inst.ID,
			InstanceNo:    inst.InstanceNo,
			Title:         inst.Title,
			TenantID:      inst.TenantID,
			FlowID:        inst.FlowID,
			ApplicantID:   inst.ApplicantID,
			ApplicantName: inst.ApplicantName,
			Status:        string(inst.Status),
			CreatedAt:     inst.CreatedAt,
			FinishedAt:    inst.FinishedAt,
		}
		if flow != nil {
			item.FlowName = flow.Name
		}

		if inst.CurrentNodeID != nil {
			if name, ok := nodeMap[*inst.CurrentNodeID]; ok {
				item.CurrentNodeName = &name
			}
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
