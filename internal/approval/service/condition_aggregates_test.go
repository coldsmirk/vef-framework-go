package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateConditionAggregates(t *testing.T) {
	svc := NewFlowDefinitionService()

	form := []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber},
		{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{
			{Key: "qty", Kind: approval.FieldNumber},
			{Key: "name", Kind: approval.FieldInput},
		}},
	}

	nodes := func(cond approval.Condition) map[string]approval.NodeData {
		return map[string]approval.NodeData{
			"cond-1": &approval.ConditionNodeData{Branches: []approval.ConditionBranch{
				{ID: "b1", Priority: 1, ConditionGroups: []approval.ConditionGroup{{Conditions: []approval.Condition{cond}}}},
				{ID: "bd", Priority: 99, IsDefault: true},
			}},
		}
	}

	t.Run("AcceptsSumOverNumberColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateSum, Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.NoError(t, svc.ValidateConditionAggregates(nodes(cond), form), "sum over a number column is the canonical aggregate")
	})

	t.Run("AcceptsCountWithoutColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateCount, Operator: approval.OperatorGreaterOrEq, Value: 1}
		assert.NoError(t, svc.ValidateConditionAggregates(nodes(cond), form), "count folds rows and needs no column")
	})

	t.Run("RejectsNonTableSubject", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "amount", Aggregate: approval.AggregateSum, Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionAggregates(nodes(cond), form), errAggregateSubjectNotTable,
			"aggregates fold detail tables, not scalar fields")
	})

	t.Run("RejectsUnknownColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateAvg, Column: "price", Operator: approval.OperatorLess, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionAggregates(nodes(cond), form), errAggregateColumnUnknown,
			"the folded column must exist in the table")
	})

	t.Run("RejectsNonNumericColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateSum, Column: "name", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionAggregates(nodes(cond), form), errAggregateColumnNotNumeric,
			"sum/avg fold numbers; a text column must be rejected at deploy")
	})

	t.Run("RejectsUnregisteredKind", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: "median", Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionAggregates(nodes(cond), form), errUnregisteredAggregate,
			"an aggregate kind with no registered aggregator must fail at deploy")
	})

	t.Run("AcceptsHostRegisteredKind", func(t *testing.T) {
		// The open-closed contract: registering a new kind makes it
		// deployable with zero framework changes.
		extended := NewFlowDefinitionService(approval.AggregateSum, approval.AggregateCount, approval.AggregateAvg, "median")
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: "median", Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.NoError(t, extended.ValidateConditionAggregates(nodes(cond), form),
			"a boot-registered custom aggregate deploys like a built-in")
	})

	t.Run("IgnoresScalarAndExpressionConditions", func(t *testing.T) {
		scalar := approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: approval.OperatorGreater, Value: 1}
		assert.NoError(t, svc.ValidateConditionAggregates(nodes(scalar), form), "non-aggregate conditions are out of scope here")
	})
}
