package voiding

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type targetView struct {
	InstanceID   *string                `json:"instanceId,omitempty"`
	InstanceNo   *string                `json:"instanceNo,omitempty"`
	Title        *string                `json:"title,omitempty"`
	FlowName     *string                `json:"flowName,omitempty"`
	Applicant    *approval.UserInfo     `json:"applicant,omitempty"`
	FinishedAt   *timex.DateTime        `json:"finishedAt,omitempty"`
	BusinessKind *approval.BusinessKind `json:"businessKind,omitempty"`
}

type requestRecordView struct {
	*approval.VoidRecord
	InstanceID *string `json:"instanceId,omitempty"`
	Reason     *string `json:"reason,omitempty"`
}

type requestView struct {
	Target targetView         `json:"target"`
	Plan   *approval.VoidPlan `json:"plan"`
	Record requestRecordView  `json:"record"`
}

func (h *RequestHandler) View(ctx context.Context, db orm.DB, view *approval.ViewContext) (any, error) {
	rec, err := requestRecord(ctx, db, view.Instance, false)
	if err != nil {
		return nil, err
	}
	vc, err := h.core.target(ctx, db, approval.VoidInstanceInput{InstanceID: rec.InstanceID, Operator: rec.Operator, Reason: rec.Reason, Options: rec.Options, Caller: approval.CallerContext{TenantID: view.Instance.TenantID}}, false)
	if err != nil {
		return nil, err
	}
	plan, err := h.core.plan(ctx, db, vc)
	if err != nil {
		return nil, err
	}
	return makeRequestView(vc, rec, plan, view.FieldPermissions), nil
}

func makeRequestView(vc *approval.VoidContext, rec *approval.VoidRecord, plan *approval.VoidPlan, perms map[string]approval.Permission) requestView {
	inst := vc.Instance
	v := requestView{Target: targetView{FinishedAt: inst.FinishedAt}, Plan: plan, Record: requestRecordView{VoidRecord: rec}}
	if perms["targetInstanceId"] != approval.PermissionHidden {
		v.Target.InstanceID = &inst.ID
		v.Record.InstanceID = &rec.InstanceID
	}
	if perms["targetInstanceNo"] != approval.PermissionHidden {
		v.Target.InstanceNo = &inst.InstanceNo
	}
	if perms["targetTitle"] != approval.PermissionHidden {
		v.Target.Title = &inst.Title
	}
	if perms["targetFlowCode"] != approval.PermissionHidden {
		v.Target.FlowName = &vc.Flow.Name
		v.Target.BusinessKind = inst.BusinessKind
	}
	if perms["targetApplicantId"] != approval.PermissionHidden {
		applicant := inst.Applicant()
		if perms["targetApplicantDepartmentId"] == approval.PermissionHidden {
			applicant.DepartmentID = nil
			applicant.DepartmentName = nil
		}
		v.Target.Applicant = &applicant
	}
	if perms["reason"] != approval.PermissionHidden {
		v.Record.Reason = &rec.Reason
	}
	return v
}
