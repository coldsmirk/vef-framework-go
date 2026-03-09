package approval

import (
	"encoding/json"
	"errors"
	"fmt"
)

// BindingMode represents the mode of binding with business data.
// It defines how the approval workflow stores and associates form data.
type BindingMode string

const (
	BindingStandalone BindingMode = "standalone" // Standalone: form data is stored in the approval workflow's own table
	BindingBusiness   BindingMode = "business"   // Business: links to existing business data table
)

// VersionStatus represents the status of a flow version.
type VersionStatus string

const (
	VersionDraft     VersionStatus = "draft"
	VersionPublished VersionStatus = "published"
	VersionArchived  VersionStatus = "archived"
)

// InitiatorKind represents the kind of initiator.
type InitiatorKind string

const (
	InitiatorUser InitiatorKind = "user"
	InitiatorRole InitiatorKind = "role"
	InitiatorDept InitiatorKind = "dept"
)

// StorageMode represents the storage mode of form data at the FlowVersion level.
// It determines the physical storage location and format of form data, and is fixed when a version is published.
// This is different from BindingMode (Flow-level), which controls how the workflow integrates with business systems.
//
// Usage scenarios:
//   - JSON mode: Flexible schema, suitable for frequently changing form fields, limited query capabilities
//   - Table mode: Structured storage, suitable for complex queries and data analysis, requires predefined schema
type StorageMode string

const (
	StorageJSON  StorageMode = "json"  // JSON: form data stored in apv_instance.form_data (JSONB column)
	StorageTable StorageMode = "table" // Table: form data stored in dynamically created tables (e.g., apv_form_data_{flow_code})
)

// NodeKind represents the kind of a flow node.
// It defines the different types of nodes that can exist in a workflow.
type NodeKind string

const (
	NodeStart     NodeKind = "start"     // Start node: the entry point of a workflow
	NodeApproval  NodeKind = "approval"  // Approval node: requires approval action from assignees
	NodeHandle    NodeKind = "handle"    // Handle node: requires processing/handling action from assignees
	NodeCondition NodeKind = "condition" // Condition node: branches the flow based on conditions
	NodeEnd       NodeKind = "end"       // End node: the terminal point of a workflow
	NodeCC        NodeKind = "cc"        // CC node: sends notifications to specified users
)

// ExecutionType represents how a node is executed.
// It determines whether the node requires manual intervention or can be processed automatically.
type ExecutionType string

const (
	ExecutionManual     ExecutionType = "manual"      // Manual: requires human intervention to process
	ExecutionAuto       ExecutionType = "auto"        // Auto: automatically executed by the system
	ExecutionAutoPass   ExecutionType = "auto_pass"   // AutoPass: automatically approved when no assignee is found
	ExecutionAutoReject ExecutionType = "auto_reject" // AutoReject: automatically rejected when no assignee is found
)

// ApprovalMethod represents the method of approval for a node with multiple assignees.
// It defines how the approval decision is made when there are multiple approvers.
type ApprovalMethod string

const (
	ApprovalSequential ApprovalMethod = "sequential" // Sequential: approvers process one by one in order, all must approve
	ApprovalParallel   ApprovalMethod = "parallel"   // Parallel: approvers process simultaneously, decision based on consensus rules
)

// PassRule represents the strategy for passing the node (for Parallel/Or methods).
type PassRule string

const (
	PassAll       PassRule = "all"        // All assignees must approve
	PassAny       PassRule = "any"        // At least one assignee must approve
	PassRatio     PassRule = "ratio"      // A certain percentage of assignees must approve
	PassAnyReject PassRule = "any_reject" // Any one rejection fails
)

// EmptyAssigneeAction represents the action when no assignee is found.
type EmptyAssigneeAction string

const (
	EmptyAssigneeAutoPass          EmptyAssigneeAction = "auto_pass"
	EmptyAssigneeTransferAdmin     EmptyAssigneeAction = "transfer_admin"
	EmptyAssigneeTransferSuperior  EmptyAssigneeAction = "transfer_superior"
	EmptyAssigneeTransferApplicant EmptyAssigneeAction = "transfer_applicant"
	EmptyAssigneeTransferSpecified EmptyAssigneeAction = "transfer_specified"
)

// SameApplicantAction represents the action when the assignee is the same as the applicant.
type SameApplicantAction string

const (
	SameApplicantAutoPass         SameApplicantAction = "auto_pass"
	SameApplicantSelfApprove      SameApplicantAction = "self_approve"      // Default
	SameApplicantTransferSuperior SameApplicantAction = "transfer_superior" // Transfer to superior
)

// RollbackType represents the type of rollback allowed.
type RollbackType string

const (
	RollbackNone      RollbackType = "none"
	RollbackPrevious  RollbackType = "previous"  // To previous node
	RollbackStart     RollbackType = "start"     // To start node (applicant)
	RollbackAny       RollbackType = "any"       // To any node
	RollbackSpecified RollbackType = "specified" // To specified nodes
)

// RollbackDataStrategy represents the strategy for handling form data during rollback.
type RollbackDataStrategy string

const (
	RollbackDataClear RollbackDataStrategy = "clear" // Clear form data
	RollbackDataKeep  RollbackDataStrategy = "keep"  // Keep history data
)

var errInvalidAddAssigneeType = errors.New("invalid AddAssigneeType")

// AddAssigneeType represents the type of dynamic assignee addition.
// It defines how a newly added assignee is positioned relative to the current task.
type AddAssigneeType string

const (
	AddAssigneeBefore   AddAssigneeType = "before"   // Before: new assignee processes first, original task becomes pending after completion
	AddAssigneeAfter    AddAssigneeType = "after"    // After: new assignee processes after the original assignee completes
	AddAssigneeParallel AddAssigneeType = "parallel" // Parallel: new assignee joins the current parallel group to process together
)

// IsValid checks if the AddAssigneeType is a valid value.
func (t AddAssigneeType) IsValid() bool {
	return t == AddAssigneeBefore || t == AddAssigneeAfter || t == AddAssigneeParallel
}

// UnmarshalJSON validates AddAssigneeType values when decoding JSON payloads.
func (t *AddAssigneeType) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	parsed := AddAssigneeType(value)
	if !parsed.IsValid() {
		return fmt.Errorf("invalid AddAssigneeType %q: %w", value, errInvalidAddAssigneeType)
	}

	*t = parsed

	return nil
}

// ConsecutiveApproverAction represents the action when the same approver appears
// in consecutive approval nodes and approved in the previous node.
type ConsecutiveApproverAction string

const (
	ConsecutiveApproverNone     ConsecutiveApproverAction = "none"
	ConsecutiveApproverAutoPass ConsecutiveApproverAction = "auto_pass"
)

// AssigneeKind represents the kind of assignee.
type AssigneeKind string

const (
	AssigneeUser       AssigneeKind = "user"
	AssigneeRole       AssigneeKind = "role"
	AssigneeDept       AssigneeKind = "dept"        // Department head
	AssigneeSelf       AssigneeKind = "self"        // Applicant themselves
	AssigneeSuperior   AssigneeKind = "superior"    // Direct superior
	AssigneeDeptLeader AssigneeKind = "dept_leader" // Continuous multi-level supervisor
	AssigneeFormField  AssigneeKind = "form_field"  // Based on form field
)

// InstanceStatus represents the status of a flow instance.
type InstanceStatus string

const (
	InstanceRunning    InstanceStatus = "running"
	InstanceApproved   InstanceStatus = "approved"
	InstanceRejected   InstanceStatus = "rejected"
	InstanceWithdrawn  InstanceStatus = "withdrawn"
	InstanceReturned   InstanceStatus = "returned"
	InstanceTerminated InstanceStatus = "terminated"
)

func (s InstanceStatus) String() string { return string(s) }
func (s InstanceStatus) IsFinal() bool {
	return s == InstanceApproved || s == InstanceRejected || s == InstanceTerminated
}

// TaskStatus represents the status of an approval task.
type TaskStatus string

const (
	TaskWaiting     TaskStatus = "waiting"
	TaskPending     TaskStatus = "pending"
	TaskApproved    TaskStatus = "approved"
	TaskRejected    TaskStatus = "rejected"
	TaskHandled     TaskStatus = "handled"
	TaskTransferred TaskStatus = "transferred"
	TaskRolledBack  TaskStatus = "rolled_back"
	TaskCanceled    TaskStatus = "canceled"
	TaskRemoved     TaskStatus = "removed"
	TaskSkipped     TaskStatus = "skipped"
)

func (s TaskStatus) String() string { return string(s) }
func (s TaskStatus) IsFinal() bool {
	return s == TaskApproved ||
		s == TaskRejected ||
		s == TaskHandled ||
		s == TaskTransferred ||
		s == TaskRolledBack ||
		s == TaskCanceled ||
		s == TaskRemoved ||
		s == TaskSkipped
}

// ConditionKind represents the kind of condition for condition branches.
type ConditionKind string

const (
	ConditionField      ConditionKind = "field"      // Field-based condition
	ConditionExpression ConditionKind = "expression" // Expression-based condition
)

// ActionType represents the type of action performed by an operator.
type ActionType string

const (
	ActionSubmit         ActionType = "submit"
	ActionApprove        ActionType = "approve"
	ActionHandle         ActionType = "handle"
	ActionReject         ActionType = "reject"
	ActionTransfer       ActionType = "transfer"
	ActionWithdraw       ActionType = "withdraw"
	ActionCancel         ActionType = "cancel"
	ActionRollback       ActionType = "rollback"
	ActionAddAssignee    ActionType = "add_assignee"
	ActionRemoveAssignee ActionType = "remove_assignee"
	ActionExecute        ActionType = "execute"   // System execution action
	ActionResubmit       ActionType = "resubmit"  // Resubmit a returned instance
	ActionReassign       ActionType = "reassign"  // Admin reassigned task to a different user
	ActionTerminate      ActionType = "terminate" // Admin force-terminated an instance
)

// CCKind represents the kind of CC recipient.
type CCKind string

const (
	CCUser      CCKind = "user"
	CCRole      CCKind = "role"
	CCDept      CCKind = "dept"
	CCFormField CCKind = "form_field"
)

// CCTiming represents the timing of CC notification.
type CCTiming string

const (
	CCTimingAlways    CCTiming = "always"     // Always: send CC regardless of result
	CCTimingOnApprove CCTiming = "on_approve" // OnApprove: send CC only when approved
	CCTimingOnReject  CCTiming = "on_reject"  // OnReject: send CC only when rejected
)

// EventOutboxStatus represents the processing status of an event outbox record.
type EventOutboxStatus string

const (
	EventOutboxPending    EventOutboxStatus = "pending"
	EventOutboxProcessing EventOutboxStatus = "processing"
	EventOutboxCompleted  EventOutboxStatus = "completed"
	EventOutboxFailed     EventOutboxStatus = "failed"
)

// FieldKind represents the kind of a form field.
type FieldKind string

const (
	FieldInput    FieldKind = "input"
	FieldTextarea FieldKind = "textarea"
	FieldSelect   FieldKind = "select"
	FieldNumber   FieldKind = "number"
	FieldDate     FieldKind = "date"
	FieldUpload   FieldKind = "upload"
)

// TimeoutAction represents the action to take when a task times out.
type TimeoutAction string

const (
	TimeoutActionNone          TimeoutAction = "none"           // Mark timeout only, no auto action
	TimeoutActionAutoPass      TimeoutAction = "auto_pass"      // Automatically approve the task
	TimeoutActionAutoReject    TimeoutAction = "auto_reject"    // Automatically reject the task
	TimeoutActionNotify        TimeoutAction = "notify"         // Send notification only
	TimeoutActionTransferAdmin TimeoutAction = "transfer_admin" // Transfer to node admin
)

// Permission represents the permission level.
type Permission string

const (
	PermissionVisible  Permission = "visible"
	PermissionEditable Permission = "editable"
	PermissionHidden   Permission = "hidden"
	PermissionRequired Permission = "required"
)
