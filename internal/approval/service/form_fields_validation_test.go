package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateFormFields(t *testing.T) {
	svc := NewFlowDefinitionService()

	field := func(key string, kind approval.FieldKind) approval.FormFieldDefinition {
		return approval.FormFieldDefinition{Key: key, Kind: kind, Label: key}
	}

	t.Run("AcceptsNilFields", func(t *testing.T) {
		assert.NoError(t, svc.ValidateFormFields(nil), "A flow without a form is valid")
	})

	t.Run("AcceptsValidFields", func(t *testing.T) {
		minLen, maxLen := 1, 100
		minVal, maxVal := 0.0, 10.0
		fields := []approval.FormFieldDefinition{
			{
				Key: "title", Kind: approval.FieldInput, Label: "标题",
				Validation: &approval.ValidationRule{MinLength: &minLen, MaxLength: &maxLen, Pattern: `^\w+$`},
			},
			{
				Key: "amount", Kind: approval.FieldNumber, Label: "金额",
				Validation: &approval.ValidationRule{Min: &minVal, Max: &maxVal},
			},
		}

		assert.NoError(t, svc.ValidateFormFields(fields), "A well-formed field list should pass")
	})

	t.Run("RejectsEmptyKey", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{field("", approval.FieldInput)}
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errFormFieldKeyEmpty, "A blank field key should be rejected")
	})

	t.Run("RejectsWhitespaceKey", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{field("   ", approval.FieldInput)}
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errFormFieldKeyEmpty,
			"a whitespace-only key sanitizes to an empty identifier at publish and must be rejected at deploy")
	})

	t.Run("RejectsDuplicateKey", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{
			field("amount", approval.FieldNumber),
			field("amount", approval.FieldInput),
		}
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errDuplicateFormFieldKey, "Duplicate field keys should be rejected")
	})

	t.Run("RejectsUnknownKind", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{field("x", approval.FieldKind("matrix"))}
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errInvalidFormFieldKind, "An unknown field kind should be rejected")
	})

	t.Run("RejectsUncompilablePattern", func(t *testing.T) {
		f := field("title", approval.FieldInput)
		f.Validation = &approval.ValidationRule{Pattern: "(unclosed"}

		assert.ErrorIs(t, svc.ValidateFormFields([]approval.FormFieldDefinition{f}), errInvalidFormPattern,
			"A pattern that does not compile must fail at deploy, not at submission")
	})

	t.Run("RejectsInvertedLengthBounds", func(t *testing.T) {
		minLen, maxLen := 10, 2
		f := field("title", approval.FieldInput)
		f.Validation = &approval.ValidationRule{MinLength: &minLen, MaxLength: &maxLen}

		assert.ErrorIs(t, svc.ValidateFormFields([]approval.FormFieldDefinition{f}), errInvalidFormLengthRange,
			"minLength > maxLength is unsatisfiable")
	})

	t.Run("RejectsInvertedValueBounds", func(t *testing.T) {
		minVal, maxVal := 10.0, 2.0
		f := field("amount", approval.FieldNumber)
		f.Validation = &approval.ValidationRule{Min: &minVal, Max: &maxVal}

		assert.ErrorIs(t, svc.ValidateFormFields([]approval.FormFieldDefinition{f}), errInvalidFormValueRange,
			"min > max is unsatisfiable")
	})
}

func TestValidateFormFieldsTableColumns(t *testing.T) {
	svc := new(FlowDefinitionService)

	table := func(columns ...approval.FormFieldDefinition) []approval.FormFieldDefinition {
		return []approval.FormFieldDefinition{
			{Key: "items", Kind: approval.FieldTable, Label: "Items", Columns: columns},
		}
	}

	t.Run("AcceptsValidTable", func(t *testing.T) {
		fields := table(
			approval.FormFieldDefinition{Key: "name", Kind: approval.FieldInput},
			approval.FormFieldDefinition{Key: "qty", Kind: approval.FieldNumber},
		)
		assert.NoError(t, svc.ValidateFormFields(fields), "a table with valid columns should deploy")
	})

	t.Run("RejectsTableWithoutColumns", func(t *testing.T) {
		assert.ErrorIs(t, svc.ValidateFormFields(table()), errTableColumnsRequired,
			"a table field with no columns has no row shape and must be rejected")
	})

	t.Run("RejectsNestedTable", func(t *testing.T) {
		fields := table(approval.FormFieldDefinition{Key: "sub", Kind: approval.FieldTable})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errNestedTableColumn,
			"detail tables are single-level by design")
	})

	t.Run("RejectsDuplicateColumnKeys", func(t *testing.T) {
		fields := table(
			approval.FormFieldDefinition{Key: "name", Kind: approval.FieldInput},
			approval.FormFieldDefinition{Key: "name", Kind: approval.FieldInput},
		)
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errDuplicateFormFieldKey,
			"column keys must be unique within their table")
	})

	t.Run("RejectsColumnsOnScalarField", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldInput, Columns: []approval.FormFieldDefinition{{Key: "x", Kind: approval.FieldInput}}},
		}
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errColumnsOnScalarField,
			"columns on a scalar field hide a designer bug and must be rejected")
	})

	t.Run("RejectsWhitespaceColumnKey", func(t *testing.T) {
		fields := table(approval.FormFieldDefinition{Key: "  ", Kind: approval.FieldInput})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errFormFieldKeyEmpty,
			"a whitespace-only column key sanitizes to an empty identifier at publish and must be rejected at deploy")
	})
}

func TestValidateFormFieldsOptionSource(t *testing.T) {
	svc := new(FlowDefinitionService)

	selectField := func(source *approval.FieldOptionSource) []approval.FormFieldDefinition {
		return []approval.FormFieldDefinition{
			{Key: "city", Kind: approval.FieldSelect, Label: "城市", OptionSource: source},
		}
	}

	t.Run("AcceptsAbsentSource", func(t *testing.T) {
		assert.NoError(t, svc.ValidateFormFields(selectField(nil)),
			"a select with no unresolved source is a static or free-form field and must deploy")
	})

	t.Run("AcceptsCompleteRemoteSource", func(t *testing.T) {
		fields := selectField(&approval.FieldOptionSource{
			Kind:    approval.OptionSourceRemote,
			Request: &approval.RemoteOptionRequest{Resource: "city", Action: "list"},
		})
		assert.NoError(t, svc.ValidateFormFields(fields), "a remote source naming an operation must deploy")
	})

	t.Run("RejectsUnknownKind", func(t *testing.T) {
		fields := selectField(&approval.FieldOptionSource{
			Kind:    approval.OptionSourceKind("graphql"),
			Request: &approval.RemoteOptionRequest{Resource: "city", Action: "list"},
		})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errInvalidOptionSourceKind,
			"a kind no consumer knows how to resolve must be rejected at deploy")
	})

	t.Run("RejectsMissingRequest", func(t *testing.T) {
		fields := selectField(&approval.FieldOptionSource{Kind: approval.OptionSourceRemote})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errRemoteOptionRequestIncomplete,
			"a remote source naming no operation resolves nowhere and must be rejected at deploy")
	})

	t.Run("RejectsBlankResource", func(t *testing.T) {
		fields := selectField(&approval.FieldOptionSource{
			Kind:    approval.OptionSourceRemote,
			Request: &approval.RemoteOptionRequest{Resource: "  ", Action: "list"},
		})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errRemoteOptionRequestIncomplete,
			"a whitespace-only resource addresses nothing and must be rejected at deploy")
	})

	t.Run("RejectsBlankAction", func(t *testing.T) {
		fields := selectField(&approval.FieldOptionSource{
			Kind:    approval.OptionSourceRemote,
			Request: &approval.RemoteOptionRequest{Resource: "city", Action: ""},
		})
		assert.ErrorIs(t, svc.ValidateFormFields(fields), errRemoteOptionRequestIncomplete,
			"a remote source without an action addresses nothing and must be rejected at deploy")
	})

	t.Run("RejectsBrokenSourceOnTableColumn", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{{
			Key: "items", Kind: approval.FieldTable, Label: "明细",
			Columns: []approval.FormFieldDefinition{{
				Key: "unit", Kind: approval.FieldSelect,
				OptionSource: &approval.FieldOptionSource{Kind: approval.OptionSourceRemote},
			}},
		}}

		err := svc.ValidateFormFields(fields)

		assert.ErrorIs(t, err, errRemoteOptionRequestIncomplete,
			"a column's option source is validated exactly like a root field's")
		assert.Contains(t, err.Error(), "items.unit",
			"the message must name the table-qualified column path so the designer can find it")
	})
}
