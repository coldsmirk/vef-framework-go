package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// completedStatuses lists the task statuses considered "completed" for user-facing queries.
var completedStatuses = []string{
	string(approval.TaskApproved),
	string(approval.TaskRejected),
	string(approval.TaskHandled),
	string(approval.TaskTransferred),
	string(approval.TaskRolledBack),
}

// FindMyCompletedTasksQuery queries tasks already processed by the current user.
type FindMyCompletedTasksQuery struct {
	cqrs.BaseQuery
	page.Pageable
	MyTaskFilter

	UserID string
	// Status matches how the caller finished the task. It narrows within the
	// completed statuses, so a non-completed status matches nothing.
	Status *approval.TaskStatus
	// InstanceStatus matches the instance's current status.
	InstanceStatus *approval.InstanceStatus
	// FinishedAtFrom and FinishedAtTo bound when the task was finished, both inclusive.
	FinishedAtFrom *timex.DateTime
	FinishedAtTo   *timex.DateTime
}

// FindMyCompletedTasksHandler handles the FindMyCompletedTasksQuery.
type FindMyCompletedTasksHandler struct {
	db orm.DB
}

// NewFindMyCompletedTasksHandler creates a new FindMyCompletedTasksHandler.
func NewFindMyCompletedTasksHandler(db orm.DB) *FindMyCompletedTasksHandler {
	return &FindMyCompletedTasksHandler{db: db}
}

func (h *FindMyCompletedTasksHandler) Handle(ctx context.Context, query FindMyCompletedTasksQuery) (*page.Page[my.CompletedTask], error) {
	db := contextx.DB(ctx, h.db)

	var tasks []approval.Task

	sq := db.NewSelect().Model(&tasks).
		Apply(joinTaskInstance).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("assignee_id", query.UserID).
				In("status", completedStatuses).
				ApplyIf(query.Status != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(*query.Status))
				}).
				ApplyIf(query.InstanceStatus != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("i.status", string(*query.InstanceStatus))
				}).
				ApplyIf(query.FinishedAtFrom != nil, func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqual("finished_at", *query.FinishedAtFrom)
				}).
				ApplyIf(query.FinishedAtTo != nil, func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqual("finished_at", *query.FinishedAtTo)
				}).
				Apply(query.MyTaskFilter.apply)
		}).
		OrderByDesc("finished_at", "id")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}

	if len(tasks) == 0 {
		result := page.New(query.Pageable, count, []my.CompletedTask{})

		return &result, nil
	}

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

	items := make([]my.CompletedTask, len(tasks))
	for i, t := range tasks {
		item := my.CompletedTask{
			TaskID:     t.ID,
			NodeName:   nodeMap[t.NodeID],
			Status:     string(t.Status),
			FinishedAt: t.FinishedAt,
		}
		if inst := instanceMap[t.InstanceID]; inst != nil {
			item.InstanceID = inst.ID
			item.InstanceTitle = inst.Title
			item.InstanceNo = inst.InstanceNo
			item.InstanceStatus = inst.Status

			item.Applicant = inst.Applicant()
			if flow := flowMap[inst.FlowID]; flow != nil {
				item.FlowName = flow.Name
				item.FlowIcon = flow.Icon
				item.Labels = flow.Labels
			}
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
