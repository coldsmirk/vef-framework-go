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

	detail := &admin.InstanceDetail{
		Instance: admin.InstanceDetailInfo{
			InstanceID:    instance.ID,
			InstanceNo:    instance.InstanceNo,
			Title:         instance.Title,
			TenantID:      instance.TenantID,
			FlowID:        instance.FlowID,
			FlowCode:      instance.FlowCode,
			FlowName:      flow.Name,
			FlowVersionID: instance.FlowVersionID,
			Labels:        flow.Labels,
			Applicant:     instance.Applicant(),
			Status:        string(instance.Status),
			CurrentNodeID: instance.CurrentNodeID,
			BusinessRef:   instance.BusinessRef,
			FormData:      instance.FormData,
			CreatedAt:     instance.CreatedAt,
			FinishedAt:    instance.FinishedAt,
		},
		FormSchema: bundle.FormSchema,
		Timeline:   buildInstanceTimeline(bundle),
		FlowGraph:  buildInstanceFlowGraph(bundle),
	}

	if instance.CurrentNodeID != nil {
		if name, ok := bundle.NodeNameMap[*instance.CurrentNodeID]; ok {
			detail.Instance.CurrentNodeName = &name
		}
	}

	return detail, nil
}
