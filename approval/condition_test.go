package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestAggregateKindIsValid(t *testing.T) {
	valid := []approval.AggregateKind{approval.AggregateSum, approval.AggregateCount, approval.AggregateAvg}
	for _, kind := range valid {
		assert.True(t, kind.IsValid(), "built-in aggregate %q should be valid", kind)
	}

	assert.False(t, approval.AggregateKind("median").IsValid(), "IsValid covers the built-in vocabulary only")
	assert.False(t, approval.AggregateKind("").IsValid(), "the empty kind means no aggregate")
}

func TestAggregateKindFoldsColumn(t *testing.T) {
	assert.True(t, approval.AggregateSum.FoldsColumn(), "sum folds a column")
	assert.True(t, approval.AggregateAvg.FoldsColumn(), "avg folds a column")
	assert.False(t, approval.AggregateCount.FoldsColumn(), "count folds rows")
	assert.True(t, approval.AggregateKind("median").FoldsColumn(),
		"unknown kinds default to column-folding — the safe shape for host aggregates")
}

func TestConditionOperatorsForField(t *testing.T) {
	cases := []struct {
		name  string
		field approval.FormFieldDefinition
		want  []approval.ConditionOperator
	}{
		{
			name:  "Input",
			field: approval.FormFieldDefinition{Kind: approval.FieldInput},
			want: []approval.ConditionOperator{
				approval.OperatorEquals, approval.OperatorNotEquals,
				approval.OperatorContains, approval.OperatorNotContains, approval.OperatorStartsWith, approval.OperatorEndsWith,
				approval.OperatorIn, approval.OperatorNotIn,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "Textarea",
			field: approval.FormFieldDefinition{Kind: approval.FieldTextarea},
			want: []approval.ConditionOperator{
				approval.OperatorEquals, approval.OperatorNotEquals,
				approval.OperatorContains, approval.OperatorNotContains, approval.OperatorStartsWith, approval.OperatorEndsWith,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "Number",
			field: approval.FormFieldDefinition{Kind: approval.FieldNumber},
			want: []approval.ConditionOperator{
				approval.OperatorEquals, approval.OperatorNotEquals,
				approval.OperatorGreater, approval.OperatorGreaterOrEq, approval.OperatorLess, approval.OperatorLessOrEq,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "Date",
			field: approval.FormFieldDefinition{Kind: approval.FieldDate},
			want: []approval.ConditionOperator{
				approval.OperatorEquals, approval.OperatorNotEquals,
				approval.OperatorGreater, approval.OperatorGreaterOrEq, approval.OperatorLess, approval.OperatorLessOrEq,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "SingleSelect",
			field: approval.FormFieldDefinition{Kind: approval.FieldSelect},
			want: []approval.ConditionOperator{
				approval.OperatorEquals, approval.OperatorNotEquals,
				approval.OperatorIn, approval.OperatorNotIn,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "MultiSelect",
			field: approval.FormFieldDefinition{Kind: approval.FieldSelect, IsMultiple: true},
			want: []approval.ConditionOperator{
				approval.OperatorContains, approval.OperatorNotContains,
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "Upload",
			field: approval.FormFieldDefinition{Kind: approval.FieldUpload},
			want: []approval.ConditionOperator{
				approval.OperatorIsEmpty, approval.OperatorIsNotEmpty,
			},
		},
		{
			name:  "Table",
			field: approval.FormFieldDefinition{Kind: approval.FieldTable},
			want:  nil,
		},
		{
			name:  "UnknownKind",
			field: approval.FormFieldDefinition{Kind: approval.FieldKind("unknown")},
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := approval.ConditionOperatorsForField(tc.field)
			assert.Equal(t, tc.want, got, "operators for %s must match the contract", tc.name)

			for _, op := range tc.want {
				assert.True(t, approval.IsOperatorValidForField(tc.field, op),
					"operator %q must be accepted for %s", op, tc.name)
			}
		})
	}

	t.Run("RejectsMismatchedOperators", func(t *testing.T) {
		multi := approval.FormFieldDefinition{Kind: approval.FieldSelect, IsMultiple: true}
		for _, op := range []approval.ConditionOperator{
			approval.OperatorEquals, approval.OperatorNotEquals,
			approval.OperatorIn, approval.OperatorNotIn,
			approval.OperatorGreater,
		} {
			assert.False(t, approval.IsOperatorValidForField(multi, op),
				"multi-select must reject %q", op)
		}

		single := approval.FormFieldDefinition{Kind: approval.FieldSelect}
		assert.False(t, approval.IsOperatorValidForField(single, approval.OperatorContains),
			"single-select must reject contains")
		assert.False(t, approval.IsOperatorValidForField(single, approval.OperatorGreater),
			"single-select must reject ordering operators")
	})

	t.Run("ReturnsDefensiveCopy", func(t *testing.T) {
		multi := approval.FormFieldDefinition{Kind: approval.FieldSelect, IsMultiple: true}
		ops := approval.ConditionOperatorsForField(multi)
		ops[0] = approval.OperatorEquals

		assert.Equal(t, approval.OperatorContains, approval.ConditionOperatorsForField(multi)[0],
			"mutating the returned slice must not alter the shared contract table")
	})
}
