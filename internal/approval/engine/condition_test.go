package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
)

// --- Test Helpers ---

// MockConditionEvaluator is a configurable mock for approval.ConditionEvaluator.
type MockConditionEvaluator struct {
	kind approval.ConditionKind
	fn   func(ctx context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error)
}

func (m *MockConditionEvaluator) Kind() approval.ConditionKind { return m.kind }

func (m *MockConditionEvaluator) Evaluate(ctx context.Context, cond approval.Condition, ec *approval.EvaluationContext) (bool, error) {
	return m.fn(ctx, cond, ec)
}

// alwaysMatch returns a MockConditionEvaluator that always matches.
func alwaysMatch(kind approval.ConditionKind) *MockConditionEvaluator {
	return &MockConditionEvaluator{kind: kind, fn: func(context.Context, approval.Condition, *approval.EvaluationContext) (bool, error) {
		return true, nil
	}}
}

// neverMatch returns a MockConditionEvaluator that never matches.
func neverMatch(kind approval.ConditionKind) *MockConditionEvaluator {
	return &MockConditionEvaluator{kind: kind, fn: func(context.Context, approval.Condition, *approval.EvaluationContext) (bool, error) {
		return false, nil
	}}
}

// subjectMatch returns a MockConditionEvaluator that matches when Subject equals the given value.
func subjectMatch(kind approval.ConditionKind, matchSubject string) *MockConditionEvaluator {
	return &MockConditionEvaluator{kind: kind, fn: func(_ context.Context, cond approval.Condition, _ *approval.EvaluationContext) (bool, error) {
		return cond.Subject == matchSubject, nil
	}}
}

func newRegistry(evaluators ...approval.ConditionEvaluator) *strategy.StrategyRegistry {
	return strategy.NewStrategyRegistry(nil, nil, evaluators)
}

func newProcessContext(branches []approval.ConditionBranch, registry *strategy.StrategyRegistry) *ProcessContext {
	return &ProcessContext{
		Instance: &approval.Instance{
			ApplicantID: "u1",
			FormData:    map[string]any{"amount": 1000},
		},
		Node:     &approval.FlowNode{Branches: branches},
		Registry: registry,
	}
}

// --- ConditionProcessor.Process ---

// TestConditionProcessor tests condition processor scenarios.
func TestConditionProcessor(t *testing.T) {
	processor := NewConditionProcessor()

	t.Run("NodeKind", func(t *testing.T) {
		assert.Equal(t, approval.NodeCondition, processor.NodeKind(), "Should return NodeCondition kind")
	})

	t.Run("NoBranches", func(t *testing.T) {
		pc := newProcessContext(nil, nil)
		_, err := processor.Process(context.Background(), pc)
		require.ErrorIs(t, err, ErrNoBranches, "Should return ErrNoBranches")
	})

	t.Run("FirstBranchMatch", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		result, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, NodeActionContinue, result.Action, "Should return Continue action")
		require.NotNil(t, result.BranchID, "Should set BranchID")
		assert.Equal(t, "b1", *result.BranchID, "Should return first branch ID")
	})

	t.Run("SecondBranchMatchWhenFirstFails", func(t *testing.T) {
		registry := newRegistry(subjectMatch("test", "yes"))
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test", Subject: "no"}}},
			}},
			{ID: "b2", Label: "Branch2", Priority: 2, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test", Subject: "yes"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		result, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, "b2", *result.BranchID, "Should return second branch ID")
	})

	t.Run("DefaultBranchFallback", func(t *testing.T) {
		registry := newRegistry(neverMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
			{ID: "default", Label: "Default", IsDefault: true},
		}
		pc := newProcessContext(branches, registry)

		result, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, "default", *result.BranchID, "Should return default branch ID")
	})

	t.Run("NoMatchNoDefault", func(t *testing.T) {
		registry := newRegistry(neverMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		_, err := processor.Process(context.Background(), pc)
		require.ErrorIs(t, err, ErrNoMatchingBranch, "Should return ErrNoMatchingBranch")
	})

	t.Run("PriorityOrdering", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "low", Label: "LowPriority", Priority: 10, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
			{ID: "high", Label: "HighPriority", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		result, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, "high", *result.BranchID, "Should match highest priority (lowest number) branch first")
	})

	t.Run("SortDoesNotMutateInput", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "low", Label: "LowPriority", Priority: 10, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
			{ID: "high", Label: "HighPriority", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		_, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, "low", pc.Node.Branches[0].ID, "Original branches should not be reordered")
		assert.Equal(t, "high", pc.Node.Branches[1].ID, "Original branches should preserve input order")
	})

	t.Run("DefaultSkippedWhenNonDefaultMatches", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		branches := []approval.ConditionBranch{
			{ID: "default", Label: "Default", IsDefault: true, Priority: 1},
			{ID: "b1", Label: "Branch1", Priority: 2, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		result, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Equal(t, "b1", *result.BranchID, "Should select non-default match over default")
	})

	t.Run("EvaluationError", func(t *testing.T) {
		evalErr := errors.New("eval failed")
		errEvaluator := &MockConditionEvaluator{
			kind: "err",
			fn: func(context.Context, approval.Condition, *approval.EvaluationContext) (bool, error) {
				return false, evalErr
			},
		}
		registry := newRegistry(errEvaluator)
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "ErrBranch", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "err"}}},
			}},
		}
		pc := newProcessContext(branches, registry)

		_, err := processor.Process(context.Background(), pc)
		require.ErrorIs(t, err, evalErr, "Should propagate evaluator error")
		assert.Contains(t, err.Error(), "ErrBranch", "Should include branch label in error")
	})

	t.Run("EvaluationContextWiring", func(t *testing.T) {
		deptID := "dept_sales"

		var captured *approval.EvaluationContext

		captureEvaluator := &MockConditionEvaluator{
			kind: "test",
			fn: func(_ context.Context, _ approval.Condition, ec *approval.EvaluationContext) (bool, error) {
				captured = ec

				return true, nil
			},
		}
		registry := newRegistry(captureEvaluator)
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := &ProcessContext{
			Instance: &approval.Instance{
				ApplicantID:     "user_42",
				ApplicantDeptID: &deptID,
				FormData:        map[string]any{"key": "value"},
			},
			Node:     &approval.FlowNode{Branches: branches},
			Registry: registry,
		}

		_, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		require.NotNil(t, captured, "Should have captured evaluation context")
		assert.Equal(t, "user_42", captured.ApplicantID, "Should pass ApplicantID")
		assert.Equal(t, &deptID, captured.ApplicantDeptID, "Should pass ApplicantDeptID pointer")
		assert.Equal(t, "value", captured.FormData.Get("key"), "Should pass FormData")
	})

	t.Run("NilApplicantDeptID", func(t *testing.T) {
		var captured *approval.EvaluationContext

		captureEvaluator := &MockConditionEvaluator{
			kind: "test",
			fn: func(_ context.Context, _ approval.Condition, ec *approval.EvaluationContext) (bool, error) {
				captured = ec

				return true, nil
			},
		}
		registry := newRegistry(captureEvaluator)
		branches := []approval.ConditionBranch{
			{ID: "b1", Label: "Branch1", Priority: 1, ConditionGroups: []approval.ConditionGroup{
				{Conditions: []approval.Condition{{Kind: "test"}}},
			}},
		}
		pc := &ProcessContext{
			Instance: &approval.Instance{ApplicantID: "u1"},
			Node:     &approval.FlowNode{Branches: branches},
			Registry: registry,
		}

		_, err := processor.Process(context.Background(), pc)
		require.NoError(t, err, "Should process without error")
		assert.Nil(t, captured.ApplicantDeptID, "Should pass nil ApplicantDeptID")
	})
}

// --- evaluateConditionGroups (OR logic between groups) ---

// TestEvaluateConditionGroups tests OR logic between condition groups.
func TestEvaluateConditionGroups(t *testing.T) {
	t.Run("EmptyGroups", func(t *testing.T) {
		result, err := evaluateConditionGroups(nil, context.Background(), nil, nil)
		require.NoError(t, err, "Should not error on empty groups")
		assert.True(t, result, "Should return true for empty groups")
	})

	t.Run("SingleGroupMatch", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		groups := []approval.ConditionGroup{
			{Conditions: []approval.Condition{{Kind: "test"}}},
		}

		result, err := evaluateConditionGroups(registry, context.Background(), &approval.EvaluationContext{}, groups)
		require.NoError(t, err, "Should not error")
		assert.True(t, result, "Should return true when single group matches")
	})

	t.Run("ORLogicSecondGroupMatches", func(t *testing.T) {
		registry := newRegistry(subjectMatch("test", "yes"))
		groups := []approval.ConditionGroup{
			{Conditions: []approval.Condition{{Kind: "test", Subject: "no"}}},
			{Conditions: []approval.Condition{{Kind: "test", Subject: "yes"}}},
		}

		result, err := evaluateConditionGroups(registry, context.Background(), &approval.EvaluationContext{}, groups)
		require.NoError(t, err, "Should not error")
		assert.True(t, result, "Should return true when second group matches (OR)")
	})

	t.Run("AllGroupsFail", func(t *testing.T) {
		registry := newRegistry(neverMatch("test"))
		groups := []approval.ConditionGroup{
			{Conditions: []approval.Condition{{Kind: "test"}}},
			{Conditions: []approval.Condition{{Kind: "test"}}},
		}

		result, err := evaluateConditionGroups(registry, context.Background(), &approval.EvaluationContext{}, groups)
		require.NoError(t, err, "Should not error")
		assert.False(t, result, "Should return false when all groups fail")
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		evalErr := errors.New("group eval failed")
		registry := newRegistry(&MockConditionEvaluator{
			kind: "err",
			fn: func(context.Context, approval.Condition, *approval.EvaluationContext) (bool, error) {
				return false, evalErr
			},
		})
		groups := []approval.ConditionGroup{
			{Conditions: []approval.Condition{{Kind: "err"}}},
		}

		_, err := evaluateConditionGroups(registry, context.Background(), &approval.EvaluationContext{}, groups)
		require.ErrorIs(t, err, evalErr, "Should propagate evaluation error")
	})
}

// --- evaluateGroupConditions (AND logic within a group) ---

// TestEvaluateGroupConditions tests AND logic within a condition group.
func TestEvaluateGroupConditions(t *testing.T) {
	t.Run("EmptyConditions", func(t *testing.T) {
		result, err := evaluateGroupConditions(nil, context.Background(), nil, nil)
		require.NoError(t, err, "Should not error on empty conditions")
		assert.True(t, result, "Should return true for empty conditions")
	})

	t.Run("AllConditionsMatch", func(t *testing.T) {
		registry := newRegistry(alwaysMatch("test"))
		conditions := []approval.Condition{{Kind: "test"}, {Kind: "test"}}

		result, err := evaluateGroupConditions(registry, context.Background(), &approval.EvaluationContext{}, conditions)
		require.NoError(t, err, "Should not error")
		assert.True(t, result, "Should return true when all conditions match")
	})

	t.Run("ANDLogicOneFails", func(t *testing.T) {
		registry := newRegistry(subjectMatch("test", "yes"))
		conditions := []approval.Condition{
			{Kind: "test", Subject: "yes"},
			{Kind: "test", Subject: "no"},
		}

		result, err := evaluateGroupConditions(registry, context.Background(), &approval.EvaluationContext{}, conditions)
		require.NoError(t, err, "Should not error")
		assert.False(t, result, "Should return false when one condition fails (AND)")
	})

	t.Run("UnknownEvaluatorKind", func(t *testing.T) {
		registry := newRegistry() // empty registry
		conditions := []approval.Condition{{Kind: "unknown"}}

		_, err := evaluateGroupConditions(registry, context.Background(), &approval.EvaluationContext{}, conditions)
		require.Error(t, err, "Should error on unknown evaluator kind")
		assert.Contains(t, err.Error(), "unknown", "Should mention the unknown kind")
	})

	t.Run("EvaluatorError", func(t *testing.T) {
		evalErr := errors.New("evaluator failed")
		registry := newRegistry(&MockConditionEvaluator{
			kind: "err",
			fn: func(context.Context, approval.Condition, *approval.EvaluationContext) (bool, error) {
				return false, evalErr
			},
		})
		conditions := []approval.Condition{{Kind: "err"}}

		_, err := evaluateGroupConditions(registry, context.Background(), &approval.EvaluationContext{}, conditions)
		require.ErrorIs(t, err, evalErr, "Should propagate evaluator error")
	})

	// Integration subtests with real field and expression evaluators
	realRegistry := strategy.NewStrategyRegistry(
		nil,
		nil,
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
			strategy.NewExpressionConditionEvaluator(),
		},
	)

	t.Run("FieldCondition", func(t *testing.T) {
		t.Run("SingleMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: float64(500)},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{"amount": float64(1000)}), ApplicantID: "u1"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid field condition")
			assert.True(t, match, "Should match when amount 1000 > 500")
		})

		t.Run("SingleNoMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: float64(2000)},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{"amount": float64(1000)}), ApplicantID: "u1"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid field condition")
			assert.False(t, match, "Should not match when amount 1000 is not > 2000")
		})

		t.Run("AllMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "amount", Operator: "gte", Value: float64(100)},
				{Kind: approval.ConditionField, Subject: "category", Operator: "eq", Value: "travel"},
			}
			evalCtx := &approval.EvaluationContext{
				FormData:    approval.NewFormData(map[string]any{"amount": float64(500), "category": "travel"}),
				ApplicantID: "u1",
			}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error when all conditions are valid")
			assert.True(t, match, "Should match when all AND conditions are satisfied")
		})

		t.Run("OneFails", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "amount", Operator: "gte", Value: float64(100)},
				{Kind: approval.ConditionField, Subject: "category", Operator: "eq", Value: "travel"},
			}
			evalCtx := &approval.EvaluationContext{
				FormData:    approval.NewFormData(map[string]any{"amount": float64(500), "category": "purchase"}),
				ApplicantID: "u1",
			}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid conditions")
			assert.False(t, match, "Should not match when one AND condition fails")
		})

		t.Run("ApplicantSubject", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "applicantId", Operator: "eq", Value: "user_42"},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "user_42"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for applicant field condition")
			assert.True(t, match, "Should match when applicant ID equals expected value")
		})

		t.Run("DeptSubjectMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "applicantDeptId", Operator: "eq", Value: "dept_001"},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "u1", ApplicantDeptID: new("dept_001")}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for dept condition")
			assert.True(t, match, "Should match when dept ID equals expected value")
		})

		t.Run("DeptSubjectNoMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "applicantDeptId", Operator: "eq", Value: "dept_999"},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "u1", ApplicantDeptID: new("dept_001")}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for dept condition")
			assert.False(t, match, "Should not match when dept ID differs from expected value")
		})

		t.Run("Operators", func(t *testing.T) {
			tests := []struct {
				name     string
				formData map[string]any
				subject  string
				operator string
				value    any
			}{
				{"LessThan", map[string]any{"amount": float64(100)}, "amount", "lt", float64(200)},
				{"LessThanOrEqual", map[string]any{"amount": float64(200)}, "amount", "lte", float64(200)},
				{"NotEqual", map[string]any{"amount": float64(100)}, "amount", "ne", float64(200)},
				{"Equal", map[string]any{"amount": float64(100)}, "amount", "eq", float64(100)},
				{"ContainsString", map[string]any{"name": "hello world"}, "name", "contains", "world"},
				{"NotContainsString", map[string]any{"name": "hello world"}, "name", "not_contains", "foo"},
				{"StartsWithString", map[string]any{"name": "hello world"}, "name", "starts_with", "hello"},
				{"EndsWithString", map[string]any{"name": "hello world"}, "name", "ends_with", "world"},
				{"IsEmpty", map[string]any{"name": ""}, "name", "is_empty", nil},
				{"IsNotEmpty", map[string]any{"name": "hello"}, "name", "is_not_empty", nil},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					conditions := []approval.Condition{
						{Kind: approval.ConditionField, Subject: tt.subject, Operator: tt.operator, Value: tt.value},
					}
					evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(tt.formData), ApplicantID: "u1"}

					match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
					require.NoError(t, err, "Should not return error for valid condition")
					assert.True(t, match, "Should match for operator %s", tt.operator)
				})
			}
		})
	})

	t.Run("ExpressionCondition", func(t *testing.T) {
		t.Run("Match", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `formData["amount"] > 500`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{"amount": float64(1000)}), ApplicantID: "u1"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid expression")
			assert.True(t, match, "Should match when expression evaluates to true")
		})

		t.Run("NoMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `formData["amount"] > 5000`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{"amount": float64(100)}), ApplicantID: "u1"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid expression")
			assert.False(t, match, "Should not match when expression evaluates to false")
		})

		t.Run("ApplicantContext", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `applicantId == "admin_user"`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "admin_user"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for applicant expression")
			assert.True(t, match, "Should match when applicant equals expected value")
		})

		t.Run("DeptContext", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `applicantDeptId == "finance"`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "u1", ApplicantDeptID: new("finance")}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for dept expression")
			assert.True(t, match, "Should match when dept equals expected value in expression")
		})

		t.Run("ComplexExpression", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `formData["amount"] > 100 && applicantId != "admin"`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(map[string]any{"amount": float64(500)}), ApplicantID: "user1"}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for complex expression")
			assert.True(t, match, "Should match for complex boolean expression")
		})

		t.Run("CompileError", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionExpression, Expression: `formData[`},
			}
			evalCtx := &approval.EvaluationContext{FormData: approval.NewFormData(nil), ApplicantID: "u1"}

			_, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			assert.Error(t, err, "Should return error for invalid expression syntax")
		})
	})

	t.Run("MixedConditions", func(t *testing.T) {
		t.Run("AllMatch", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "status", Operator: "eq", Value: "active"},
				{Kind: approval.ConditionExpression, Expression: `formData["priority"] == "high"`},
			}
			evalCtx := &approval.EvaluationContext{
				FormData:    approval.NewFormData(map[string]any{"status": "active", "priority": "high"}),
				ApplicantID: "u1",
			}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for mixed conditions")
			assert.True(t, match, "Should match when all mixed conditions are satisfied")
		})

		t.Run("OneFails", func(t *testing.T) {
			conditions := []approval.Condition{
				{Kind: approval.ConditionField, Subject: "status", Operator: "eq", Value: "active"},
				{Kind: approval.ConditionExpression, Expression: `formData["priority"] == "low"`},
			}
			evalCtx := &approval.EvaluationContext{
				FormData:    approval.NewFormData(map[string]any{"status": "active", "priority": "high"}),
				ApplicantID: "u1",
			}

			match, err := evaluateGroupConditions(realRegistry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error when expression simply does not match")
			assert.False(t, match, "Should not match when expression condition fails in AND group")
		})
	})
}
