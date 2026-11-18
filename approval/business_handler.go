package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// BusinessKind names a kind of business record approvals are submitted for
// (a meeting booking, an official document). It is registered by exactly one
// BusinessHandler; flows select it through Flow.BusinessKind.
type BusinessKind string

type BusinessKindDescriptor struct {
	Kind  BusinessKind `json:"kind"`
	Label string       `json:"label"`
	// FormFields is the catalog of the approval-relevant fields the handler
	// snapshots onto the instance: what conditions, form-field assignees,
	// title templates and field permissions may reference.
	FormFields []FormFieldDefinition `json:"formFields"`
}

type BusinessHandler interface {
	// Describe returns the kind this handler serves and its field catalog.
	Describe() BusinessKindDescriptor
	// Submit validates the submitted form and writes the business record —
	// creating it on a start, updating it on a resubmission — inside the
	// approval transaction, and returns the record's reference and the
	// approval field snapshot. An error rolls the whole start or resubmission back.
	Submit(ctx context.Context, db orm.DB, sc *SubmissionContext) (*Submission, error)
	// OnTaskAction runs when a person decides a task of an instance of this
	// kind (approve, handle, reject, transfer, rollback), after the instance
	// and task are locked and the operator verified, before the engine moves.
	// It applies the operator's form edits to the business record and may veto
	// the action by returning an error. A nil outcome or nil FormData leaves
	// the instance's form data unchanged.
	OnTaskAction(ctx context.Context, db orm.DB, ac *TaskActionContext) (*TaskActionOutcome, error)
	// View returns the business payload an instance detail shows, already
	// narrowed to what the viewer may see (vc.FieldPermissions, vc.Participation).
	View(ctx context.Context, db orm.DB, vc *ViewContext) (any, error)
}

type SubmissionContext struct {
	// InstanceID is allocated before Submit runs, so a record can point at the
	// instance it is submitted with.
	InstanceID string
	// Instance is nil on a start and the returned instance on a resubmission.
	Instance  *Instance
	Flow      *Flow
	Version   *FlowVersion
	TenantID  string // the tenant the instance runs in
	Applicant UserInfo
	// BusinessRef is the reference the client named — an existing record it
	// submits — on a start, and the instance's reference on a resubmission.
	// The handler must verify the applicant may submit that record.
	BusinessRef *string
	FormData    map[string]any // the submitted form, unvalidated
}

func (sc *SubmissionContext) IsResubmission() bool { return sc.Instance != nil }

type Submission struct {
	BusinessRef string         // required, non-blank
	FormData    map[string]any // the approval field snapshot
}

type TaskActionContext struct {
	Instance *Instance
	Task     *Task
	Node     *FlowNode
	Action   ActionType // ActionApprove, ActionHandle, ActionReject, ActionTransfer, ActionRollback
	Operator UserInfo
	Opinion  string
	FormData map[string]any // the operator's submitted form, unvalidated
	// FieldPermissions are the acting node's; the handler enforces which
	// business fields the operator may edit by them.
	FieldPermissions map[string]Permission
	TransferTo       *UserInfo // ActionTransfer only
	TargetNodeID     string    // ActionRollback only
}

type TaskActionOutcome struct {
	FormData map[string]any // the new snapshot; nil leaves it unchanged
}

type ViewContext struct {
	Instance *Instance
	Viewer   UserInfo
	// Participation is the viewer's; nil on an administrative view, which
	// sees everything.
	Participation *Participation
	// FieldPermissions is the viewer's effective permission per catalog field
	// (the same map the detail returns); nil on an administrative view.
	FieldPermissions map[string]Permission
}
