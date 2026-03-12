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
)

// FindMyInitiatedQuery queries instances initiated by the current user.
type FindMyInitiatedQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID   string
	TenantID *string
	Status   *approval.InstanceStatus
	Keyword  *string
}

// FindMyInitiatedHandler handles the FindMyInitiatedQuery.
type FindMyInitiatedHandler struct {
	db orm.DB
}

// NewFindMyInitiatedHandler creates a new FindMyInitiatedHandler.
func NewFindMyInitiatedHandler(db orm.DB) *FindMyInitiatedHandler {
	return &FindMyInitiatedHandler{db: db}
}

func (h *FindMyInitiatedHandler) Handle(ctx context.Context, query FindMyInitiatedQuery) (*page.Page[my.InitiatedInstance], error) {
	db := contextx.DB(ctx, h.db)

	var instances []approval.Instance

	sq := db.NewSelect().Model(&instances).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("applicant_id", query.UserID).
				ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", *query.TenantID)
				}).
				ApplyIf(query.Status != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("status", *query.Status)
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
		return nil, fmt.Errorf("query initiated instances: %w", err)
	}

	if len(instances) == 0 {
		result := page.New(query.Pageable, count, []my.InitiatedInstance{})

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

	items := make([]my.InitiatedInstance, len(instances))
	for i, inst := range instances {
		flow := flowMap[inst.FlowID]

		item := my.InitiatedInstance{
			InstanceID: inst.ID,
			InstanceNo: inst.InstanceNo,
			Title:      inst.Title,
			Status:     string(inst.Status),
			CreatedAt:  inst.CreatedAt,
			FinishedAt: inst.FinishedAt,
		}
		if flow != nil {
			item.FlowName = flow.Name
			item.FlowIcon = flow.Icon
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
