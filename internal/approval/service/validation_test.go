package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// --- ValidateOpinion ---

func TestValidateOpinion(t *testing.T) {
	svc := NewValidationService(nil)

	t.Run("RequiredAndProvided", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "looks good")
		assert.NoError(t, err, "Should pass when opinion is required and provided")
	})

	t.Run("RequiredButEmpty", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "")
		assert.ErrorIs(t, err, shared.ErrOpinionRequired, "Should fail when opinion is required but empty")
	})

	t.Run("RequiredButWhitespaceOnly", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "   \t")
		assert.ErrorIs(t, err, shared.ErrOpinionRequired, "Should fail when opinion is only whitespace")
	})

	t.Run("NotRequiredAndEmpty", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: false}
		err := svc.ValidateOpinion(node, "")
		assert.NoError(t, err, "Should pass when opinion is not required")
	})

	t.Run("NotRequiredAndProvided", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: false}
		err := svc.ValidateOpinion(node, "optional opinion")
		assert.NoError(t, err, "Should pass when opinion is optional and provided")
	})
}

// --- FilterEditableFormData ---

func TestFilterEditableFormData(t *testing.T) {
	t.Run("NilPermissions", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": 30}
		result := FilterEditableFormData(data, nil)
		assert.Empty(t, result, "Should deny all fields when permissions are nil (least privilege)")
	})

	t.Run("EmptyPermissions", func(t *testing.T) {
		data := map[string]any{"name": "Alice"}
		result := FilterEditableFormData(data, map[string]approval.Permission{})
		assert.Empty(t, result, "Should deny all fields when permissions are empty (least privilege)")
	})

	t.Run("EditableFields", func(t *testing.T) {
		data := map[string]any{"name": "Alice", "age": 30, "dept": "IT"}
		perms := map[string]approval.Permission{
			"name": approval.PermissionEditable,
			"age":  approval.PermissionVisible,
			"dept": approval.PermissionRequired,
		}
		result := FilterEditableFormData(data, perms)
		assert.Equal(t, map[string]any{"name": "Alice", "dept": "IT"}, result,
			"Should only include editable and required fields")
	})

	t.Run("HiddenFields", func(t *testing.T) {
		data := map[string]any{"secret": "123", "name": "Alice"}
		perms := map[string]approval.Permission{
			"secret": approval.PermissionHidden,
			"name":   approval.PermissionEditable,
		}
		result := FilterEditableFormData(data, perms)
		assert.Equal(t, map[string]any{"name": "Alice"}, result,
			"Should exclude hidden fields")
	})

	t.Run("FieldNotInPermissions", func(t *testing.T) {
		data := map[string]any{"unknown": "val", "name": "Alice"}
		perms := map[string]approval.Permission{
			"name": approval.PermissionEditable,
		}
		result := FilterEditableFormData(data, perms)
		assert.Equal(t, map[string]any{"name": "Alice"}, result,
			"Should exclude fields not listed in permissions")
	})

	t.Run("EmptyFormData", func(t *testing.T) {
		perms := map[string]approval.Permission{"name": approval.PermissionEditable}
		result := FilterEditableFormData(nil, perms)
		assert.Empty(t, result, "Should return empty map for nil form data")
	})

	t.Run("AllVisible", func(t *testing.T) {
		data := map[string]any{"a": 1, "b": 2}
		perms := map[string]approval.Permission{
			"a": approval.PermissionVisible,
			"b": approval.PermissionVisible,
		}
		result := FilterEditableFormData(data, perms)
		assert.Empty(t, result, "Should return empty when all fields are visible-only")
	})
}

// --- MergeFormData ---

func TestMergeFormData(t *testing.T) {
	t.Run("NilFormData", func(t *testing.T) {
		instance := &approval.Instance{FormData: map[string]any{"existing": "val"}}
		MergeFormData(instance, nil, nil)
		assert.Equal(t, map[string]any{"existing": "val"}, instance.FormData,
			"Should not modify instance when form data is nil")
	})

	t.Run("EmptyFormData", func(t *testing.T) {
		instance := &approval.Instance{FormData: map[string]any{"existing": "val"}}
		MergeFormData(instance, map[string]any{}, nil)
		assert.Equal(t, map[string]any{"existing": "val"}, instance.FormData,
			"Should not modify instance when form data is empty")
	})

	t.Run("NilPermissionsDenyAll", func(t *testing.T) {
		instance := &approval.Instance{FormData: map[string]any{"existing": "val"}}
		MergeFormData(instance, map[string]any{"name": "Alice"}, nil)
		assert.Equal(t, map[string]any{"existing": "val"}, instance.FormData,
			"Should not modify instance when permissions are nil (least privilege)")
	})

	t.Run("MergeWithNilInstanceFormData", func(t *testing.T) {
		perms := map[string]approval.Permission{"name": approval.PermissionEditable}
		instance := &approval.Instance{}
		MergeFormData(instance, map[string]any{"name": "Alice"}, perms)
		assert.Equal(t, map[string]any{"name": "Alice"}, instance.FormData,
			"Should initialize FormData on instance")
	})

	t.Run("MergeOverwritesExisting", func(t *testing.T) {
		perms := map[string]approval.Permission{
			"name": approval.PermissionEditable,
			"dept": approval.PermissionEditable,
			"age":  approval.PermissionVisible,
		}
		instance := &approval.Instance{FormData: map[string]any{"name": "Bob", "age": 20}}
		MergeFormData(instance, map[string]any{"name": "Alice", "dept": "IT"}, perms)
		assert.Equal(t, "Alice", instance.FormData["name"], "Should overwrite existing editable field")
		assert.Equal(t, 20, instance.FormData["age"], "Should preserve visible-only field")
		assert.Equal(t, "IT", instance.FormData["dept"], "Should add new editable field")
	})

	t.Run("MergeWithPermissions", func(t *testing.T) {
		instance := &approval.Instance{FormData: map[string]any{"name": "Bob", "secret": "old"}}
		perms := map[string]approval.Permission{
			"name":   approval.PermissionEditable,
			"secret": approval.PermissionHidden,
		}
		MergeFormData(instance, map[string]any{"name": "Alice", "secret": "new"}, perms)
		assert.Equal(t, "Alice", instance.FormData["name"], "Should merge editable field")
		assert.Equal(t, "old", instance.FormData["secret"], "Should not merge hidden field")
	})

	t.Run("AllFilteredOut", func(t *testing.T) {
		instance := &approval.Instance{FormData: map[string]any{"name": "Bob"}}
		perms := map[string]approval.Permission{
			"name": approval.PermissionVisible,
		}
		MergeFormData(instance, map[string]any{"name": "Alice"}, perms)
		assert.Equal(t, "Bob", instance.FormData["name"],
			"Should not merge when all fields are filtered out")
	})
}

func TestValidateFormData(t *testing.T) {
	svc := NewValidationService(nil)

	schema := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput, Label: "Reason", IsRequired: true, Validation: &approval.ValidationRule{MinLength: new(3)}},
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", Validation: &approval.ValidationRule{Min: new(10.0), Max: new(100.0)}},
	}}

	t.Run("ValidData", func(t *testing.T) {
		err := svc.ValidateFormData(schema, map[string]any{"reason": "Travel", "amount": 20})
		assert.NoError(t, err, "Should accept valid form data")
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		err := svc.ValidateFormData(schema, map[string]any{"amount": 20})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("UnknownField", func(t *testing.T) {
		err := svc.ValidateFormData(schema, map[string]any{"reason": "Travel", "extra": true})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("InvalidStringLength", func(t *testing.T) {
		err := svc.ValidateFormData(schema, map[string]any{"reason": "Go"})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("InvalidNumberType", func(t *testing.T) {
		err := svc.ValidateFormData(schema, map[string]any{"reason": "Travel", "amount": "bad"})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})
}
