package query

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// GetAdminInstanceDetailQuery retrieves the full admin detail of an instance.
// Tenant-scoped: handler authorizes Caller against the loaded instance's
// TenantID before returning data so a tenant admin cannot peek at another
// tenant's instance by guessing the ID.
type GetAdminInstanceDetailQuery struct {
	cqrs.BaseQuery

	InstanceID string
	Caller     approval.CallerContext
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

	bundle, err := loadInstanceDetailBundle(ctx, db, query.InstanceID)
	if err != nil {
		return nil, err
	}

	if !query.Caller.Allows(bundle.Instance.TenantID) {
		// Indistinguishable from "no such instance" on purpose — see
		// opaque response policy for query handlers (avoids cross-tenant
		// existence probing).
		return nil, shared.ErrInstanceNotFound
	}

	// Build DTO.
	instance := bundle.Instance
	flow := bundle.Flow
	tasks := bundle.Tasks
	actionLogs := bundle.ActionLogs
	nodeNameMap := bundle.NodeNameMap

	detail := &admin.InstanceDetail{
		Instance: admin.InstanceDetailInfo{
			InstanceID:              instance.ID,
			InstanceNo:              instance.InstanceNo,
			Title:                   instance.Title,
			TenantID:                instance.TenantID,
			FlowID:                  instance.FlowID,
			FlowName:                flow.Name,
			FlowVersionID:           instance.FlowVersionID,
			ApplicantID:             instance.ApplicantID,
			ApplicantName:           instance.ApplicantName,
			ApplicantDepartmentName: instance.ApplicantDepartmentName,
			Status:                  string(instance.Status),
			CurrentNodeID:           instance.CurrentNodeID,
			BusinessRecordID:        instance.BusinessRecordID,
			FormData:                instance.FormData,
			FormSchema:              bundle.FormSchema,
			CreatedAt:               instance.CreatedAt,
			FinishedAt:              instance.FinishedAt,
		},
		Tasks:      make([]admin.TaskDetailInfo, len(tasks)),
		ActionLogs: make([]admin.ActionLog, len(actionLogs)),
		FlowGraph:  buildInstanceFlowGraph(bundle),
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
		detail.ActionLogs[i] = toAdminActionLog(log)
	}

	return detail, nil
}
