package engine

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
)

// ConditionProcessor evaluates condition branches and selects the matching branch.
type ConditionProcessor struct{}

// NewConditionProcessor creates a ConditionProcessor.
func NewConditionProcessor() NodeProcessor { return &ConditionProcessor{} }

func (*ConditionProcessor) NodeKind() approval.NodeKind { return approval.NodeCondition }

func (*ConditionProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	branches := slices.Clone(pc.Node.Branches)
	if len(branches) == 0 {
		return nil, ErrNoBranches
	}

	slices.SortFunc(branches, func(a, b approval.ConditionBranch) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	formData := approval.NewFormData(pc.Instance.FormData)

	evalCtx := &approval.EvaluationContext{
		FormData:        formData,
		ApplicantID:     pc.Instance.ApplicantID,
		ApplicantDeptID: pc.Instance.ApplicantDeptID,
	}

	var defaultBranch *approval.ConditionBranch

	for i := range branches {
		branch := &branches[i]
		if branch.IsDefault {
			defaultBranch = branch

			continue
		}

		match, err := evaluateConditionGroups(pc.Registry, ctx, evalCtx, branch.ConditionGroups)
		if err != nil {
			return nil, fmt.Errorf("evaluate branch %q: %w", branch.Label, err)
		}

		if match {
			return &ProcessResult{Action: NodeActionContinue, BranchID: new(branch.ID)}, nil
		}
	}

	if defaultBranch != nil {
		return &ProcessResult{Action: NodeActionContinue, BranchID: new(defaultBranch.ID)}, nil
	}

	return nil, ErrNoMatchingBranch
}

// evaluateConditionGroups evaluates condition groups using OR between groups, AND within each group.
//
//nolint:revive // registry is the primary dependency
func evaluateConditionGroups(registry *strategy.StrategyRegistry, ctx context.Context, evalCtx *approval.EvaluationContext, groups []approval.ConditionGroup) (bool, error) {
	if len(groups) == 0 {
		return true, nil
	}

	for _, group := range groups {
		match, err := evaluateGroupConditions(registry, ctx, evalCtx, group.Conditions)
		if err != nil {
			return false, err
		}

		if match {
			return true, nil
		}
	}

	return false, nil
}

// evaluateGroupConditions evaluates a set of conditions using AND logic.
//
//nolint:revive // registry is the primary dependency
func evaluateGroupConditions(registry *strategy.StrategyRegistry, ctx context.Context, evalCtx *approval.EvaluationContext, conditions []approval.Condition) (bool, error) {
	if len(conditions) == 0 {
		return true, nil
	}

	for _, condition := range conditions {
		evaluator, err := registry.GetConditionEvaluator(condition.Kind)
		if err != nil {
			return false, err
		}

		match, err := evaluator.Evaluate(ctx, condition, evalCtx)
		if err != nil {
			return false, err
		}

		if !match {
			return false, nil
		}
	}

	return true, nil
}
