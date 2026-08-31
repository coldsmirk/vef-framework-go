package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// newBuiltinRegistry builds the registry production boots with, minus host
// registrations.
func newBuiltinRegistry(t *testing.T) *StrategyRegistry {
	t.Helper()

	assignees, err := NewCompositeAssigneeResolver(BuiltinAssigneeResolvers(nil), nil)
	require.NoError(t, err, "Should build built-in assignee resolvers")

	ccs, err := NewCompositeCCResolver(BuiltinCCResolvers(nil), nil)
	require.NoError(t, err, "Should build built-in CC resolvers")

	initiators, err := NewCompositeInitiatorResolver(BuiltinInitiatorResolvers(nil), nil)
	require.NoError(t, err, "Should build built-in initiator resolvers")

	return NewStrategyRegistry(
		[]approval.PassRuleStrategy{NewAllPassStrategy(), NewAnyPassStrategy(), NewRatioPassStrategy()},
		[]approval.ConditionEvaluator{NewFieldConditionEvaluator(), NewExpressionConditionEvaluator(nil)},
		assignees,
		ccs,
		initiators,
	)
}

// TestNewStrategyRegistry tests new strategy registry scenarios.
func TestNewStrategyRegistry(t *testing.T) {
	t.Run("RegistersAll", func(t *testing.T) {
		r := newBuiltinRegistry(t)

		assert.Len(t, r.passRules, 3, "Should register 3 pass rule strategies")
		assert.Len(t, r.conditions, 2, "Should register 2 condition evaluators")
		assert.Len(t, r.Assignees().Descriptors(), len(expectedAssigneeKinds), "Should register every built-in assignee kind")
		assert.Len(t, r.CCs().Descriptors(), len(expectedCCKinds), "Should register every built-in CC kind")
		assert.Len(t, r.Initiators().Descriptors(), len(expectedInitiatorKinds), "Should register every built-in initiator kind")
	})

	t.Run("NilSlices", func(t *testing.T) {
		r := NewStrategyRegistry(nil, nil, nil, nil, nil)

		assert.Empty(t, r.passRules, "Should have no pass rules for nil input")
		assert.Empty(t, r.conditions, "Should have no conditions for nil input")
	})
}

// TestValidateBuiltins tests that boot validation covers every vocabulary.
func TestValidateBuiltins(t *testing.T) {
	t.Run("CompleteRegistry", func(t *testing.T) {
		require.NoError(t, newBuiltinRegistry(t).ValidateBuiltins(), "A complete registry should validate")
	})

	t.Run("MissingPassRule", func(t *testing.T) {
		r := newBuiltinRegistry(t)
		delete(r.passRules, approval.PassRatio)

		require.ErrorIs(t, r.ValidateBuiltins(), errBuiltinPassRuleMissing, "Should report the missing pass rule")
	})

	t.Run("MissingConditionEvaluator", func(t *testing.T) {
		r := newBuiltinRegistry(t)
		delete(r.conditions, approval.ConditionExpression)

		require.ErrorIs(t, r.ValidateBuiltins(), errBuiltinEvaluatorMissing, "Should report the missing evaluator")
	})

	t.Run("MissingAssigneeKind", func(t *testing.T) {
		r := newBuiltinRegistry(t)
		delete(r.assignees.resolvers, approval.AssigneeSuperior)

		require.ErrorIs(t, r.ValidateBuiltins(), errBuiltinAssigneeMissing, "Should report the missing assignee kind")
	})

	t.Run("MissingCCKind", func(t *testing.T) {
		r := newBuiltinRegistry(t)
		delete(r.ccs.resolvers, approval.CCRole)

		require.ErrorIs(t, r.ValidateBuiltins(), errBuiltinCCMissing, "Should report the missing CC kind")
	})

	t.Run("MissingInitiatorKind", func(t *testing.T) {
		r := newBuiltinRegistry(t)
		delete(r.initiators.resolvers, approval.InitiatorDepartment)

		require.ErrorIs(t, r.ValidateBuiltins(), errBuiltinInitiatorMissing, "Should report the missing initiator kind")
	})

	t.Run("NilComposites", func(t *testing.T) {
		r := NewStrategyRegistry(
			[]approval.PassRuleStrategy{NewAllPassStrategy(), NewAnyPassStrategy(), NewRatioPassStrategy()},
			[]approval.ConditionEvaluator{NewFieldConditionEvaluator(), NewExpressionConditionEvaluator(nil)},
			nil, nil, nil,
		)

		require.ErrorIs(t, r.ValidateBuiltins(), errNilResolver, "A registry without resolvers should not validate")
	})
}

// TestGetPassRuleStrategy tests get pass rule strategy scenarios.
func TestGetPassRuleStrategy(t *testing.T) {
	r := NewStrategyRegistry(
		[]approval.PassRuleStrategy{NewAllPassStrategy()},
		nil, nil, nil, nil,
	)

	t.Run("Found", func(t *testing.T) {
		s, err := r.GetPassRuleStrategy(approval.PassAll)
		require.NoError(t, err, "Should find AllPassStrategy")
		assert.IsType(t, &AllPassStrategy{}, s, "Should be expected type")
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := r.GetPassRuleStrategy("nonexistent")
		require.ErrorIs(t, err, ErrPassRuleNotFound, "Should return ErrPassRuleNotFound")
	})
}

// TestGetConditionEvaluator tests get condition evaluator scenarios.
func TestGetConditionEvaluator(t *testing.T) {
	r := NewStrategyRegistry(
		nil,
		[]approval.ConditionEvaluator{NewFieldConditionEvaluator()},
		nil, nil, nil,
	)

	t.Run("Found", func(t *testing.T) {
		e, err := r.GetConditionEvaluator(approval.ConditionField)
		require.NoError(t, err, "Should find FieldConditionEvaluator")
		assert.IsType(t, &FieldConditionEvaluator{}, e, "Should be expected type")
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := r.GetConditionEvaluator("nonexistent")
		require.ErrorIs(t, err, ErrConditionEvaluatorNotFound, "Should return ErrConditionEvaluatorNotFound")
	})
}

// TestRegistryResolverAccessors tests the composite accessors.
func TestRegistryResolverAccessors(t *testing.T) {
	r := newBuiltinRegistry(t)

	assert.NotNil(t, r.Assignees(), "Should return the assignee composite")
	assert.NotNil(t, r.CCs(), "Should return the CC composite")
	assert.NotNil(t, r.Initiators(), "Should return the initiator composite")
}
