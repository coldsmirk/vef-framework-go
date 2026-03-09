package strategy

import (
	"fmt"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StrategyRegistry holds all strategy implementations indexed by their type.
type StrategyRegistry struct {
	passRules  map[approval.PassRule]approval.PassRuleStrategy
	assignees  map[approval.AssigneeKind]AssigneeResolver
	conditions map[approval.ConditionKind]approval.ConditionEvaluator
	composite  *CompositeAssigneeResolver
}

// NewStrategyRegistry creates a registry from slices (designed for FX group injection).
func NewStrategyRegistry(
	passRules []approval.PassRuleStrategy,
	assignees []AssigneeResolver,
	conditions []approval.ConditionEvaluator,
) *StrategyRegistry {
	return &StrategyRegistry{
		passRules: streams.AssociateBy(streams.FromSlice(passRules), func(r approval.PassRuleStrategy) approval.PassRule {
			return r.Rule()
		}),
		assignees: streams.AssociateBy(streams.FromSlice(assignees), func(a AssigneeResolver) approval.AssigneeKind {
			return a.Kind()
		}),
		conditions: streams.AssociateBy(streams.FromSlice(conditions), func(c approval.ConditionEvaluator) approval.ConditionKind {
			return c.Kind()
		}),
		composite: NewCompositeAssigneeResolver(assignees...),
	}
}

// GetPassRuleStrategy returns the pass rule strategy for the given rule.
func (r *StrategyRegistry) GetPassRuleStrategy(rule approval.PassRule) (approval.PassRuleStrategy, error) {
	s, ok := r.passRules[rule]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPassRuleNotFound, rule)
	}

	return s, nil
}

// GetConditionEvaluator returns the condition evaluator for the given type.
func (r *StrategyRegistry) GetConditionEvaluator(t approval.ConditionKind) (approval.ConditionEvaluator, error) {
	s, ok := r.conditions[t]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrConditionEvaluatorNotFound, t)
	}

	return s, nil
}

// CompositeAssigneeResolver returns the cached CompositeAssigneeResolver.
func (r *StrategyRegistry) CompositeAssigneeResolver() *CompositeAssigneeResolver {
	return r.composite
}
