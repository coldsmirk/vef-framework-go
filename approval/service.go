package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// Service is the programmatic control surface of the approval engine: every
// runtime operation the approval/instance and approval/admin API resources
// expose, callable from host code without an HTTP principal. The API
// resources are themselves callers of this interface, so an operation behaves
// identically whether a request or a host routine triggered it — the same
// validation, the same permission checks, the same domain events, audit rows,
// lifecycle hooks and business write-back. Inject it via DI; it is available
// whenever vef.ApprovalModule is enabled.
//
// Every method takes the orm.DB handle the operation runs on. The handle
// decides the transaction boundary: pass the handle of an open RunInTx scope
// and the operation joins that transaction, so a business write and the
// approval action it triggers commit or roll back together; pass a plain
// handle and the operation opens (and commits) a transaction of its own. The
// handle is explicit rather than read off the context because orm.DB.RunInTx
// does not attach the transaction to the context it hands the callback — a
// method that only took a context would silently open a second, independent
// transaction inside the caller's. The handle must belong to the primary data
// source (approval tables live there, and the domain events publish with
// event.WithTx against it). Audit columns (created_by / updated_by) render
// the operator bound on the handle: the injected primary orm.DB carries
// orm.OperatorSystem, and a caller acting on behalf of a person binds that
// person first with db.WithNamedArg(orm.PlaceholderKeyOperator, userID).
//
// Errors are the public sentinels in api_errors.go (ErrFlowNotActive,
// ErrNotAllowedInitiate, ErrTaskNotPending, …), matchable with errors.Is.
//
// Flow-definition management (create / deploy / publish / update) is
// deliberately not part of this contract: it is design-time administration
// with a different trust posture, served by the approval/flow resource.
type Service interface {
	// StartInstance starts a new instance of the flow named by
	// StartInstanceInput.FlowCode and returns it after the engine has
	// traversed the start node — a flow whose start node reaches an end node
	// without stopping comes back already completed. Initiation permission is
	// enforced against the applicant exactly as for a request.
	StartInstance(ctx context.Context, db orm.DB, in StartInstanceInput) (*Instance, error)

	// WithdrawInstance withdraws a running or returned instance on behalf of
	// its applicant; the operator must be the applicant.
	WithdrawInstance(ctx context.Context, db orm.DB, in WithdrawInstanceInput) error

	// ResubmitInstance resubmits a returned instance with new form data on
	// behalf of its applicant; the operator must be the applicant.
	ResubmitInstance(ctx context.Context, db orm.DB, in ResubmitInstanceInput) error

	// TerminateInstance force-closes an instance (an admin operation).
	// Running, returned and withdrawn instances can all be terminated — the
	// instance state machine is the single authority on which statuses may
	// close.
	TerminateInstance(ctx context.Context, db orm.DB, in TerminateInstanceInput) error

	// ApproveTask approves a pending approval task, or finishes a pending
	// handle task — the two share one command, the difference being the
	// node's execution semantics rather than the API. The operator must hold
	// the task.
	ApproveTask(ctx context.Context, db orm.DB, in ApproveTaskInput) error

	// RejectTask rejects a pending task; the operator must hold the task.
	RejectTask(ctx context.Context, db orm.DB, in RejectTaskInput) error

	// TransferTask hands a pending task over to another user (转办); the
	// operator must hold the task and the node must allow transfer.
	TransferTask(ctx context.Context, db orm.DB, in TransferTaskInput) error

	// RollbackTask returns the instance to an earlier node (退回); the
	// operator must hold the task and the node must allow rollback.
	RollbackTask(ctx context.Context, db orm.DB, in RollbackTaskInput) error

	// ReassignTask moves a pending task to a different assignee (an admin
	// operation — the operator need not hold the task).
	ReassignTask(ctx context.Context, db orm.DB, in ReassignTaskInput) error

	// AddAssignee adds assignees before, after, or alongside a pending task
	// (加签); the operator must hold the task and the node must allow it.
	AddAssignee(ctx context.Context, db orm.DB, in AddAssigneeInput) error

	// RemoveAssignee cancels a pending task so its assignee no longer takes
	// part (减签); the node's last remaining assignee cannot be removed.
	RemoveAssignee(ctx context.Context, db orm.DB, in RemoveAssigneeInput) error

	// AddCC copies additional users on an instance (手动抄送); the current
	// node must allow manual CC.
	AddCC(ctx context.Context, db orm.DB, in AddCCInput) error

	// MarkCCRead records that a CC recipient has read the instance.
	MarkCCRead(ctx context.Context, db orm.DB, in MarkCCReadInput) error

	// UrgeTask sends an urge (催办) for a pending task, subject to the node's
	// per-(task, urger) cooldown. The urger must be a decision participant of
	// the instance — applicant or anyone a task was opened on — not a mere CC
	// observer.
	UrgeTask(ctx context.Context, db orm.DB, in UrgeTaskInput) error

	// RetryBusinessProjection re-applies one eventual business projection
	// immediately instead of waiting for the worker's next backoff window
	// (see vef.approval.business_binding.consistency). This is the entry
	// point for a repair routine draining projections wedged behind a schema
	// or permission fault. Administrative: only the tenant scope is enforced,
	// so the caller owns the permission check (see the interface comment).
	RetryBusinessProjection(ctx context.Context, db orm.DB, in RetryBusinessProjectionInput) error
}

// StartInstanceInput describes a new instance to start.
//
// Caller decides tenant authority and is required: a zero value fails closed
// (as ErrFlowNotFound, so callers cannot probe other tenants' flows). Use
// SystemCaller for trusted automation, or CallerContext{TenantID: …} to pin
// the operation to one tenant. Globals are the host-defined condition
// variables the request path resolves server-side through
// InstanceGlobalsResolver; here the host passes them directly, so a caller
// must supply values it trusts rather than forward anything a client sent —
// globals steer condition branches.
type StartInstanceInput struct {
	// TenantID selects the tenant whose flow to start; empty means
	// DefaultTenantID.
	TenantID string
	// FlowCode identifies the flow within the tenant.
	FlowCode string
	// Applicant is the person the instance is started for. Initiation
	// permission, the same-applicant node policy and the applicant subjects
	// of condition routing all read it, so fill the department fields when
	// the flow routes or resolves assignees by department.
	Applicant UserInfo
	// BusinessRef is the optional opaque handle of the business record the
	// instance belongs to. A business-bound flow may also allocate it through
	// its BusinessRefProvider, in which case a non-empty value here wins.
	BusinessRef *string
	// FormData is the submitted form, validated against the published
	// version's form fields.
	FormData map[string]any
	// Globals is the host-supplied global-variable snapshot persisted onto the
	// instance (Instance.Globals) and resolved by condition evaluation —
	// field subjects and expression bindings alike (see the type comment).
	// Snapshotting at start keeps routing deterministic across re-evaluation.
	Globals map[string]any
	// Caller carries the tenant authority of this call (see the type comment).
	Caller CallerContext
}

// WithdrawInstanceInput describes an instance withdrawal.
type WithdrawInstanceInput struct {
	InstanceID string
	// Operator is the acting person, recorded on the action log; it must be
	// the instance applicant.
	Operator UserInfo
	Reason   string
	Caller   CallerContext
}

// ResubmitInstanceInput describes the resubmission of a returned instance.
type ResubmitInstanceInput struct {
	InstanceID string
	// Operator is the acting person, recorded on the action log; it must be
	// the instance applicant.
	Operator UserInfo
	// FormData replaces the instance's form data and is validated against
	// the published version's form fields.
	FormData map[string]any
	Caller   CallerContext
}

// TerminateInstanceInput describes a forced instance closure.
type TerminateInstanceInput struct {
	InstanceID string
	// Operator is the acting person, recorded on the action log.
	Operator UserInfo
	Reason   string
	Caller   CallerContext
}

// ApproveTaskInput describes an approve (or handle) decision on a task.
type ApproveTaskInput struct {
	TaskID string
	// Operator is the acting person; it must hold the task (directly or as
	// the delegate of its assignee).
	Operator UserInfo
	Opinion  string
	// FormData carries the fields the node lets the approver edit; keys the
	// node does not mark editable are dropped, and `required` ones must be
	// filled.
	FormData    map[string]any
	Attachments []string
	Caller      CallerContext
}

// RejectTaskInput describes a reject decision on a task.
type RejectTaskInput struct {
	TaskID string
	// Operator is the acting person; it must hold the task.
	Operator    UserInfo
	Opinion     string
	FormData    map[string]any
	Attachments []string
	Caller      CallerContext
}

// TransferTaskInput describes handing a task over to another user.
type TransferTaskInput struct {
	TaskID string
	// Operator is the acting person; it must hold the task.
	Operator UserInfo
	Opinion  string
	FormData map[string]any
	// TransferToID is the user who receives the task.
	TransferToID string
	Attachments  []string
	Caller       CallerContext
}

// RollbackTaskInput describes returning an instance to an earlier node.
type RollbackTaskInput struct {
	TaskID string
	// Operator is the acting person; it must hold the task.
	Operator UserInfo
	Opinion  string
	FormData map[string]any
	// TargetNodeID names the node to return to; it must be one of the node's
	// configured rollback targets when the rollback type is `specified`.
	TargetNodeID string
	Attachments  []string
	Caller       CallerContext
}

// ReassignTaskInput describes moving a task to a different assignee.
type ReassignTaskInput struct {
	TaskID string
	// NewAssigneeID is the user who takes over the task.
	NewAssigneeID string
	// Operator is the acting administrator, recorded on the action log.
	Operator UserInfo
	Reason   string
	Caller   CallerContext
}

// AddAssigneeInput describes adding assignees to a task.
type AddAssigneeInput struct {
	TaskID  string
	UserIDs []string
	// AddType places the new assignees before, after, or alongside the
	// task's assignee; `parallel` is rejected on sequential nodes.
	AddType AddAssigneeType
	// Operator is the acting person; it must hold the task.
	Operator UserInfo
	Caller   CallerContext
}

// RemoveAssigneeInput describes canceling one assignee's task.
type RemoveAssigneeInput struct {
	// TaskID is the task to cancel — the removed assignee's, not the
	// operator's.
	TaskID string
	// Operator is the acting person; it must be an assignee of the task's
	// node.
	Operator UserInfo
	Caller   CallerContext
}

// AddCCInput describes copying users on an instance.
type AddCCInput struct {
	InstanceID string
	CCUserIDs  []string
	// Operator is the acting person; it must be an assignee of the
	// instance's current node.
	Operator UserInfo
	Caller   CallerContext
}

// MarkCCReadInput describes a CC read receipt.
type MarkCCReadInput struct {
	InstanceID string
	// UserID is the CC recipient marking their records read.
	UserID string
	Caller CallerContext
}

// UrgeTaskInput describes an urge for a pending task.
type UrgeTaskInput struct {
	TaskID string
	// UrgerID is the person urging; see Service.UrgeTask for who may.
	UrgerID string
	Message string
	Caller  CallerContext
}

// RetryBusinessProjectionInput describes a manual business-projection retry.
type RetryBusinessProjectionInput struct {
	// ProjectionID identifies the apv_business_projection row to re-apply.
	ProjectionID string
	Caller       CallerContext
}
