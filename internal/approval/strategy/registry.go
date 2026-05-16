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

// expectedPassRules lists the built-in PassRule values the framework guarantees
// have a registered strategy. Missing registrations surface at boot.
var expectedPassRules = []approval.PassRule{
	approval.PassAll,
	approval.PassAny,
	approval.PassRatio,
	approval.PassAnyReject,
}

// expectedAssigneeKinds lists the built-in AssigneeKind values the framework
// guarantees have a registered resolver. Missing registrations surface at boot.
var expectedAssigneeKinds = []approval.AssigneeKind{
	approval.AssigneeUser,
	approval.AssigneeRole,
	approval.AssigneeDepartment,
	approval.AssigneeSelf,
	approval.AssigneeSuperior,
	approval.AssigneeDepartmentLeader,
	approval.AssigneeFormField,
}

// expectedConditionKinds lists the built-in ConditionKind values the framework
// guarantees have a registered evaluator. Missing registrations surface at boot.
var expectedConditionKinds = []approval.ConditionKind{
	approval.ConditionField,
	approval.ConditionExpression,
}

// NewStrategyRegistry creates a registry from slices (designed for FX group
// injection). It does not validate completeness on construction so test
// suites can build registries with partial strategy sets. Production
// modules call ValidateBuiltins via fx.Invoke after construction.
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

// ValidateBuiltins asserts that every built-in enum value the framework
// guarantees has a registered strategy. The production strategy.Module
// invokes this during boot so misconfigurations surface immediately rather
// than as a runtime "strategy not found" error deep inside a flow.
func (r *StrategyRegistry) ValidateBuiltins() error {
	for _, rule := range expectedPassRules {
		if _, ok := r.passRules[rule]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinPassRuleMissing, rule)
		}
	}

	for _, kind := range expectedAssigneeKinds {
		if _, ok := r.assignees[kind]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinAssigneeMissing, kind)
		}
	}

	for _, kind := range expectedConditionKinds {
		if _, ok := r.conditions[kind]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinEvaluatorMissing, kind)
		}
	}

	return nil
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
