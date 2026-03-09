package approval

import (
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
	// ApplyTo applies this node data's configuration to the given FlowNode.
	ApplyTo(node *FlowNode)
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
type TaskNodeData struct {
	Assignees                []AssigneeDefinition  `json:"assignees,omitempty"`
	ExecutionType            ExecutionType         `json:"executionType,omitempty"`
	EmptyAssigneeAction      EmptyAssigneeAction   `json:"emptyAssigneeAction,omitempty"`
	FallbackUserIDs          []string              `json:"fallbackUserIds,omitempty"`
	AdminUserIDs             []string              `json:"adminUserIds,omitempty"`
	IsTransferAllowed        bool                  `json:"isTransferAllowed,omitempty"`
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

// applyTaskNodeData applies TaskNodeData fields to a FlowNode.
func applyTaskNodeData(node *FlowNode, data *TaskNodeData) {
	if data.ExecutionType != "" {
		node.ExecutionType = data.ExecutionType
	}

	if data.EmptyAssigneeAction != "" {
		node.EmptyAssigneeAction = data.EmptyAssigneeAction
	}

	node.FallbackUserIDs = data.FallbackUserIDs
	node.AdminUserIDs = data.AdminUserIDs
	node.IsTransferAllowed = data.IsTransferAllowed
	node.IsOpinionRequired = data.IsOpinionRequired
	node.TimeoutHours = data.TimeoutHours

	if data.TimeoutAction != "" {
		node.TimeoutAction = data.TimeoutAction
	}

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
	IsRollbackAllowed         bool                      `json:"isRollbackAllowed,omitempty"`
	IsAddAssigneeAllowed      bool                      `json:"isAddAssigneeAllowed,omitempty"`
	AddAssigneeTypes          []AddAssigneeType         `json:"addAssigneeTypes,omitempty"`
	IsRemoveAssigneeAllowed   bool                      `json:"isRemoveAssigneeAllowed,omitempty"`
	IsManualCCAllowed         bool                      `json:"isManualCcAllowed,omitempty"`
}

// Kind returns the node kind.
func (*ApprovalNodeData) Kind() NodeKind { return NodeApproval }

// ApplyTo applies approval node data to a FlowNode.
func (d *ApprovalNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	applyTaskNodeData(node, &d.TaskNodeData)

	if d.ApprovalMethod != "" {
		node.ApprovalMethod = d.ApprovalMethod
	}

	if d.PassRule != "" {
		node.PassRule = d.PassRule
	}

	if !d.PassRatio.IsZero() {
		node.PassRatio = d.PassRatio
	}

	if d.SameApplicantAction != "" {
		node.SameApplicantAction = d.SameApplicantAction
	}

	if d.ConsecutiveApproverAction != "" {
		node.ConsecutiveApproverAction = d.ConsecutiveApproverAction
	}

	if d.RollbackType != "" {
		node.RollbackType = d.RollbackType
	}

	if d.RollbackDataStrategy != "" {
		node.RollbackDataStrategy = d.RollbackDataStrategy
	}

	node.RollbackTargetKeys = d.RollbackTargetKeys
	node.IsRollbackAllowed = d.IsRollbackAllowed
	node.IsAddAssigneeAllowed = d.IsAddAssigneeAllowed
	node.AddAssigneeTypes = d.AddAssigneeTypes
	node.IsRemoveAssigneeAllowed = d.IsRemoveAssigneeAllowed
	node.IsManualCCAllowed = d.IsManualCCAllowed
}

// --- HandleNodeData ---

// HandleNodeData contains data specific to handle nodes.
type HandleNodeData struct {
	BaseNodeData
	TaskNodeData
}

// Kind returns the node kind.
func (*HandleNodeData) Kind() NodeKind { return NodeHandle }

// ApplyTo applies handle node data to a FlowNode.
// Handle nodes default to sequential execution with PassAny rule,
// since any handler completing the task is sufficient.
func (d *HandleNodeData) ApplyTo(node *FlowNode) {
	applyBaseNodeData(node, &d.BaseNodeData)
	applyTaskNodeData(node, &d.TaskNodeData)

	if node.ApprovalMethod == "" {
		node.ApprovalMethod = ApprovalSequential
	}

	if node.PassRule == "" {
		node.PassRule = PassAny
	}
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
