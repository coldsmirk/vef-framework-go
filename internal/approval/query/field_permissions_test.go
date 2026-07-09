package query

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// permFields builds a flat form-field list carrying only the keys under test.
func permFields(keys ...string) []approval.FormFieldDefinition {
	defs := make([]approval.FormFieldDefinition, len(keys))
	for i, k := range keys {
		defs[i] = approval.FormFieldDefinition{Key: k}
	}

	return defs
}

// permNode builds a flow node with the given id and field-permission map.
func permNode(id string, perms map[string]approval.Permission) approval.FlowNode {
	n := approval.FlowNode{FieldPermissions: perms}
	n.ID = id

	return n
}

// permTask builds an assignee task for the merge fixtures.
func permTask(assignee, nodeID string, status approval.TaskStatus) approval.Task {
	return approval.Task{AssigneeID: assignee, NodeID: nodeID, Status: status}
}

// permCC builds a CC record; nodeID nil marks an instance-level manual CC.
func permCC(user string, nodeID *string) approval.CCRecord {
	return approval.CCRecord{CCUserID: user, NodeID: nodeID}
}

func TestResolveViewerFieldPermissions(t *testing.T) {
	// vocab exercises the full permission vocabulary on one node: "plain" is
	// deliberately absent (defaults to visible).
	vocab := map[string]approval.Permission{
		"edit": approval.PermissionEditable,
		"req":  approval.PermissionRequired,
		"hide": approval.PermissionHidden,
	}

	tests := []struct {
		name   string
		bundle *instanceDetailBundle
		userID string
		want   map[string]approval.Permission
	}{
		{
			name: "PendingTaskContributesFullStrength",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Tasks:      []approval.Task{permTask("u", "N1", approval.TaskPending)},
				Instance:   approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionEditable, "req": approval.PermissionRequired,
				"hide": approval.PermissionHidden, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "FinishedTaskReadDowngradesWriteButKeepsHidden",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Tasks:      []approval.Task{permTask("u", "N1", approval.TaskApproved)},
				Instance:   approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionVisible, "req": approval.PermissionVisible,
				"hide": approval.PermissionHidden, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "WaitingTaskReadDowngradesLikeFinished",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Tasks:      []approval.Task{permTask("u", "N1", approval.TaskWaiting)},
				Instance:   approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionVisible, "req": approval.PermissionVisible,
				"hide": approval.PermissionHidden, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "NodeAnchoredCCReadDowngrades",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				CCRecords:  []approval.CCRecord{permCC("u", new("N1"))},
				Instance:   approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionVisible, "req": approval.PermissionVisible,
				"hide": approval.PermissionHidden, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "InstanceLevelManualCCSeesEverything",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				CCRecords:  []approval.CCRecord{permCC("u", nil)},
				Instance:   approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionVisible, "req": approval.PermissionVisible,
				"hide": approval.PermissionVisible, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "ApplicantRunningSeesEverythingReadOnly",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Instance:   approval.Instance{ApplicantID: "u", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionVisible, "req": approval.PermissionVisible,
				"hide": approval.PermissionVisible, "plain": approval.PermissionVisible,
			},
		},
		{
			name: "ApplicantResubmittableGetsWholeFormEditable",
			bundle: &instanceDetailBundle{
				FormFields: permFields("edit", "req", "hide", "plain"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Instance:   approval.Instance{ApplicantID: "u", Status: approval.InstanceReturned},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"edit": approval.PermissionEditable, "req": approval.PermissionEditable,
				"hide": approval.PermissionEditable, "plain": approval.PermissionEditable,
			},
		},
		{
			name: "UnionPendingEditableAndCCHiddenTakesEditable",
			bundle: &instanceDetailBundle{
				FormFields: permFields("x"),
				FlowNodes: []approval.FlowNode{
					permNode("A", map[string]approval.Permission{"x": approval.PermissionEditable}),
					permNode("B", map[string]approval.Permission{"x": approval.PermissionHidden}),
				},
				Tasks:     []approval.Task{permTask("u", "A", approval.TaskPending)},
				CCRecords: []approval.CCRecord{permCC("u", new("B"))},
				Instance:  approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want:   map[string]approval.Permission{"x": approval.PermissionEditable},
		},
		{
			name: "UnionFinishedHiddenAndApplicantVisibleTakesVisible",
			bundle: &instanceDetailBundle{
				FormFields: permFields("x"),
				FlowNodes: []approval.FlowNode{
					permNode("A", map[string]approval.Permission{"x": approval.PermissionHidden}),
				},
				Tasks:    []approval.Task{permTask("u", "A", approval.TaskApproved)},
				Instance: approval.Instance{ApplicantID: "u", Status: approval.InstanceRunning},
			},
			userID: "u",
			want:   map[string]approval.Permission{"x": approval.PermissionVisible},
		},
		{
			name: "AbsentNodeMapKeyDefaultsToVisible",
			bundle: &instanceDetailBundle{
				FormFields: permFields("a", "b"),
				FlowNodes: []approval.FlowNode{
					permNode("N1", map[string]approval.Permission{"a": approval.PermissionEditable}),
				},
				Tasks:    []approval.Task{permTask("u", "N1", approval.TaskPending)},
				Instance: approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want: map[string]approval.Permission{
				"a": approval.PermissionEditable, "b": approval.PermissionVisible,
			},
		},
		{
			name: "NodeMapKeyNotInFormFieldsIsIgnored",
			bundle: &instanceDetailBundle{
				FormFields: permFields("a"),
				FlowNodes: []approval.FlowNode{
					permNode("N1", map[string]approval.Permission{
						"a": approval.PermissionEditable, "ghost": approval.PermissionHidden,
					}),
				},
				Tasks:    []approval.Task{permTask("u", "N1", approval.TaskPending)},
				Instance: approval.Instance{ApplicantID: "other", Status: approval.InstanceRunning},
			},
			userID: "u",
			want:   map[string]approval.Permission{"a": approval.PermissionEditable},
		},
		{
			name: "EmptyFormFieldsReturnsNil",
			bundle: &instanceDetailBundle{
				FormFields: nil,
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Tasks:      []approval.Task{permTask("u", "N1", approval.TaskPending)},
				Instance:   approval.Instance{ApplicantID: "u", Status: approval.InstanceRunning},
			},
			userID: "u",
			want:   nil,
		},
		{
			name: "ViewerWithNoContextsDefaultsToAllVisible",
			bundle: &instanceDetailBundle{
				FormFields: permFields("a", "b"),
				FlowNodes:  []approval.FlowNode{permNode("N1", vocab)},
				Tasks:      []approval.Task{permTask("someone", "N1", approval.TaskPending)},
				CCRecords:  []approval.CCRecord{permCC("other", nil)},
				Instance:   approval.Instance{ApplicantID: "applicant", Status: approval.InstanceRunning},
			},
			userID: "stranger",
			want: map[string]approval.Permission{
				"a": approval.PermissionVisible, "b": approval.PermissionVisible,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveViewerFieldPermissions(tt.bundle, tt.userID)
			assert.Equal(t, tt.want, got, "Resolved viewer field permissions should match the expected projection")
		})
	}
}

func TestStripHiddenFormData(t *testing.T) {
	perms := map[string]approval.Permission{
		"visible": approval.PermissionVisible,
		"secret":  approval.PermissionHidden,
	}

	t.Run("RemovesHiddenKeepsVisibleAndLegacy", func(t *testing.T) {
		src := map[string]any{"visible": 1, "secret": 2, "legacy": 3}

		got := stripHiddenFormData(src, perms)

		assert.Equal(t, map[string]any{"visible": 1, "legacy": 3}, got, "Hidden field should be stripped; visible and schemaless legacy fields kept")
		assert.Equal(t, map[string]any{"visible": 1, "secret": 2, "legacy": 3}, src, "Source form data must not be mutated in place")
	})

	t.Run("NilFormDataStaysNil", func(t *testing.T) {
		assert.Nil(t, stripHiddenFormData(nil, perms), "Nil form data should yield nil")
	})

	t.Run("NilPermsStripsNothing", func(t *testing.T) {
		src := map[string]any{"a": 1, "b": 2}

		got := stripHiddenFormData(src, nil)

		assert.Equal(t, src, got, "With no permissions nothing is hidden, so every key survives")
	})
}
