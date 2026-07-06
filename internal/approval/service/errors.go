package service

import "errors"

// Flow-definition validation sentinels. These are internal errors wrapped
// into shared.ErrInvalidFlowDesign at the command layer and never surfaced raw.
var (
	errEmptyNodeID        = errors.New("node ID must not be empty")
	errDuplicateNodeID    = errors.New("duplicate node ID")
	errInvalidNodeKind    = errors.New("invalid node kind")
	errStartNodeCount     = errors.New("flow must have exactly 1 start node")
	errEndNodeCount       = errors.New("flow must have at least 1 end node")
	errEmptyEdgeID        = errors.New("edge ID must not be empty")
	errDuplicateEdgeID    = errors.New("duplicate edge ID")
	errUnknownSourceNode  = errors.New("edge references unknown source node")
	errUnknownTargetNode  = errors.New("edge references unknown target node")
	errStartIncoming      = errors.New("start node must not have incoming edges")
	errStartOutgoing      = errors.New("start node must have exactly 1 outgoing edge")
	errEndOutgoing        = errors.New("end node must not have outgoing edges")
	errEndIncoming        = errors.New("end node must have at least 1 incoming edge")
	errNodeOutgoingCount  = errors.New("node must have exactly 1 outgoing edge")
	errNodeSourceHandle   = errors.New("non-condition node must not have sourceHandle on outgoing edge")
	errGraphCycle         = errors.New("flow graph contains a cycle")
	errNodeUnreachable    = errors.New("node is not reachable from start node")
	errNodeCannotReachEnd = errors.New("node cannot reach end node")
	errNoNodes            = errors.New("flow must have at least one node")
	errCondMinBranches    = errors.New("condition node must have at least 2 branches")
	errCondEmptyBranchID  = errors.New("condition node has a branch with empty ID")
	errCondDupBranchID    = errors.New("condition node has duplicate branch ID")
	errCondDefaultCount   = errors.New("condition node must have exactly 1 default branch")
	errCondMissingHandle  = errors.New("edge must have a sourceHandle")
	errCondUnknownHandle  = errors.New("edge has unknown sourceHandle")
	errCondDupHandle      = errors.New("duplicate outgoing edge for handle")
	errCondBranchNoEdge   = errors.New("branch has no outgoing edge")
	errUnexpectedCondData = errors.New("unexpected condition node data type")
	errInvalidCCKind      = errors.New("invalid cc kind")
)

// Node-configuration validation sentinels. Deploy-time guards that reject any
// enum or dependent-field combination the engine could not execute, so a
// misconfigured node fails loudly at deploy instead of stalling an instance
// at runtime. Like the structural sentinels above, they surface wrapped in
// shared.ErrInvalidFlowDesign.
var (
	errInvalidExecutionType       = errors.New("invalid execution type")
	errInvalidApprovalMethod      = errors.New("invalid approval method")
	errInvalidPassRule            = errors.New("invalid pass rule")
	errPassRatioOutOfRange        = errors.New("pass ratio must be a percentage within (0, 100]")
	errInvalidEmptyAssigneeAction = errors.New("invalid empty-assignee action")
	errInvalidSameApplicantAction = errors.New("invalid same-applicant action")
	errInvalidConsecutiveAction   = errors.New("invalid consecutive-approver action")
	errInvalidRollbackType        = errors.New("invalid rollback type")
	errInvalidRollbackStrategy    = errors.New("invalid rollback data strategy")
	errRollbackTargetsRequired    = errors.New("rollback type 'specified' requires rollbackTargetKeys")
	errInvalidTimeoutAction       = errors.New("invalid timeout action")
	errInvalidAssigneeKind        = errors.New("invalid assignee kind")
	errAssigneeFormFieldRequired  = errors.New("assignee kind 'form_field' requires a form field name")
	errInvalidCCTiming            = errors.New("invalid cc timing")
	errCCFormFieldRequired        = errors.New("cc kind 'form_field' requires a form field name")
	errFallbackUsersRequired      = errors.New("empty-assignee action 'transfer_specified' requires fallbackUserIds")
	errAdminUsersRequired         = errors.New("empty-assignee action 'transfer_admin' requires adminUserIds")
	errInvalidConditionKind       = errors.New("invalid condition kind")
	errInvalidConditionOperator   = errors.New("invalid condition operator")
	errConditionSubjectRequired   = errors.New("field condition requires a subject")
	errConditionExprRequired      = errors.New("expression condition requires an expression")
	errDuplicateBranchPriority    = errors.New("condition branches must have unique priorities")
	errHandleExecutionAutoReject  = errors.New("handle nodes do not support execution type 'auto_reject'")
	errHandleTimeoutAutoReject    = errors.New("handle nodes do not support timeout action 'auto_reject'")
	errRollbackTargetUnknown      = errors.New("rollback target key does not reference an approval or handle node in the flow")
	errRollbackTargetSelf         = errors.New("rollback target keys must not include the node itself")
	errInvalidAddAssigneeType     = errors.New("invalid add-assignee type")
	errSequentialParallelAdd      = errors.New("sequential nodes do not support add-assignee type 'parallel'")
	errBranchConditionsRequired   = errors.New("non-default condition branch requires at least one condition group")
	errConditionGroupEmpty        = errors.New("condition group must contain at least one condition")
	errUnregisteredAggregate      = errors.New("aggregate kind has no registered aggregator")
	errAggregateOnExpression      = errors.New("expression conditions must not declare an aggregate")
	errAggregateOperator          = errors.New("aggregate conditions support numeric comparison operators only")
	errAggregateColumnRequired    = errors.New("sum/avg aggregates require a column")
	errAggregateColumnForbidden   = errors.New("count aggregates must not declare a column")
	errAggregateSubjectNotTable   = errors.New("aggregate subject must reference a table field")
	errAggregateColumnUnknown     = errors.New("aggregate column does not exist in the table field")
	errAggregateColumnNotNumeric  = errors.New("aggregate column must be a number field")
)

// Form-definition validation sentinels. Deploy-time guards over the form
// schema attached to a flow version; without them a broken schema (duplicate
// keys, invalid kind, uncompilable pattern) only surfaces when an applicant
// submits — misreported as a data error. Surface wrapped in
// shared.ErrInvalidFormDesign.
var (
	errFormFieldKeyEmpty      = errors.New("form field key must not be empty")
	errDuplicateFormFieldKey  = errors.New("duplicate form field key")
	errInvalidFormFieldKind   = errors.New("invalid form field kind")
	errInvalidFormPattern     = errors.New("form field validation pattern does not compile")
	errInvalidFormLengthRange = errors.New("form field minLength must not exceed maxLength")
	errInvalidFormValueRange  = errors.New("form field min must not exceed max")
	errTableColumnsRequired   = errors.New("table field requires at least one column")
	errNestedTableColumn      = errors.New("table columns must not nest another table")
	errColumnsOnScalarField   = errors.New("only table fields may declare columns")
)
