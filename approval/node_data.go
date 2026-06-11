package approval

import (
	"cmp"

	"github.com/coldsmirk/vef-framework-go/decimal"
)

// NodeData is the interface implemented by all node data types.
type NodeData interface {
	// Kind returns the node kind (start, end, approval, handle, cc, condition).
	Kind() NodeKind
	// GetName returns the display name of the node.
	GetName() string
	// GetDescription returns the optional description of the node.
	GetDescription() *string
	// ApplyTo applies this node data's configuration to the given FlowNode,
	// resolving omitted optional fields to their documented defaults so the
	// persisted node always carries complete, valid configuration.
	ApplyTo(node *FlowNode)
}

// Designer-aligned defaults resolved by ApplyTo when a field is omitted from
// the node data payload. The flow editor displays exactly these values for
// untouched controls, so resolving them here keeps "what the designer shows"
// and "what the engine runs" identical by construction.
const (
	DefaultExecutionType             = ExecutionManual
	DefaultApprovalMethod            = ApprovalParallel
	DefaultPassRule                  = PassAll
	DefaultEmptyAssigneeAction       = EmptyAssigneeAutoPass
	DefaultSameApplicantAction       = SameApplicantSelfApprove
	DefaultConsecutiveApproverAction = ConsecutiveApproverNone
	DefaultRollbackType              = RollbackPrevious
	DefaultRollbackDataStrategy      = RollbackDataKeep
	DefaultTimeoutAction             = TimeoutActionNone
	DefaultCCTiming                  = CCTimingAlways

	// Handle nodes default to sequential execution with the PassAny rule,
	// since any handler completing the task is sufficient.
	DefaultHandleApprovalMethod = ApprovalSequential
	DefaultHandlePassRule       = PassAny

	// DefaultUrgeCooldownMinutes is the urge cooldown applied when a node
	// leaves UrgeCooldownMinutes at 0. The flow editor surfaces the same
	// value in its placeholder text; keep the two in lockstep.
	DefaultUrgeCooldownMinutes = 30
)

// orTrue resolves an omitted optional bool to true. Permission toggles
// (rollback / transfer / add-assignee / remove-assignee / manual-CC) default
// to allowed, which a plain bool cannot express — its zero value would
// silently flip an omitted field to "forbidden".
func orTrue(value *bool) bool {
	if value == nil {
		return true
	}

	return *value
}

// BaseNodeData contains common fields shared across all node data types.
type BaseNodeData struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// GetName returns the node name.
func (d BaseNodeData) GetName() string { return d.Name }

// GetDescription returns the node description.
func (d BaseNodeData) GetDescription() *string { return d.Description }

// applyBaseNodeData applies BaseNodeData fields to a FlowNode.
func applyBaseNodeData(node *FlowNode, data *BaseNodeData) {
	node.Name = data.Name
	node.Description = data.Description
}

// --- TaskNodeData ---

// TaskNodeData contains fields shared by approval and handle nodes.
//
// IsTransferAllowed is a pointer because its default is true: an omitted
// field must resolve to "allowed", which a plain bool zero value cannot
// represent.
type TaskNodeData struct {
	Assignees                []AssigneeDefinition  `json:"assignees,omitempty"`
	ExecutionType            ExecutionType         `json:"executionType,omitempty"`
	EmptyAssigneeAction      EmptyAssigneeAction   `json:"emptyAssigneeAction,omitempty"`
	FallbackUserIDs          []string              `json:"fallbackUserIds,omitempty"`
	AdminUserIDs             []string              `json:"adminUserIds,omitempty"`
	IsTransferAllowed        *bool                 `json:"isTransferAllowed,omitempty"`
	IsOpinionRequired        bool                  `json:"isOpinionRequired,omitempty"`
	TimeoutHours             int                   `json:"timeoutHours,omitempty"`
	TimeoutAction            TimeoutAction         `json:"timeoutAction,omitempty"`
	TimeoutNotifyBeforeHours int                   `json:"timeoutNotifyBeforeHours,omitempty"`
	UrgeCooldownMinutes      int                   `json:"urgeCooldownMinutes,omitempty"`
	CCs                      []CCDefinition        `json:"ccs,omitempty"`
	FieldPermissions         map[string]Permission `json:"fieldPermissions,omitempty"`
}

// GetAssignees returns the assignee definitions from TaskNodeData.
func (d *TaskNodeData) GetAssignees() []AssigneeDefinition {
	return d.Assignees
}

// GetCCs returns the CC definitions from TaskNodeData.
func (d *TaskNodeData) GetCCs() []CCDefinition {
	return d.CCs
}

// applyTaskNodeData applies TaskNodeData fields to a FlowNode, resolving
// omitted fields to their defaults. The target node is assumed to be freshly
// constructed (zero value) so all fields are overwritten unconditionally —
// this is intentional full-snapshot semantics, not a partial update.
func applyTaskNodeData(node *FlowNode, data *TaskNodeData) {
	node.ExecutionType = cmp.Or(data.ExecutionType, DefaultExecutionType)
	node.EmptyAssigneeAction = cmp.Or(data.EmptyAssigneeAction, DefaultEmptyAssigneeAction)
	node.FallbackUserIDs = data.FallbackUserIDs
	node.AdminUserIDs = data.AdminUserIDs
	node.IsTransferAllowed = orTrue(data.IsTransferAllowed)
	node.IsOpinionRequired = data.IsOpinionRequired
	node.TimeoutHours = data.TimeoutHours
	node.TimeoutAction = cmp.Or(data.TimeoutAction, DefaultTimeoutAction)
	node.TimeoutNotifyBeforeHours = data.TimeoutNotifyBeforeHours
	node.UrgeCooldownMinutes = data.UrgeCooldownMinutes
	node.FieldPermissions = data.FieldPermissions
}

// --- StartNodeData ---

// StartNodeData contains data specific to start nodes.
type StartNodeData struct {
	BaseNodeData
}

// Kind returns the node kind.
func (*StartNodeData) Kind() NodeKind { return NodeStart }

// ApplyTo applies start node data to a FlowNode.
func (d *StartNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
}

// --- EndNodeData ---

// EndNodeData contains data specific to end nodes.
type EndNodeData struct {
	BaseNodeData
}

// Kind returns the node kind.
func (*EndNodeData) Kind() NodeKind { return NodeEnd }

// ApplyTo applies end node data to a FlowNode.
func (d *EndNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
}

// --- ApprovalNodeData ---

// ApprovalNodeData contains data specific to approval nodes.
//
// The Is*Allowed permission toggles are pointers because their default is
// true; see TaskNodeData for the rationale.
type ApprovalNodeData struct {
	BaseNodeData
	TaskNodeData

	ApprovalMethod            ApprovalMethod            `json:"approvalMethod,omitempty"`
	PassRule                  PassRule                  `json:"passRule,omitempty"`
	PassRatio                 decimal.Decimal           `json:"passRatio"`
	SameApplicantAction       SameApplicantAction       `json:"sameApplicantAction,omitempty"`
	ConsecutiveApproverAction ConsecutiveApproverAction `json:"consecutiveApproverAction,omitempty"`
	RollbackType              RollbackType              `json:"rollbackType,omitempty"`
	RollbackDataStrategy      RollbackDataStrategy      `json:"rollbackDataStrategy,omitempty"`
	RollbackTargetKeys        []string                  `json:"rollbackTargetKeys,omitempty"`
	IsRollbackAllowed         *bool                     `json:"isRollbackAllowed,omitempty"`
	IsAddAssigneeAllowed      *bool                     `json:"isAddAssigneeAllowed,omitempty"`
	AddAssigneeTypes          []AddAssigneeType         `json:"addAssigneeTypes,omitempty"`
	IsRemoveAssigneeAllowed   *bool                     `json:"isRemoveAssigneeAllowed,omitempty"`
	IsManualCCAllowed         *bool                     `json:"isManualCcAllowed,omitempty"`
}

// Kind returns the node kind.
func (*ApprovalNodeData) Kind() NodeKind { return NodeApproval }

// ApplyTo applies approval node data to a FlowNode, resolving omitted fields
// to the designer defaults. The target node is assumed to be freshly
// constructed (zero value); all fields are overwritten unconditionally as a
// full-snapshot deploy operation.
func (d *ApprovalNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	applyTaskNodeData(node, &d.TaskNodeData)

	node.ApprovalMethod = cmp.Or(d.ApprovalMethod, DefaultApprovalMethod)
	node.PassRule = cmp.Or(d.PassRule, DefaultPassRule)
	node.PassRatio = d.PassRatio
	node.SameApplicantAction = cmp.Or(d.SameApplicantAction, DefaultSameApplicantAction)
	node.ConsecutiveApproverAction = cmp.Or(d.ConsecutiveApproverAction, DefaultConsecutiveApproverAction)
	node.RollbackType = cmp.Or(d.RollbackType, DefaultRollbackType)
	node.RollbackDataStrategy = cmp.Or(d.RollbackDataStrategy, DefaultRollbackDataStrategy)
	node.RollbackTargetKeys = d.RollbackTargetKeys
	node.IsRollbackAllowed = orTrue(d.IsRollbackAllowed)
	node.IsAddAssigneeAllowed = orTrue(d.IsAddAssigneeAllowed)
	node.AddAssigneeTypes = d.AddAssigneeTypes
	node.IsRemoveAssigneeAllowed = orTrue(d.IsRemoveAssigneeAllowed)
	node.IsManualCCAllowed = orTrue(d.IsManualCCAllowed)
}

// --- HandleNodeData ---

// HandleNodeData contains data specific to handle nodes.
type HandleNodeData struct {
	BaseNodeData
	TaskNodeData
}

// Kind returns the node kind.
func (*HandleNodeData) Kind() NodeKind { return NodeHandle }

// ApplyTo applies handle node data to a FlowNode, resolving omitted fields to
// the handle defaults (sequential execution, PassAny rule).
func (d *HandleNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	applyTaskNodeData(node, &d.TaskNodeData)

	node.ApprovalMethod = DefaultHandleApprovalMethod
	node.PassRule = DefaultHandlePassRule
}

// --- CCNodeData ---

// CCNodeData contains data specific to CC nodes.
type CCNodeData struct {
	BaseNodeData

	CCs                   []CCDefinition        `json:"ccs,omitempty"`
	IsReadConfirmRequired bool                  `json:"isReadConfirmRequired,omitempty"`
	FieldPermissions      map[string]Permission `json:"fieldPermissions,omitempty"`
}

// Kind returns the node kind.
func (*CCNodeData) Kind() NodeKind { return NodeCC }

// GetCCs returns the CC definitions from CCNodeData.
func (d *CCNodeData) GetCCs() []CCDefinition {
	return d.CCs
}

// ApplyTo applies CC node data to a FlowNode.
func (d *CCNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	node.IsReadConfirmRequired = d.IsReadConfirmRequired
	node.FieldPermissions = d.FieldPermissions
}

// --- ConditionNodeData ---

// ConditionNodeData contains data specific to condition nodes.
type ConditionNodeData struct {
	BaseNodeData

	Branches []ConditionBranch `json:"branches,omitempty"`
}

// Kind returns the node kind.
func (*ConditionNodeData) Kind() NodeKind { return NodeCondition }

// ApplyTo applies condition node data to a FlowNode.
func (d *ConditionNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	node.Branches = d.Branches
}
