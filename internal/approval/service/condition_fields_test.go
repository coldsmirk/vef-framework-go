package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateConditionFields(t *testing.T) {
	svc := NewFlowDefinitionService()

	form := []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber},
		{Key: "category", Kind: approval.FieldSelect},
		{Key: "tags", Kind: approval.FieldSelect, IsMultiple: true},
		{Key: "attachment", Kind: approval.FieldUpload},
		// Colliding with a built-in applicant subject: runtime resolves
		// applicantId from the evaluation context before form data, so deploy
		// validation must treat it as an applicant subject, not this field.
		{Key: approval.SubjectApplicantID, Kind: approval.FieldSelect, IsMultiple: true},
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
		assert.NoError(t, svc.ValidateConditionFields(nodes(cond), form), "sum over a number column is the canonical aggregate")
	})

	t.Run("AcceptsCountWithoutColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateCount, Operator: approval.OperatorGreaterOrEq, Value: 1}
		assert.NoError(t, svc.ValidateConditionFields(nodes(cond), form), "count folds rows and needs no column")
	})

	t.Run("RejectsNonTableSubject", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "amount", Aggregate: approval.AggregateSum, Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), errAggregateSubjectNotTable,
			"aggregates fold detail tables, not scalar fields")
	})

	t.Run("RejectsUnknownColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateAvg, Column: "price", Operator: approval.OperatorLess, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), errAggregateColumnUnknown,
			"the folded column must exist in the table")
	})

	t.Run("RejectsNonNumericColumn", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateSum, Column: "name", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), errAggregateColumnNotNumeric,
			"sum/avg fold numbers; a text column must be rejected at deploy")
	})

	t.Run("RejectsUnregisteredKind", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: "median", Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), errUnregisteredAggregate,
			"an aggregate kind with no registered aggregator must fail at deploy")
	})

	t.Run("AcceptsHostRegisteredKind", func(t *testing.T) {
		// The open-closed contract: registering a new kind makes it
		// deployable with zero framework changes.
		extended := NewFlowDefinitionService(WithAggregateKinds(approval.AggregateSum, approval.AggregateCount, approval.AggregateAvg, "median"))
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: "median", Column: "qty", Operator: approval.OperatorGreater, Value: 1}
		assert.NoError(t, extended.ValidateConditionFields(nodes(cond), form),
			"a boot-registered custom aggregate deploys like a built-in")
	})

	t.Run("AcceptsValidScalarCondition", func(t *testing.T) {
		scalar := approval.Condition{Kind: approval.ConditionField, Subject: "amount", Operator: approval.OperatorGreater, Value: 1}
		assert.NoError(t, svc.ValidateConditionFields(nodes(scalar), form), "numeric field accepts ordering operator")
	})

	t.Run("AcceptsMultiSelectOperators", func(t *testing.T) {
		for _, op := range []approval.ConditionOperator{
			approval.OperatorContains,
			approval.OperatorNotContains,
			approval.OperatorIsEmpty,
			approval.OperatorIsNotEmpty,
		} {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "tags", Operator: op, Value: "a"}
			assert.NoError(t, svc.ValidateConditionFields(nodes(cond), form), "multi-select field must accept %q", op)
		}
	})

	t.Run("RejectsScalarEqualityOnMultiSelect", func(t *testing.T) {
		for _, op := range []approval.ConditionOperator{
			approval.OperatorEquals,
			approval.OperatorNotEquals,
			approval.OperatorIn,
			approval.OperatorNotIn,
		} {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "tags", Operator: op, Value: "a"}
			assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), approval.ErrInvalidConditionOperator,
				"multi-select field must reject %q", op)
		}
	})

	t.Run("RejectsContainsOnSingleSelect", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "category", Operator: approval.OperatorContains, Value: "a"}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), approval.ErrInvalidConditionOperator,
			"single-select field must reject contains")
	})

	t.Run("RejectsEqualsOnUpload", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "attachment", Operator: approval.OperatorEquals, Value: "file.pdf"}
		assert.ErrorIs(t, svc.ValidateConditionFields(nodes(cond), form), approval.ErrInvalidConditionOperator,
			"upload field only supports emptiness operators")
	})

	t.Run("SkipsApplicantSubjects", func(t *testing.T) {
		for _, subject := range []string{approval.SubjectApplicantID, approval.SubjectApplicantDepartmentID} {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: subject, Operator: approval.OperatorEquals, Value: "u1"}
			assert.NoError(t, svc.ValidateConditionFields(nodes(cond), form),
				"applicant subject %q must bypass form-field operator checks", subject)
		}
	})

	t.Run("SkipsHostGlobalSubjects", func(t *testing.T) {
		cond := approval.Condition{Kind: approval.ConditionField, Subject: "tenantTier", Operator: approval.OperatorEquals, Value: "vip"}
		assert.NoError(t, svc.ValidateConditionFields(nodes(cond), form),
			"subjects absent from form fields (such as host globals) must not be checked against form_fields")
	})

	t.Run("IgnoresExpressionConditions", func(t *testing.T) {
		expr := approval.Condition{Kind: approval.ConditionExpression, Expression: "formData.amount > 1"}
		assert.NoError(t, svc.ValidateConditionFields(nodes(expr), form), "expression conditions are not field conditions")
	})
}
