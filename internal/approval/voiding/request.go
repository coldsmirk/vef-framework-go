package voiding

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

type RequestHandler struct{ core *Core }

func (*RequestHandler) Describe() approval.BusinessKindDescriptor {
	return approval.BusinessKindDescriptor{Kind: approval.BusinessKindVoidRequest, Label: i18n.T("approval_void_request_label"), FormFields: []approval.FormFieldDefinition{
		{Key: "targetInstanceId", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_instance_id")},
		{Key: "targetInstanceNo", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_instance_no")},
		{Key: "targetTitle", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_title")},
		{Key: "targetFlowCode", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_flow_code")},
		{Key: "targetApplicantId", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_applicant_id")},
		{Key: "targetApplicantDepartmentId", Kind: approval.FieldInput, Label: i18n.T("approval_void_target_applicant_department_id")},
		{Key: "reason", Kind: approval.FieldTextarea, Label: i18n.T("approval_void_reason")},
	}}
}

type requestInput struct {
	TargetID    string         `json:"targetInstanceId"`
	Reason      string         `json:"reason"`
	Attachments []string       `json:"attachments"`
	Options     map[string]any `json:"options"`
}

func (h *RequestHandler) Submit(ctx context.Context, db orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
	var in requestInput

	raw, err := json.Marshal(sc.FormData)
	if err != nil {
		return nil, errInvalidVoidInput
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	if err := decoder.Decode(&in); err != nil || in.TargetID == "" {
		return nil, errInvalidVoidInput
	}

	vc, err := h.core.target(
		ctx,
		db,
		approval.VoidInstanceInput{InstanceID: in.TargetID, Operator: sc.Applicant, Reason: in.Reason, Options: in.Options, Caller: approval.CallerContext{TenantID: sc.TenantID}},
		true,
	)
	if err != nil {
		return nil, err
	}

	if err := h.core.eligible(vc, approval.VoidApproval, sc.Flow.Code); err != nil {
		return nil, err
	}

	if err := validateReason(in.Reason, in.Attachments); err != nil {
		return nil, err
	}

	current, err := pending(ctx, db, vc.Instance.ID, true)
	if err != nil {
		return nil, err
	}

	if current != nil && (!sc.IsResubmission() || current.RequestInstanceID == nil || *current.RequestInstanceID != sc.InstanceID) {
		return nil, approval.ErrVoidRequestPending
	}

	record := &approval.VoidRecord{TenantID: sc.TenantID, InstanceID: in.TargetID, RequestInstanceID: &sc.InstanceID, Reversals: []string{}, FollowUps: []string{}}
	if sc.IsResubmission() {
		record, err = requestRecord(ctx, db, sc.Instance, true)
		if err != nil {
			return nil, err
		}

		if record.InstanceID != vc.Instance.ID || record.Status == approval.VoidVoided {
			return nil, approval.ErrVoidNotAllowed
		}
	} else if sc.BusinessRef != nil {
		return nil, approval.ErrInvalidBusinessRef
	}

	if _, err := h.core.executablePlan(ctx, db, vc); err != nil {
		return nil, err
	}

	record.Operator = sc.Applicant
	record.Status = approval.VoidRequested
	record.Reason = in.Reason
	record.Attachments = in.Attachments

	record.Options = in.Options
	if sc.IsResubmission() {
		_, err = db.NewUpdate().Model(record).Select("status", "reason", "attachments", "options").WherePK().Exec(ctx)
	} else {
		_, err = db.NewInsert().Model(record).Exec(ctx)
	}

	if err != nil {
		return nil, err
	}

	snapshot := map[string]any{
		"targetInstanceId":  vc.Instance.ID,
		"targetInstanceNo":  vc.Instance.InstanceNo,
		"targetTitle":       vc.Instance.Title,
		"targetFlowCode":    vc.Flow.Code,
		"targetApplicantId": vc.Instance.ApplicantID,
		"reason":            record.Reason,
	}
	if vc.Instance.ApplicantDepartmentID != nil {
		snapshot["targetApplicantDepartmentId"] = *vc.Instance.ApplicantDepartmentID
	}

	return &approval.Submission{BusinessRef: record.ID, FormData: snapshot}, nil
}

func requestRecord(ctx context.Context, db orm.DB, request *approval.Instance, lock bool) (*approval.VoidRecord, error) {
	if request.BusinessRef == nil {
		return nil, approval.ErrVoidNotAllowed
	}

	var rec approval.VoidRecord

	q := db.NewSelect().Model(&rec).Where(func(cb orm.ConditionBuilder) {
		cb.PKEquals(*request.BusinessRef).Equals("request_instance_id", request.ID).Equals("tenant_id", request.TenantID)
	})
	if lock {
		q = q.ForUpdate()
	}

	if err := q.Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, approval.ErrVoidNotAllowed
		}

		return nil, err
	}

	return &rec, nil
}

func (*RequestHandler) OnTaskAction(context.Context, orm.DB, *approval.TaskActionContext) (*approval.TaskActionOutcome, error) {
	return nil, nil
}

func (*RequestHandler) OnInstanceCreated(context.Context, orm.DB, *approval.Instance) error {
	return nil
}

func (h *RequestHandler) OnInstanceTransition(ctx context.Context, db orm.DB, request *approval.Instance, from, to approval.InstanceStatus) error {
	switch to {
	case approval.InstanceApproved:
		if from != approval.InstanceRunning {
			return nil
		}

		record, err := requestRecord(ctx, db, request, false)
		if err != nil {
			return err
		}

		vc, err := h.core.target(ctx, db, approval.VoidInstanceInput{InstanceID: record.InstanceID, Caller: approval.CallerContext{TenantID: request.TenantID}}, true)
		if err != nil {
			return err
		}

		if err := h.core.eligible(vc, approval.VoidApproval, request.FlowCode); err != nil {
			return err
		}

		current, err := pending(ctx, db, vc.Instance.ID, true)
		if err != nil {
			return err
		}

		if current == nil || current.ID != record.ID || current.RequestInstanceID == nil || *current.RequestInstanceID != request.ID {
			return approval.ErrVoidNotAllowed
		}

		vc.Operator = current.Operator
		vc.Reason = current.Reason
		vc.Options = current.Options
		vc.Request = request

		return h.core.execute(ctx, db, vc, current)

	case approval.InstanceRejected, approval.InstanceWithdrawn, approval.InstanceTerminated:
		_, err := db.NewUpdate().Model((*approval.VoidRecord)(nil)).Set("status", approval.VoidDismissed).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("tenant_id", request.TenantID).Equals("request_instance_id", request.ID).Equals("status", approval.VoidRequested)
		}).Exec(ctx)

		return err

	default:
		return nil
	}
}
