package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateFormDefinition(t *testing.T) {
	svc := NewFlowDefinitionService()

	field := func(key string, kind approval.FieldKind) approval.FormFieldDefinition {
		return approval.FormFieldDefinition{Key: key, Kind: kind, Label: key}
	}

	t.Run("AcceptsNilDefinition", func(t *testing.T) {
		assert.NoError(t, svc.ValidateFormDefinition(nil), "A flow without a form schema is valid")
	})

	t.Run("AcceptsValidSchema", func(t *testing.T) {
		minLen, maxLen := 1, 100
		minVal, maxVal := 0.0, 10.0
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{
			{
				Key: "title", Kind: approval.FieldInput, Label: "标题",
				Validation: &approval.ValidationRule{MinLength: &minLen, MaxLength: &maxLen, Pattern: `^\w+$`},
			},
			{
				Key: "amount", Kind: approval.FieldNumber, Label: "金额",
				Validation: &approval.ValidationRule{Min: &minVal, Max: &maxVal},
			},
		}}

		assert.NoError(t, svc.ValidateFormDefinition(def), "A well-formed schema should pass")
	})

	t.Run("RejectsEmptyKey", func(t *testing.T) {
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{field("", approval.FieldInput)}}
		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errFormFieldKeyEmpty, "A blank field key should be rejected")
	})

	t.Run("RejectsDuplicateKey", func(t *testing.T) {
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{
			field("amount", approval.FieldNumber),
			field("amount", approval.FieldInput),
		}}
		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errDuplicateFormFieldKey, "Duplicate field keys should be rejected")
	})

	t.Run("RejectsUnknownKind", func(t *testing.T) {
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{field("x", approval.FieldKind("matrix"))}}
		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errInvalidFormFieldKind, "An unknown field kind should be rejected")
	})

	t.Run("RejectsUncompilablePattern", func(t *testing.T) {
		f := field("title", approval.FieldInput)
		f.Validation = &approval.ValidationRule{Pattern: "(unclosed"}
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{f}}

		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errInvalidFormPattern,
			"A pattern that does not compile must fail at deploy, not at submission")
	})

	t.Run("RejectsInvertedLengthBounds", func(t *testing.T) {
		minLen, maxLen := 10, 2
		f := field("title", approval.FieldInput)
		f.Validation = &approval.ValidationRule{MinLength: &minLen, MaxLength: &maxLen}
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{f}}

		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errInvalidFormLengthRange, "minLength > maxLength is unsatisfiable")
	})

	t.Run("RejectsInvertedValueBounds", func(t *testing.T) {
		minVal, maxVal := 10.0, 2.0
		f := field("amount", approval.FieldNumber)
		f.Validation = &approval.ValidationRule{Min: &minVal, Max: &maxVal}
		def := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{f}}

		assert.ErrorIs(t, svc.ValidateFormDefinition(def), errInvalidFormValueRange, "min > max is unsatisfiable")
	})
}
