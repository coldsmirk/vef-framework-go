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

// FindAdminTasksQuery queries tasks for admin management.
type FindAdminTasksQuery struct {
	cqrs.BaseQuery
	page.Pageable

	TenantID   *string
	AssigneeID *string
	InstanceID *string
	Status     *approval.TaskStatus
}

// FindAdminTasksHandler handles the FindAdminTasksQuery.
type FindAdminTasksHandler struct {
	db orm.DB
}

// NewFindAdminTasksHandler creates a new FindAdminTasksHandler.
func NewFindAdminTasksHandler(db orm.DB) *FindAdminTasksHandler {
	return &FindAdminTasksHandler{db: db}
}

func (h *FindAdminTasksHandler) Handle(ctx context.Context, query FindAdminTasksQuery) (*page.Page[admin.Task], error) {
	db := contextx.DB(ctx, h.db)

	var tasks []approval.Task

	sq := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", *query.TenantID)
			}).
				ApplyIf(query.AssigneeID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("assignee_id", *query.AssigneeID)
				}).
				ApplyIf(query.InstanceID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("instance_id", *query.InstanceID)
				}).
				ApplyIf(query.Status != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("status", *query.Status)
				})
		}).
		OrderByDesc("created_at")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query admin tasks: %w", err)
	}

	if len(tasks) == 0 {
		result := page.New(query.Pageable, count, []admin.Task{})

		return &result, nil
	}

	// Collect instance IDs and node IDs for batch lookup.
	instanceIDs := make([]string, 0, len(tasks))

	nodeIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		instanceIDs = append(instanceIDs, t.InstanceID)
		nodeIDs = append(nodeIDs, t.NodeID)
	}

	instanceMap, flowMap, nodeMap, err := loadEnrichmentMaps(ctx, db, instanceIDs, nodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]admin.Task, len(tasks))
	for i, t := range tasks {
		item := admin.Task{
			TaskID:       t.ID,
			InstanceID:   t.InstanceID,
			NodeName:     nodeMap[t.NodeID],
			AssigneeID:   t.AssigneeID,
			AssigneeName: t.AssigneeName,
			Status:       string(t.Status),
			CreatedAt:    t.CreatedAt,
			Deadline:     t.Deadline,
			FinishedAt:   t.FinishedAt,
		}
		if inst := instanceMap[t.InstanceID]; inst != nil {
			item.InstanceTitle = inst.Title
			if flow := flowMap[inst.FlowID]; flow != nil {
				item.FlowName = flow.Name
			}
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
