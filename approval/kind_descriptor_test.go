package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

func TestSelectionMode(t *testing.T) {
	tests := []struct {
		name             string
		mode             approval.SelectionMode
		valid            bool
		requiresIDs      bool
		requiresFormFiel bool
	}{
		{"None", approval.SelectionNone, true, false, false},
		{"User", approval.SelectionUser, true, true, false},
		{"Role", approval.SelectionRole, true, true, false},
		{"Department", approval.SelectionDepartment, true, true, false},
		{"FormField", approval.SelectionFormField, true, false, true},
		{"Custom", approval.SelectionCustom, true, true, false},
		{"Empty", approval.SelectionMode(""), false, false, false},
		{"Unknown", approval.SelectionMode("whatever"), false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.mode.IsValid(), "%s: IsValid should report %v", tt.name, tt.valid)
			assert.Equal(t, tt.requiresIDs, tt.mode.RequiresIDs(), "%s: RequiresIDs should report %v", tt.name, tt.requiresIDs)
			assert.Equal(t, tt.requiresFormFiel, tt.mode.RequiresFormField(), "%s: RequiresFormField should report %v", tt.name, tt.requiresFormFiel)
		})
	}
}

func TestKindDescriptorValidate(t *testing.T) {
	t.Run("Complete", func(t *testing.T) {
		descriptor := approval.KindDescriptor[approval.AssigneeKind]{
			Kind:      approval.AssigneeUser,
			Label:     "Specified users",
			Selection: approval.SelectionUser,
		}

		require.NoError(t, descriptor.Validate(), "A complete descriptor should validate")
	})

	t.Run("BlankKind", func(t *testing.T) {
		descriptor := approval.KindDescriptor[approval.AssigneeKind]{Label: "L", Selection: approval.SelectionNone}

		require.Error(t, descriptor.Validate(), "A kind nothing can address must be rejected")
	})

	t.Run("BlankLabel", func(t *testing.T) {
		descriptor := approval.KindDescriptor[approval.CCKind]{Kind: approval.CCUser, Selection: approval.SelectionUser}

		require.Error(t, descriptor.Validate(), "A blank label would render as an empty designer option")
	})

	t.Run("InvalidSelection", func(t *testing.T) {
		descriptor := approval.KindDescriptor[approval.InitiatorKind]{
			Kind:      approval.InitiatorUser,
			Label:     "L",
			Selection: "whatever",
		}

		require.Error(t, descriptor.Validate(), "An out-of-enum selection leaves the designer with no input to draw")
	})
}
