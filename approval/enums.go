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

// IsValid checks if the BindingMode is a valid value.
func (m BindingMode) IsValid() bool {
	return m == BindingStandalone || m == BindingBusiness
}

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
	InitiatorUser       InitiatorKind = "user"
	InitiatorRole       InitiatorKind = "role"
	InitiatorDepartment InitiatorKind = "department"
)

// IsValid checks if the InitiatorKind is a valid value.
func (k InitiatorKind) IsValid() bool {
	return k == InitiatorUser || k == InitiatorRole || k == InitiatorDepartment
}

// StorageMode represents the storage mode of form data at the FlowVersion level.
// It determines the physical storage location and format of form data, and is fixed when a version is published.
// This is different from BindingMode (Flow-level), which controls how the workflow integrates with business systems.
//
// Usage scenarios:
//   - JSON mode: Flexible schema, suitable for frequently changing form fields, limited query capabilities
//   - Table mode: Structured storage, suitable for complex queries and data analysis, requires predefined schema
type StorageMode string

const (
	// StorageJSON stores form data in the apv_instance.form_data JSONB column.
	StorageJSON StorageMode = "json"
	// StorageTable stores form data in a dedicated physical table generated per
	// published version (one table per version). The apv_instance.form_data
	// JSONB column is still populated so existing read paths keep working; the
	// physical table is the structured, queryable projection.
	StorageTable StorageMode = "table"
)

// IsValid reports whether the storage mode is one of the defined values.
func (m StorageMode) IsValid() bool {
	return m == StorageJSON || m == StorageTable
}

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

// ExecutionType represents how a task node is executed.
// It determines whether the node waits for human decisions or resolves itself
// the moment the instance enters it.
type ExecutionType string

const (
	ExecutionManual     ExecutionType = "manual"      // Manual: creates tasks and waits for assignees to act
	ExecutionAutoPass   ExecutionType = "auto_pass"   // AutoPass: the node passes immediately on entry, no tasks are created
	ExecutionAutoReject ExecutionType = "auto_reject" // AutoReject: the instance is rejected immediately on entry
)

// IsValid reports whether the execution type is one of the defined values.
func (t ExecutionType) IsValid() bool {
	return t == ExecutionManual || t == ExecutionAutoPass || t == ExecutionAutoReject
}

// ApprovalMethod represents the method of approval for a node with multiple assignees.
// It defines how the approval decision is made when there are multiple approvers.
type ApprovalMethod string

const (
	ApprovalSequential ApprovalMethod = "sequential" // Sequential: approvers process one by one in order, all must approve
	ApprovalParallel   ApprovalMethod = "parallel"   // Parallel: approvers process simultaneously, decision based on consensus rules
)

// IsValid reports whether the approval method is one of the defined values.
func (m ApprovalMethod) IsValid() bool {
	return m == ApprovalSequential || m == ApprovalParallel
}

// PassRule represents the strategy for passing the node (for Parallel/Or methods).
type PassRule string

const (
	PassAll   PassRule = "all"   // All assignees must approve; any rejection fails the node
	PassAny   PassRule = "any"   // At least one assignee must approve
	PassRatio PassRule = "ratio" // A certain percentage of assignees must approve
)

// IsValid reports whether the pass rule is one of the defined values.
func (r PassRule) IsValid() bool {
	return r == PassAll || r == PassAny || r == PassRatio
}

// EmptyAssigneeAction represents the action when no assignee is found.
type EmptyAssigneeAction string

const (
	EmptyAssigneeAutoPass          EmptyAssigneeAction = "auto_pass"
	EmptyAssigneeTransferAdmin     EmptyAssigneeAction = "transfer_admin"
	EmptyAssigneeTransferSuperior  EmptyAssigneeAction = "transfer_superior"
	EmptyAssigneeTransferApplicant EmptyAssigneeAction = "transfer_applicant"
	EmptyAssigneeTransferSpecified EmptyAssigneeAction = "transfer_specified"
)

// IsValid reports whether the empty-assignee action is one of the defined values.
func (a EmptyAssigneeAction) IsValid() bool {
	switch a {
	case EmptyAssigneeAutoPass, EmptyAssigneeTransferAdmin, EmptyAssigneeTransferSuperior,
		EmptyAssigneeTransferApplicant, EmptyAssigneeTransferSpecified:
		return true
	default:
		return false
	}
}

// SameApplicantAction represents the action when the assignee is the same as the applicant.
type SameApplicantAction string

const (
	SameApplicantAutoPass         SameApplicantAction = "auto_pass"
	SameApplicantSelfApprove      SameApplicantAction = "self_approve"      // Default
	SameApplicantTransferSuperior SameApplicantAction = "transfer_superior" // Transfer to superior
)

// IsValid reports whether the same-applicant action is one of the defined values.
func (a SameApplicantAction) IsValid() bool {
	return a == SameApplicantAutoPass || a == SameApplicantSelfApprove || a == SameApplicantTransferSuperior
}

// RollbackType represents the type of rollback allowed.
type RollbackType string

const (
	RollbackNone      RollbackType = "none"
	RollbackPrevious  RollbackType = "previous"  // To previous node
	RollbackStart     RollbackType = "start"     // To start node (applicant)
	RollbackAny       RollbackType = "any"       // To any node
	RollbackSpecified RollbackType = "specified" // To specified nodes
)

// IsValid reports whether the rollback type is one of the defined values.
func (t RollbackType) IsValid() bool {
	switch t {
	case RollbackNone, RollbackPrevious, RollbackStart, RollbackAny, RollbackSpecified:
		return true
	default:
		return false
	}
}

// RollbackDataStrategy represents the strategy for handling form data during rollback.
type RollbackDataStrategy string

const (
	RollbackDataClear RollbackDataStrategy = "clear" // Clear form data
	RollbackDataKeep  RollbackDataStrategy = "keep"  // Restore the form snapshot captured when the target node was first entered
)

// IsValid reports whether the rollback data strategy is one of the defined values.
func (s RollbackDataStrategy) IsValid() bool {
	return s == RollbackDataClear || s == RollbackDataKeep
}

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

// IsValid reports whether the consecutive-approver action is one of the defined values.
func (a ConsecutiveApproverAction) IsValid() bool {
	return a == ConsecutiveApproverNone || a == ConsecutiveApproverAutoPass
}

// AssigneeKind represents the kind of assignee.
type AssigneeKind string

const (
	AssigneeUser             AssigneeKind = "user"
	AssigneeRole             AssigneeKind = "role"
	AssigneeDepartment       AssigneeKind = "department"        // Department head
	AssigneeSelf             AssigneeKind = "self"              // Applicant themselves
	AssigneeSuperior         AssigneeKind = "superior"          // Direct superior
	AssigneeDepartmentLeader AssigneeKind = "department_leader" // Leaders of the applicant's own department (single level)
	AssigneeFormField        AssigneeKind = "form_field"        // Based on form field
)

// IsValid reports whether the assignee kind is one of the defined values.
func (k AssigneeKind) IsValid() bool {
	switch k {
	case AssigneeUser, AssigneeRole, AssigneeDepartment, AssigneeSelf,
		AssigneeSuperior, AssigneeDepartmentLeader, AssigneeFormField:
		return true
	default:
		return false
	}
}

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

// NodeVisitStatus represents the lifecycle status of a node visit — one
// traversal of a flow node by an instance.
type NodeVisitStatus string

const (
	NodeVisitActive   NodeVisitStatus = "active"   // The instance is currently sitting on the node
	NodeVisitPassed   NodeVisitStatus = "passed"   // The node concluded and the flow moved on
	NodeVisitRejected NodeVisitStatus = "rejected" // The node concluded by rejecting the instance
	NodeVisitReturned NodeVisitStatus = "returned" // The flow was sent back from this node (rollback)
	NodeVisitCanceled NodeVisitStatus = "canceled" // The visit was cut short (withdraw / terminate)
)

func (s NodeVisitStatus) String() string { return string(s) }

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
	ActionAddCC          ActionType = "add_cc"    // Participant added CC recipients
)

// CCKind represents the kind of CC recipient.
type CCKind string

const (
	CCUser       CCKind = "user"
	CCRole       CCKind = "role"
	CCDepartment CCKind = "department"
	CCFormField  CCKind = "form_field"
)

// IsValid reports whether the CC kind is one of the defined kinds.
func (k CCKind) IsValid() bool {
	return k == CCUser || k == CCRole || k == CCDepartment || k == CCFormField
}

// CCTiming represents the timing of CC notification.
type CCTiming string

const (
	CCTimingAlways    CCTiming = "always"     // Always: send CC regardless of result
	CCTimingOnApprove CCTiming = "on_approve" // OnApprove: send CC only when approved
	CCTimingOnReject  CCTiming = "on_reject"  // OnReject: send CC only when rejected
)

// IsValid reports whether the CC timing is one of the defined values.
func (t CCTiming) IsValid() bool {
	return t == CCTimingAlways || t == CCTimingOnApprove || t == CCTimingOnReject
}

// FieldKind represents the kind of a form field.
type FieldKind string

const (
	FieldInput    FieldKind = "input"
	FieldTextarea FieldKind = "textarea"
	FieldSelect   FieldKind = "select"
	FieldNumber   FieldKind = "number"
	FieldDate     FieldKind = "date"
	FieldUpload   FieldKind = "upload"
	// FieldTable is a single-level detail table: its value is a list of rows,
	// each row an object keyed by the field's Columns. Columns must not nest
	// another table — approval forms are applications, not data models; deep
	// structures belong to business tables reached via the business binding.
	FieldTable FieldKind = "table"
)

// IsValid reports whether the field kind is one of the defined values.
func (k FieldKind) IsValid() bool {
	switch k {
	case FieldInput, FieldTextarea, FieldSelect, FieldNumber, FieldDate, FieldUpload, FieldTable:
		return true
	default:
		return false
	}
}

// ColumnDataType is the dialect-independent logical column type a form field
// materializes into when the flow version's StorageMode is StorageTable. The
// form designer infers it from the widget (and lets the user override the few
// ambiguous ones); the storage layer maps it to a concrete SQL type per dialect.
//
// It is deliberately separate from FieldKind: Kind stays the coarse bucket the
// field-permission matrix keys off, while a field's physical storage type is an
// orthogonal concern. An empty ColumnType falls back to a Kind-derived type.
type ColumnDataType string

const (
	ColumnString   ColumnDataType = "string"   // short text → VARCHAR(maxLength), TEXT without one
	ColumnText     ColumnDataType = "text"     // long/free text → TEXT
	ColumnInteger  ColumnDataType = "integer"  // whole number → BIGINT
	ColumnDecimal  ColumnDataType = "decimal"  // fixed-point number → NUMERIC/DECIMAL(38, scale)
	ColumnBoolean  ColumnDataType = "boolean"  // true/false → BOOLEAN
	ColumnDate     ColumnDataType = "date"     // calendar date → DATE
	ColumnDatetime ColumnDataType = "datetime" // date + time → TIMESTAMP/DATETIME
	ColumnJSON     ColumnDataType = "json"     // array/composite → JSONB/JSON
)

// IsValid reports whether the column data type is one of the defined values.
func (c ColumnDataType) IsValid() bool {
	switch c {
	case ColumnString, ColumnText, ColumnInteger, ColumnDecimal,
		ColumnBoolean, ColumnDate, ColumnDatetime, ColumnJSON:
		return true
	default:
		return false
	}
}

// TimeoutAction represents the action to take when a task times out.
type TimeoutAction string

const (
	TimeoutActionNone          TimeoutAction = "none"           // Mark timeout only, no auto action
	TimeoutActionAutoPass      TimeoutAction = "auto_pass"      // Automatically approve the task
	TimeoutActionAutoReject    TimeoutAction = "auto_reject"    // Automatically reject the task
	TimeoutActionNotify        TimeoutAction = "notify"         // Send notification only
	TimeoutActionTransferAdmin TimeoutAction = "transfer_admin" // Transfer to node admin
)

// IsValid reports whether the timeout action is one of the defined values.
func (a TimeoutAction) IsValid() bool {
	switch a {
	case TimeoutActionNone, TimeoutActionAutoPass, TimeoutActionAutoReject,
		TimeoutActionNotify, TimeoutActionTransferAdmin:
		return true
	default:
		return false
	}
}

// Permission represents the permission level.
type Permission string

const (
	PermissionVisible  Permission = "visible"
	PermissionEditable Permission = "editable"
	PermissionHidden   Permission = "hidden"
	PermissionRequired Permission = "required"
)
