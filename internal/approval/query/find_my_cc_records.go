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

// FindMyCCRecordsQuery queries CC records addressed to the current user.
type FindMyCCRecordsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID   string
	TenantID string
	IsRead   *bool
}

// FindMyCCRecordsHandler handles the FindMyCCRecordsQuery.
type FindMyCCRecordsHandler struct {
	db orm.DB
}

// NewFindMyCCRecordsHandler creates a new FindMyCCRecordsHandler.
func NewFindMyCCRecordsHandler(db orm.DB) *FindMyCCRecordsHandler {
	return &FindMyCCRecordsHandler{db: db}
}

func (h *FindMyCCRecordsHandler) Handle(ctx context.Context, query FindMyCCRecordsQuery) (*page.Page[my.CCRecord], error) {
	db := contextx.DB(ctx, h.db)

	var records []approval.CCRecord

	sq := db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("cc_user_id", query.UserID).
				ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
					cb.In("instance_id", db.NewSelect().
						Model((*approval.Instance)(nil)).
						Select("id").
						Where(func(cb orm.ConditionBuilder) {
							cb.Equals("tenant_id", query.TenantID)
						}),
					)
				})

			if query.IsRead != nil {
				if *query.IsRead {
					cb.IsNotNull("read_at")
				} else {
					cb.IsNull("read_at")
				}
			}
		}).
		OrderByDesc("created_at")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query cc records: %w", err)
	}

	if len(records) == 0 {
		result := page.New(query.Pageable, count, []my.CCRecord{})

		return &result, nil
	}

	instanceIDs := make([]string, 0, len(records))

	nodeIDs := make([]string, 0, len(records))
	for _, r := range records {
		instanceIDs = append(instanceIDs, r.InstanceID)
		if r.NodeID != nil {
			nodeIDs = append(nodeIDs, *r.NodeID)
		}
	}

	instanceMap, err := loadInstanceMap(ctx, db, instanceIDs)
	if err != nil {
		return nil, err
	}

	flowIDs := make([]string, 0, len(instanceMap))
	for _, inst := range instanceMap {
		flowIDs = append(flowIDs, inst.FlowID)
	}

	flowMap, err := loadFlowMap(ctx, db, flowIDs)
	if err != nil {
		return nil, err
	}

	nodeMap, err := loadNodeNameMap(ctx, db, nodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]my.CCRecord, len(records))
	for i, r := range records {
		item := my.CCRecord{
			CCRecordID: r.ID,
			InstanceID: r.InstanceID,
			IsRead:     r.ReadAt != nil,
			CreatedAt:  r.CreatedAt,
		}
		if r.NodeID != nil {
			if name, ok := nodeMap[*r.NodeID]; ok {
				item.NodeName = &name
			}
		}

		if inst := instanceMap[r.InstanceID]; inst != nil {
			item.InstanceTitle = inst.Title
			item.InstanceNo = inst.InstanceNo

			item.ApplicantName = inst.ApplicantName
			if flow := flowMap[inst.FlowID]; flow != nil {
				item.FlowName = flow.Name
				item.FlowIcon = flow.Icon
			}
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
