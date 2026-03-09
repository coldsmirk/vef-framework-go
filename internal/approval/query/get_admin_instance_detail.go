package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// GetAdminInstanceDetailQuery retrieves the full admin detail of an instance (no participant check).
type GetAdminInstanceDetailQuery struct {
	cqrs.BaseQuery

	InstanceID string
}

// GetAdminInstanceDetailHandler handles the GetAdminInstanceDetailQuery.
type GetAdminInstanceDetailHandler struct {
	db orm.DB
}

// NewGetAdminInstanceDetailHandler creates a new GetAdminInstanceDetailHandler.
func NewGetAdminInstanceDetailHandler(db orm.DB) *GetAdminInstanceDetailHandler {
	return &GetAdminInstanceDetailHandler{db: db}
}

func (h *GetAdminInstanceDetailHandler) Handle(ctx context.Context, query GetAdminInstanceDetailQuery) (*admin.InstanceDetail, error) {
	db := contextx.DB(ctx, h.db)

	// Load instance.
	var instance approval.Instance

	instance.ID = query.InstanceID

	if err := db.NewSelect().Model(&instance).WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("query instance: %w", err)
	}

	// Load flow.
	var flow approval.Flow

	flow.ID = instance.FlowID
	if err := db.NewSelect().Model(&flow).WherePK().Scan(ctx); err != nil && !result.IsRecordNotFound(err) {
		return nil, fmt.Errorf("query flow: %w", err)
	}

	// Load tasks.
	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	// Load action logs.
	var actionLogs []approval.ActionLog
	if err := db.NewSelect().Model(&actionLogs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	// Load flow nodes.
	var flowNodes []approval.FlowNode
	if err := db.NewSelect().Model(&flowNodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", instance.FlowVersionID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	nodeNameMap := make(map[string]string, len(flowNodes))
	for _, n := range flowNodes {
		nodeNameMap[n.ID] = n.Name
	}

	// Build DTO.
	detail := &admin.InstanceDetail{
		Instance: admin.InstanceDetailInfo{
			InstanceID:       instance.ID,
			InstanceNo:       instance.InstanceNo,
			Title:            instance.Title,
			TenantID:         instance.TenantID,
			FlowID:           instance.FlowID,
			FlowName:         flow.Name,
			FlowVersionID:    instance.FlowVersionID,
			ApplicantID:      instance.ApplicantID,
			ApplicantName:    instance.ApplicantName,
			Status:           string(instance.Status),
			BusinessRecordID: instance.BusinessRecordID,
			FormData:         instance.FormData,
			CreatedAt:        instance.CreatedAt,
			FinishedAt:       instance.FinishedAt,
		},
		Tasks:      make([]admin.TaskDetailInfo, len(tasks)),
		ActionLogs: make([]admin.ActionLog, len(actionLogs)),
		FlowNodes:  make([]admin.FlowNodeInfo, len(flowNodes)),
	}

	if instance.CurrentNodeID != nil {
		if name, ok := nodeNameMap[*instance.CurrentNodeID]; ok {
			detail.Instance.CurrentNodeName = &name
		}
	}

	for i, t := range tasks {
		detail.Tasks[i] = admin.TaskDetailInfo{
			TaskID:        t.ID,
			NodeID:        t.NodeID,
			NodeName:      nodeNameMap[t.NodeID],
			AssigneeID:    t.AssigneeID,
			AssigneeName:  t.AssigneeName,
			DelegatorID:   t.DelegatorID,
			DelegatorName: t.DelegatorName,
			Status:        string(t.Status),
			SortOrder:     t.SortOrder,
			Deadline:      t.Deadline,
			IsTimeout:     t.IsTimeout,
			CreatedAt:     t.CreatedAt,
			FinishedAt:    t.FinishedAt,
		}
	}

	for i, log := range actionLogs {
		detail.ActionLogs[i] = admin.ActionLog{
			LogID:            log.ID,
			Action:           string(log.Action),
			OperatorID:       log.OperatorID,
			OperatorName:     log.OperatorName,
			OperatorDeptName: log.OperatorDeptName,
			TransferToID:     log.TransferToID,
			TransferToName:   log.TransferToName,
			Opinion:          log.Opinion,
			CreatedAt:        log.CreatedAt,
		}
	}

	for i, n := range flowNodes {
		detail.FlowNodes[i] = admin.FlowNodeInfo{
			NodeID:        n.ID,
			Key:           n.Key,
			Kind:          string(n.Kind),
			Name:          n.Name,
			ExecutionType: string(n.ExecutionType),
		}
	}

	return detail, nil
}
