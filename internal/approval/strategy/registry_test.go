package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestNewStrategyRegistry tests new strategy registry scenarios.
func TestNewStrategyRegistry(t *testing.T) {
	t.Run("RegistersAll", func(t *testing.T) {
		r := NewStrategyRegistry(
			[]approval.PassRuleStrategy{NewAllPassStrategy(), NewOnePassStrategy()},
			[]AssigneeResolver{NewUserAssigneeResolver(), NewSelfAssigneeResolver()},
			[]approval.ConditionEvaluator{NewFieldConditionEvaluator()},
		)

		assert.Len(t, r.passRules, 2, "Should register 2 pass rule strategies")
		assert.Len(t, r.assignees, 2, "Should register 2 assignee resolvers")
		assert.Len(t, r.conditions, 1, "Should register 1 condition evaluator")
		assert.NotNil(t, r.composite, "Composite resolver should be initialized")
	})

	t.Run("NilSlices", func(t *testing.T) {
		r := NewStrategyRegistry(nil, nil, nil)

		assert.Empty(t, r.passRules, "Should have no pass rules for nil input")
		assert.Empty(t, r.assignees, "Should have no assignees for nil input")
		assert.Empty(t, r.conditions, "Should have no conditions for nil input")
		assert.NotNil(t, r.composite, "Composite resolver should be initialized even with nil")
	})
}

// TestGetPassRuleStrategy tests get pass rule strategy scenarios.
func TestGetPassRuleStrategy(t *testing.T) {
	r := NewStrategyRegistry(
		[]approval.PassRuleStrategy{NewAllPassStrategy()},
		nil, nil,
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
		nil, nil,
		[]approval.ConditionEvaluator{NewFieldConditionEvaluator()},
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

// TestRegistryCompositeAssigneeResolver tests registry composite resolver accessor.
func TestRegistryCompositeAssigneeResolver(t *testing.T) {
	r := NewStrategyRegistry(
		nil,
		[]AssigneeResolver{NewUserAssigneeResolver(), NewSelfAssigneeResolver()},
		nil,
	)

	composite := r.CompositeAssigneeResolver()
	assert.NotNil(t, composite, "Should return non-nil composite resolver")
	assert.Len(t, composite.resolvers, 2, "Composite should contain 2 resolvers")
}
