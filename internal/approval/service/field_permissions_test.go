package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestValidateFieldPermissions(t *testing.T) {
	svc := NewFlowDefinitionService()

	form := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput},
		{Key: "amount", Kind: approval.FieldNumber},
		{Key: "status", Kind: approval.FieldSelect},
		{Key: "notes", Kind: approval.FieldTextarea},
	}

	approvalNode := func(perms map[string]approval.Permission) map[string]approval.NodeData {
		return map[string]approval.NodeData{
			"n1": &approval.ApprovalNodeData{
				TaskNodeData: approval.TaskNodeData{FieldPermissions: perms},
			},
		}
	}

	ccNode := func(perms map[string]approval.Permission) map[string]approval.NodeData {
		return map[string]approval.NodeData{
			"n1": &approval.CCNodeData{FieldPermissions: perms},
		}
	}

	t.Run("AcceptsFullVocabularyOnApprovalNode", func(t *testing.T) {
		perms := map[string]approval.Permission{
			"reason": approval.PermissionVisible,
			"amount": approval.PermissionEditable,
			"status": approval.PermissionHidden,
			"notes":  approval.PermissionRequired,
		}
		assert.NoError(t, svc.ValidateFieldPermissions(approvalNode(perms), form),
			"approval nodes accept the full visible/editable/hidden/required vocabulary")
	})

	t.Run("AcceptsVisibleHiddenOnCCNode", func(t *testing.T) {
		perms := map[string]approval.Permission{
			"reason": approval.PermissionVisible,
			"amount": approval.PermissionHidden,
		}
		assert.NoError(t, svc.ValidateFieldPermissions(ccNode(perms), form),
			"CC nodes accept the visible/hidden subset")
	})

	t.Run("RejectsOutOfEnumValue", func(t *testing.T) {
		perms := map[string]approval.Permission{"reason": approval.Permission("locked")}
		assert.ErrorIs(t, svc.ValidateFieldPermissions(approvalNode(perms), form), errInvalidFieldPermission,
			"an out-of-enum permission value must fail at deploy")
	})

	t.Run("RejectsUnknownFieldKey", func(t *testing.T) {
		perms := map[string]approval.Permission{"nonexistent": approval.PermissionVisible}
		assert.ErrorIs(t, svc.ValidateFieldPermissions(approvalNode(perms), form), errFieldPermissionKeyUnknown,
			"a key with no matching form field is a dangling reference")
	})

	t.Run("RejectsEditableOnCCNode", func(t *testing.T) {
		perms := map[string]approval.Permission{"reason": approval.PermissionEditable}
		assert.ErrorIs(t, svc.ValidateFieldPermissions(ccNode(perms), form), errCCFieldPermissionNotAllowed,
			"a CC node only observes the form and must not unlock editing")
	})

	t.Run("RejectsRequiredOnCCNode", func(t *testing.T) {
		perms := map[string]approval.Permission{"reason": approval.PermissionRequired}
		assert.ErrorIs(t, svc.ValidateFieldPermissions(ccNode(perms), form), errCCFieldPermissionNotAllowed,
			"a CC node only observes the form and must not demand a value")
	})

	t.Run("AcceptsNodeWithoutFieldPermissions", func(t *testing.T) {
		data := map[string]approval.NodeData{
			"n1": &approval.ApprovalNodeData{},
			"n2": &approval.ConditionNodeData{},
		}
		assert.NoError(t, svc.ValidateFieldPermissions(data, form),
			"a node with no configured field permissions has nothing to check")
	})

	t.Run("RejectsPermissionsWithNoFormFields", func(t *testing.T) {
		perms := map[string]approval.Permission{"reason": approval.PermissionVisible}
		assert.ErrorIs(t, svc.ValidateFieldPermissions(approvalNode(perms), nil), errFieldPermissionKeyUnknown,
			"a flow without a form makes every permission entry a dangling reference")
	})
}
