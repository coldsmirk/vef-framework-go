package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/result"
)

// mustInitiatorCompositeInternal builds the framework's own initiator
// vocabulary. Registration is static, so a failure here is a programming error
// rather than a test condition.
func mustInitiatorCompositeInternal(svc approval.AssigneeService) *strategy.CompositeInitiatorResolver {
	composite, err := strategy.NewCompositeInitiatorResolver(strategy.BuiltinInitiatorResolvers(svc), nil)
	if err != nil {
		panic(err)
	}

	return composite
}

// --- ValidateOpinion ---

func TestValidateOpinion(t *testing.T) {
	svc := NewValidationService(mustInitiatorCompositeInternal(nil))

	t.Run("RequiredAndProvided", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "looks good")
		assert.NoError(t, err, "Should pass when opinion is required and provided")
	})

	t.Run("RequiredButEmpty", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "")
		assert.ErrorIs(t, err, approval.ErrOpinionRequired, "Should fail when opinion is required but empty")
	})

	t.Run("RequiredButWhitespaceOnly", func(t *testing.T) {
		node := &approval.FlowNode{IsOpinionRequired: true}
		err := svc.ValidateOpinion(node, "   \t")
		assert.ErrorIs(t, err, approval.ErrOpinionRequired, "Should fail when opinion is only whitespace")
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
	svc := NewValidationService(mustInitiatorCompositeInternal(nil))

	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput, Label: "Reason", IsRequired: true, Validation: &approval.ValidationRule{MinLength: new(3)}},
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", Validation: &approval.ValidationRule{Min: new(10.0), Max: new(100.0)}},
	}

	t.Run("ValidData", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"reason": "Travel", "amount": 20})
		assert.NoError(t, err, "Should accept valid form data")
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"amount": 20})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("UnknownField", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"reason": "Travel", "extra": true})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("InvalidStringLength", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"reason": "Go"})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("InvalidNumberType", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"reason": "Travel", "amount": "bad"})

		var re result.Error
		require.ErrorAs(t, err, &re, "Should return business error")
		assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
	})

	t.Run("RejectsPayloadOverTheAbsoluteCap", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"reason": "Travel", "amount": 20, "blob": strings.Repeat("x", config.DefaultFormDataMaxBytes+1)})
		require.ErrorIs(t, err, approval.ErrFormDataTooLarge, "Start / resubmit must reject a payload over the absolute size cap")
	})

	t.Run("SizeGuardAppliesWithoutSchema", func(t *testing.T) {
		err := svc.ValidateFormData(nil, map[string]any{"blob": strings.Repeat("x", config.DefaultFormDataMaxBytes+1)})
		require.ErrorIs(t, err, approval.ErrFormDataTooLarge, "Size guard must apply even when the flow has no form schema")
	})
}

// TestFormDataMaxBytesIsConfigurable pins the cap as host policy rather than a
// framework constant. A flow whose detail table carries thousands of rows — a
// scheduling roster, an itemized settlement — legitimately exceeds the 64 KiB
// default, and raising it must not require rebuilding the framework.
func TestFormDataMaxBytesIsConfigurable(t *testing.T) {
	// Comfortably past the default, so a service still on it would reject.
	const raised = config.DefaultFormDataMaxBytes * 4

	payload := map[string]any{"blob": strings.Repeat("x", config.DefaultFormDataMaxBytes+1)}

	t.Run("DefaultRejectsWhatTheRaisedLimitAccepts", func(t *testing.T) {
		require.ErrorIs(t, NewValidationService(mustInitiatorCompositeInternal(nil)).ValidateFormData(nil, payload),
			approval.ErrFormDataTooLarge,
			"The default cap must still reject the payload, or this test proves nothing")

		require.NoError(t, NewValidationService(mustInitiatorCompositeInternal(nil), WithFormDataMaxBytes(raised)).ValidateFormData(nil, payload),
			"A raised cap must accept a payload the default rejects")
	})

	t.Run("RaisedLimitStillRejectsBeyondIt", func(t *testing.T) {
		svc := NewValidationService(mustInitiatorCompositeInternal(nil), WithFormDataMaxBytes(raised))
		err := svc.ValidateFormData(nil, map[string]any{"blob": strings.Repeat("x", raised+1)})
		require.ErrorIs(t, err, approval.ErrFormDataTooLarge, "Raising the cap must move the bound, not remove it")
	})

	t.Run("NonPositiveLimitKeepsTheDefault", func(t *testing.T) {
		// EffectiveFormDataMaxBytes maps unset config to the default, but the
		// option is public API too — a caller passing 0 must not disable the cap.
		for name, limit := range map[string]int{"Zero": 0, "Negative": -1} {
			t.Run(name, func(t *testing.T) {
				svc := NewValidationService(mustInitiatorCompositeInternal(nil), WithFormDataMaxBytes(limit))
				require.ErrorIs(t, svc.ValidateFormData(nil, payload), approval.ErrFormDataTooLarge,
					"A non-positive limit must fall back to the default, never to unbounded")
			})
		}
	})
}

// Role-membership resolution moved to shared.UserHasRole; its tests live in
// internal/approval/shared/user_resolve_test.go.

func TestValidateFormDataTableField(t *testing.T) {
	svc := NewValidationService(mustInitiatorCompositeInternal(nil))

	minRows, maxRows := 1, 2
	fields := []approval.FormFieldDefinition{
		{
			Key: "items", Kind: approval.FieldTable, Label: "Items", IsRequired: true,
			Validation: &approval.ValidationRule{MinLength: &minRows, MaxLength: &maxRows},
			Columns: []approval.FormFieldDefinition{
				{Key: "name", Kind: approval.FieldInput, Label: "Name", IsRequired: true},
				{Key: "qty", Kind: approval.FieldNumber, Label: "Qty"},
			},
		},
	}

	valid := func(rows ...any) map[string]any { return map[string]any{"items": rows} }

	t.Run("AcceptsValidRows", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid(map[string]any{"name": "hotel", "qty": 2}))
		assert.NoError(t, err, "a well-shaped detail row should validate")
	})

	t.Run("RequiredMeansAtLeastOneRow", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"items": []any{}})
		assert.Error(t, err, "an empty list on a required table is an empty value")
	})

	t.Run("RejectsNonListValue", func(t *testing.T) {
		err := svc.ValidateFormData(fields, map[string]any{"items": "oops"})
		assert.Error(t, err, "a non-list table value must be rejected")
	})

	t.Run("RejectsNonObjectRow", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid("not-a-row"))
		assert.Error(t, err, "a non-object row must be rejected")
	})

	t.Run("EnforcesRowBounds", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid(
			map[string]any{"name": "a"}, map[string]any{"name": "b"}, map[string]any{"name": "c"},
		))
		assert.Error(t, err, "row count above MaxLength must be rejected")
	})

	t.Run("EnforcesRequiredColumns", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid(map[string]any{"qty": 1}))
		assert.Error(t, err, "a required column missing in a row must be rejected")
	})

	t.Run("RejectsUndeclaredRowKeys", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid(map[string]any{"name": "ok", "ghost": 1}))
		assert.Error(t, err, "rows are closed like the top-level form — undeclared keys must be rejected")
	})

	t.Run("ValidatesColumnValuesPerRow", func(t *testing.T) {
		err := svc.ValidateFormData(fields, valid(map[string]any{"name": "ok", "qty": "NaN"}))
		assert.Error(t, err, "a non-numeric value in a number column must be rejected")
	})
}

func TestValidateRequiredPermissionFields(t *testing.T) {
	svc := NewValidationService(mustInitiatorCompositeInternal(nil))

	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput, Label: "Reason"},
		{Key: "items", Kind: approval.FieldTable, Label: "Items"},
	}

	required := func(keys ...string) map[string]approval.Permission {
		perms := make(map[string]approval.Permission, len(keys))
		for _, key := range keys {
			perms[key] = approval.PermissionRequired
		}

		return perms
	}

	tests := []struct {
		name        string
		fields      []approval.FormFieldDefinition
		permissions map[string]approval.Permission
		formData    map[string]any
		wantErr     bool
	}{
		{
			name:        "RequiredButMissing",
			fields:      fields,
			permissions: required("reason"),
			formData:    map[string]any{},
			wantErr:     true,
		},
		{
			name:        "RequiredButEmptyString",
			fields:      fields,
			permissions: required("reason"),
			formData:    map[string]any{"reason": ""},
			wantErr:     true,
		},
		{
			name:        "RequiredButWhitespaceOnly",
			fields:      fields,
			permissions: required("reason"),
			formData:    map[string]any{"reason": "   \t"},
			wantErr:     true,
		},
		{
			name:        "RequiredButEmptyList",
			fields:      fields,
			permissions: required("items"),
			formData:    map[string]any{"items": []any{}},
			wantErr:     true,
		},
		{
			name:        "RequiredAndFilled",
			fields:      fields,
			permissions: required("reason"),
			formData:    map[string]any{"reason": "Travel"},
			wantErr:     false,
		},
		{
			name:        "RequiredKeyAbsentFromFields",
			fields:      fields,
			permissions: required("ghost"),
			formData:    map[string]any{},
			wantErr:     false,
		},
		{
			name:        "NoRequiredEntries",
			fields:      fields,
			permissions: map[string]approval.Permission{"reason": approval.PermissionEditable, "items": approval.PermissionVisible},
			formData:    map[string]any{},
			wantErr:     false,
		},
		{
			name:        "NilFormDataWithRequired",
			fields:      fields,
			permissions: required("reason"),
			formData:    nil,
			wantErr:     true,
		},
		{
			name:        "NilFormDataNoPermissions",
			fields:      fields,
			permissions: nil,
			formData:    nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateRequiredPermissionFields(tt.fields, tt.permissions, tt.formData)

			if !tt.wantErr {
				assert.NoError(t, err, "Should accept when every required-permission field is filled")

				return
			}

			var re result.Error
			require.ErrorAs(t, err, &re, "Should return a business validation error")
			assert.Equal(t, approval.ErrCodeFormValidationFailed, re.Code, "Should carry the form validation error code")
		})
	}
}
