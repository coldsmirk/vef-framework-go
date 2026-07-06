package service

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// validateNodeConfig validates the parsed node data of a single node. It is
// the semantic counterpart to the structural checks in ValidateFlowDefinition:
// every enum must be a defined value (omitted fields are fine — ApplyTo
// resolves them to defaults at deploy) and every action that depends on a
// companion field must have that field populated. Anything the engine could
// not execute is rejected here, at deploy time, instead of stalling a running
// instance.
func validateNodeConfig(nodeID string, data approval.NodeData) error {
	switch typed := data.(type) {
	case *approval.ApprovalNodeData:
		return validateApprovalNodeData(nodeID, typed)
	case *approval.HandleNodeData:
		return validateHandleNodeData(nodeID, typed)
	case *approval.CCNodeData:
		return validateCCDefinitions(nodeID, typed.CCs)
	case *approval.ConditionNodeData:
		return validateConditionBranches(nodeID, typed.Branches)
	default:
		return nil
	}
}

// validateHandleNodeData applies the shared task-node checks plus the
// handle-specific restriction: a handle node performs work, it is not a
// decision point, so neither its execution type nor its timeout action may
// reject the whole instance. The designer offers the same narrowed sets.
func validateHandleNodeData(nodeID string, data *approval.HandleNodeData) error {
	if err := validateTaskNodeData(nodeID, &data.TaskNodeData); err != nil {
		return err
	}

	if data.ExecutionType == approval.ExecutionAutoReject {
		return fmt.Errorf("%w: node %q", errHandleExecutionAutoReject, nodeID)
	}

	if data.TimeoutAction == approval.TimeoutActionAutoReject {
		return fmt.Errorf("%w: node %q", errHandleTimeoutAutoReject, nodeID)
	}

	return nil
}

func validateApprovalNodeData(nodeID string, data *approval.ApprovalNodeData) error {
	if err := validateTaskNodeData(nodeID, &data.TaskNodeData); err != nil {
		return err
	}

	if data.ApprovalMethod != "" && !data.ApprovalMethod.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidApprovalMethod, data.ApprovalMethod, nodeID)
	}

	if data.PassRule != "" && !data.PassRule.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidPassRule, data.PassRule, nodeID)
	}

	if err := validatePassRatio(nodeID, data); err != nil {
		return err
	}

	if data.SameApplicantAction != "" && !data.SameApplicantAction.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidSameApplicantAction, data.SameApplicantAction, nodeID)
	}

	if data.ConsecutiveApproverAction != "" && !data.ConsecutiveApproverAction.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidConsecutiveAction, data.ConsecutiveApproverAction, nodeID)
	}

	if data.RollbackType != "" && !data.RollbackType.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidRollbackType, data.RollbackType, nodeID)
	}

	if data.RollbackType == approval.RollbackSpecified && len(data.RollbackTargetKeys) == 0 {
		return fmt.Errorf("%w: node %q", errRollbackTargetsRequired, nodeID)
	}

	if data.RollbackDataStrategy != "" && !data.RollbackDataStrategy.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidRollbackStrategy, data.RollbackDataStrategy, nodeID)
	}

	for _, addType := range data.AddAssigneeTypes {
		if !addType.IsValid() {
			return fmt.Errorf("%w: %q in node %q", errInvalidAddAssigneeType, addType, nodeID)
		}
	}

	// A sequential node advances its queue one task at a time, so a
	// "parallel" addition has nothing to join — reject the configuration at
	// deploy instead of letting every add attempt fail at runtime.
	if cmp.Or(data.ApprovalMethod, approval.DefaultApprovalMethod) == approval.ApprovalSequential &&
		slices.Contains(data.AddAssigneeTypes, approval.AddAssigneeParallel) {
		return fmt.Errorf("%w: node %q", errSequentialParallelAdd, nodeID)
	}

	return nil
}

// validatePassRatio requires an explicit, in-range ratio whenever the node
// resolves to the ratio pass rule. The single storage convention is a
// percentage in (0, 100] — the engine consumes the stored value verbatim, so
// anything outside that range could never pass (or always would). Zero is
// rejected rather than defaulted: a ratio node without a threshold would
// otherwise pass on the first evaluation regardless of votes.
func validatePassRatio(nodeID string, data *approval.ApprovalNodeData) error {
	if data.PassRule != approval.PassRatio {
		return nil
	}

	ratio := data.PassRatio.InexactFloat64()
	if ratio <= 0 || ratio > 100 {
		return fmt.Errorf("%w: got %v in node %q", errPassRatioOutOfRange, data.PassRatio, nodeID)
	}

	return nil
}

func validateTaskNodeData(nodeID string, data *approval.TaskNodeData) error {
	if data.ExecutionType != "" && !data.ExecutionType.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidExecutionType, data.ExecutionType, nodeID)
	}

	if data.EmptyAssigneeAction != "" && !data.EmptyAssigneeAction.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidEmptyAssigneeAction, data.EmptyAssigneeAction, nodeID)
	}

	if data.EmptyAssigneeAction == approval.EmptyAssigneeTransferSpecified && len(data.FallbackUserIDs) == 0 {
		return fmt.Errorf("%w: node %q", errFallbackUsersRequired, nodeID)
	}

	if data.EmptyAssigneeAction == approval.EmptyAssigneeTransferAdmin && len(data.AdminUserIDs) == 0 {
		return fmt.Errorf("%w: node %q", errAdminUsersRequired, nodeID)
	}

	if data.TimeoutAction != "" && !data.TimeoutAction.IsValid() {
		return fmt.Errorf("%w: %q in node %q", errInvalidTimeoutAction, data.TimeoutAction, nodeID)
	}

	if err := validateAssigneeDefinitions(nodeID, data.Assignees); err != nil {
		return err
	}

	return validateCCDefinitions(nodeID, data.CCs)
}

func validateAssigneeDefinitions(nodeID string, assignees []approval.AssigneeDefinition) error {
	for _, assignee := range assignees {
		if !assignee.Kind.IsValid() {
			return fmt.Errorf("%w: %q in node %q", errInvalidAssigneeKind, assignee.Kind, nodeID)
		}

		if assignee.Kind == approval.AssigneeFormField && !hasText(assignee.FormField) {
			return fmt.Errorf("%w: node %q", errAssigneeFormFieldRequired, nodeID)
		}
	}

	return nil
}

func validateCCDefinitions(nodeID string, ccs []approval.CCDefinition) error {
	for _, cc := range ccs {
		if !cc.Kind.IsValid() {
			return fmt.Errorf("%w: %q in node %q", errInvalidCCKind, cc.Kind, nodeID)
		}

		if cc.Kind == approval.CCFormField && !hasText(cc.FormField) {
			return fmt.Errorf("%w: node %q", errCCFormFieldRequired, nodeID)
		}

		if cc.Timing != "" && !cc.Timing.IsValid() {
			return fmt.Errorf("%w: %q in node %q", errInvalidCCTiming, cc.Timing, nodeID)
		}
	}

	return nil
}

// validateConditionBranches checks every branch condition is executable:
// known kind, and the kind-specific payload present (subject + whitelisted
// operator for field conditions, non-blank source for expression conditions).
// Expression syntax itself is not verified at deploy time — deploy only
// checks the source is non-blank — so a syntactically broken expression still
// surfaces at evaluation; this guard eliminates the structurally-empty cases.
//
// Non-default branches must also carry unique priorities: "first match wins"
// is only a meaningful contract when the evaluation order is total, so a tie
// is a design error rather than something to resolve arbitrarily at runtime.
//
// Structurally-empty conditions are rejected for the same reason: the engine
// treats an empty group list (or an empty group) as an unconditional match,
// which would silently shadow every lower-priority branch and the default.
func validateConditionBranches(nodeID string, branches []approval.ConditionBranch) error {
	seenPriorities := make(map[int]string, len(branches))

	for _, branch := range branches {
		if !branch.IsDefault {
			if prevID, dup := seenPriorities[branch.Priority]; dup {
				return fmt.Errorf("%w: priority %d shared by branches %q and %q in node %q",
					errDuplicateBranchPriority, branch.Priority, prevID, branch.ID, nodeID)
			}

			seenPriorities[branch.Priority] = branch.ID

			if len(branch.ConditionGroups) == 0 {
				return fmt.Errorf("%w: branch %q in node %q", errBranchConditionsRequired, branch.ID, nodeID)
			}
		}

		for _, group := range branch.ConditionGroups {
			if len(group.Conditions) == 0 {
				return fmt.Errorf("%w: branch %q in node %q", errConditionGroupEmpty, branch.ID, nodeID)
			}

			for _, cond := range group.Conditions {
				if err := validateCondition(nodeID, branch.ID, cond); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateCondition(nodeID, branchID string, cond approval.Condition) error {
	switch cond.Kind {
	case approval.ConditionField:
		if cond.Subject == "" {
			return fmt.Errorf("%w: branch %q in node %q", errConditionSubjectRequired, branchID, nodeID)
		}

		if !cond.Operator.IsValid() {
			return fmt.Errorf("%w: %q in branch %q of node %q", errInvalidConditionOperator, cond.Operator, branchID, nodeID)
		}

		return nil

	case approval.ConditionExpression:
		if strings.TrimSpace(cond.Expression) == "" {
			return fmt.Errorf("%w: branch %q in node %q", errConditionExprRequired, branchID, nodeID)
		}

		return nil

	default:
		return fmt.Errorf("%w: %q in branch %q of node %q", errInvalidConditionKind, cond.Kind, branchID, nodeID)
	}
}

// hasText reports whether the optional string pointer holds non-blank text.
func hasText(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}
