package strategy

import (
	"fmt"

	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// StrategyRegistry holds all strategy implementations indexed by their type.
type StrategyRegistry struct {
	passRules  map[approval.PassRule]approval.PassRuleStrategy
	conditions map[approval.ConditionKind]approval.ConditionEvaluator
	assignees  *CompositeAssigneeResolver
	ccs        *CompositeCCResolver
	initiators *CompositeInitiatorResolver
}

// expectedPassRules lists the built-in PassRule values the framework guarantees
// have a registered strategy. Missing registrations surface at boot.
var expectedPassRules = []approval.PassRule{
	approval.PassAll,
	approval.PassAny,
	approval.PassRatio,
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

// expectedCCKinds lists the built-in CCKind values the framework guarantees
// have a registered resolver.
var expectedCCKinds = []approval.CCKind{
	approval.CCUser,
	approval.CCRole,
	approval.CCDepartment,
	approval.CCFormField,
}

// expectedInitiatorKinds lists the built-in InitiatorKind values the framework
// guarantees have a registered resolver.
var expectedInitiatorKinds = []approval.InitiatorKind{
	approval.InitiatorUser,
	approval.InitiatorRole,
	approval.InitiatorDepartment,
}

// expectedConditionKinds lists the built-in ConditionKind values the framework
// guarantees have a registered evaluator. Missing registrations surface at boot.
var expectedConditionKinds = []approval.ConditionKind{
	approval.ConditionField,
	approval.ConditionExpression,
}

// NewStrategyRegistry creates a registry from the pass-rule and condition
// strategy groups plus the three composite principal resolvers. It does not
// validate completeness on construction so test suites can build registries
// with partial strategy sets. Production modules call ValidateBuiltins via
// fx.Invoke after construction.
func NewStrategyRegistry(
	passRules []approval.PassRuleStrategy,
	conditions []approval.ConditionEvaluator,
	assignees *CompositeAssigneeResolver,
	ccs *CompositeCCResolver,
	initiators *CompositeInitiatorResolver,
) *StrategyRegistry {
	return &StrategyRegistry{
		passRules: streams.AssociateBy(streams.FromSlice(passRules), func(r approval.PassRuleStrategy) approval.PassRule {
			return r.Rule()
		}),
		conditions: streams.AssociateBy(streams.FromSlice(conditions), func(c approval.ConditionEvaluator) approval.ConditionKind {
			return c.Kind()
		}),
		assignees:  assignees,
		ccs:        ccs,
		initiators: initiators,
	}
}

// ValidateBuiltins asserts that every built-in enum value the framework
// guarantees has a registered strategy. The production strategy.Module
// invokes this during boot so misconfigurations surface immediately rather
// than as a runtime "strategy not found" error deep inside a flow.
//
// It covers the three principal vocabularies too. They are open registries —
// a host may add kinds and override built-ins — but a built-in kind that no
// longer resolves means the framework's own registration list and its
// constants drifted apart, and every flow already using that kind would fail
// at the node that references it.
func (r *StrategyRegistry) ValidateBuiltins() error {
	for _, rule := range expectedPassRules {
		if _, ok := r.passRules[rule]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinPassRuleMissing, rule)
		}
	}

	for _, kind := range expectedConditionKinds {
		if _, ok := r.conditions[kind]; !ok {
			return fmt.Errorf("%w: %s", errBuiltinEvaluatorMissing, kind)
		}
	}

	if r.assignees == nil || r.ccs == nil || r.initiators == nil {
		return errNilResolver
	}

	if err := requireKinds(r.assignees.resolvers, expectedAssigneeKinds, errBuiltinAssigneeMissing); err != nil {
		return err
	}

	if err := requireKinds(r.ccs.resolvers, expectedCCKinds, errBuiltinCCMissing); err != nil {
		return err
	}

	return requireKinds(r.initiators.resolvers, expectedInitiatorKinds, errBuiltinInitiatorMissing)
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

// Assignees returns the composite assignee resolver.
func (r *StrategyRegistry) Assignees() *CompositeAssigneeResolver { return r.assignees }

// CCs returns the composite CC resolver.
func (r *StrategyRegistry) CCs() *CompositeCCResolver { return r.ccs }

// Initiators returns the composite initiator resolver.
func (r *StrategyRegistry) Initiators() *CompositeInitiatorResolver { return r.initiators }
