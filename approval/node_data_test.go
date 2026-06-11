package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// newFreshNode returns a zero-value FlowNode with only its kind set,
// mirroring what deploy_flow.go constructs before calling ApplyTo.
func newFreshNode(kind approval.NodeKind) *approval.FlowNode {
	return &approval.FlowNode{Kind: kind}
}

func TestStartNodeDataApplyTo(t *testing.T) {
	desc := "entry point"
	data := &approval.StartNodeData{
		BaseNodeData: approval.BaseNodeData{
			Name:        "Start",
			Description: &desc,
		},
	}

	node := newFreshNode(approval.NodeStart)
	data.ApplyTo(node)

	assert.Equal(t, "Start", node.Name, "ApplyTo should set node Name from StartNodeData")
	require.NotNil(t, node.Description, "ApplyTo should set Description pointer")
	assert.Equal(t, desc, *node.Description, "ApplyTo should propagate Description value")
	assert.Equal(t, approval.NodeStart, data.Kind(), "Kind() should return NodeStart")
}

func TestEndNodeDataApplyTo(t *testing.T) {
	data := &approval.EndNodeData{
		BaseNodeData: approval.BaseNodeData{Name: "End"},
	}
	node := newFreshNode(approval.NodeEnd)
	data.ApplyTo(node)

	assert.Equal(t, "End", node.Name, "ApplyTo should set node Name from EndNodeData")
	assert.Equal(t, approval.NodeEnd, data.Kind(), "Kind() should return NodeEnd")
}

func TestApprovalNodeDataApplyTo(t *testing.T) {
	data := &approval.ApprovalNodeData{
		BaseNodeData: approval.BaseNodeData{Name: "Review"},
		TaskNodeData: approval.TaskNodeData{
			ExecutionType:     approval.ExecutionManual,
			FallbackUserIDs:   []string{"u1"},
			IsTransferAllowed: new(true),
			TimeoutHours:      48,
		},
		ApprovalMethod:       approval.ApprovalSequential,
		PassRule:             approval.PassAny,
		IsRollbackAllowed:    new(true),
		IsAddAssigneeAllowed: new(true),
		AddAssigneeTypes:     []approval.AddAssigneeType{approval.AddAssigneeBefore},
	}

	node := newFreshNode(approval.NodeApproval)
	data.ApplyTo(node)

	assert.Equal(t, "Review", node.Name, "ApplyTo should set Name")
	assert.Equal(t, approval.ExecutionManual, node.ExecutionType, "ApplyTo should set ExecutionType")
	assert.Equal(t, []string{"u1"}, node.FallbackUserIDs, "ApplyTo should set FallbackUserIDs")
	assert.True(t, node.IsTransferAllowed, "ApplyTo should set IsTransferAllowed")
	assert.Equal(t, 48, node.TimeoutHours, "ApplyTo should set TimeoutHours")
	assert.Equal(t, approval.ApprovalSequential, node.ApprovalMethod, "ApplyTo should keep an explicit ApprovalMethod")
	assert.Equal(t, approval.PassAny, node.PassRule, "ApplyTo should keep an explicit PassRule")
	assert.True(t, node.IsRollbackAllowed, "ApplyTo should set IsRollbackAllowed")
	assert.True(t, node.IsAddAssigneeAllowed, "ApplyTo should set IsAddAssigneeAllowed")
	assert.Equal(t, []approval.AddAssigneeType{approval.AddAssigneeBefore}, node.AddAssigneeTypes, "ApplyTo should set AddAssigneeTypes")
	assert.Equal(t, approval.NodeApproval, data.Kind(), "Kind() should return NodeApproval")
}

func TestApprovalNodeDataApplyToDefaults(t *testing.T) {
	t.Run("OmittedFieldsResolveToDesignerDefaults", func(t *testing.T) {
		// The designer serializes only touched controls, so a bare payload
		// must deploy to a fully-configured node carrying the same defaults
		// the designer displays.
		data := &approval.ApprovalNodeData{
			BaseNodeData: approval.BaseNodeData{Name: "Blank"},
		}
		node := newFreshNode(approval.NodeApproval)
		data.ApplyTo(node)

		assert.Equal(t, approval.DefaultExecutionType, node.ExecutionType, "omitted ExecutionType should resolve to manual")
		assert.Equal(t, approval.DefaultApprovalMethod, node.ApprovalMethod, "omitted ApprovalMethod should resolve to parallel")
		assert.Equal(t, approval.DefaultPassRule, node.PassRule, "omitted PassRule should resolve to all")
		assert.Equal(t, approval.DefaultEmptyAssigneeAction, node.EmptyAssigneeAction, "omitted EmptyAssigneeAction should resolve to auto_pass")
		assert.Equal(t, approval.DefaultSameApplicantAction, node.SameApplicantAction, "omitted SameApplicantAction should resolve to self_approve")
		assert.Equal(t, approval.DefaultConsecutiveApproverAction, node.ConsecutiveApproverAction, "omitted ConsecutiveApproverAction should resolve to none")
		assert.Equal(t, approval.DefaultRollbackType, node.RollbackType, "omitted RollbackType should resolve to previous")
		assert.Equal(t, approval.DefaultRollbackDataStrategy, node.RollbackDataStrategy, "omitted RollbackDataStrategy should resolve to keep")
		assert.Equal(t, approval.DefaultTimeoutAction, node.TimeoutAction, "omitted TimeoutAction should resolve to none")
		assert.True(t, node.IsTransferAllowed, "omitted IsTransferAllowed should resolve to allowed")
		assert.True(t, node.IsRollbackAllowed, "omitted IsRollbackAllowed should resolve to allowed")
		assert.True(t, node.IsAddAssigneeAllowed, "omitted IsAddAssigneeAllowed should resolve to allowed")
		assert.True(t, node.IsRemoveAssigneeAllowed, "omitted IsRemoveAssigneeAllowed should resolve to allowed")
		assert.True(t, node.IsManualCCAllowed, "omitted IsManualCCAllowed should resolve to allowed")
	})

	t.Run("ExplicitFalseSurvivesDefaulting", func(t *testing.T) {
		// Pointer booleans exist precisely so explicit false is
		// distinguishable from omitted; defaulting must not flip it back.
		data := &approval.ApprovalNodeData{
			TaskNodeData:            approval.TaskNodeData{IsTransferAllowed: new(false)},
			IsRollbackAllowed:       new(false),
			IsAddAssigneeAllowed:    new(false),
			IsRemoveAssigneeAllowed: new(false),
			IsManualCCAllowed:       new(false),
		}
		node := newFreshNode(approval.NodeApproval)
		data.ApplyTo(node)

		assert.False(t, node.IsTransferAllowed, "explicit false IsTransferAllowed must persist")
		assert.False(t, node.IsRollbackAllowed, "explicit false IsRollbackAllowed must persist")
		assert.False(t, node.IsAddAssigneeAllowed, "explicit false IsAddAssigneeAllowed must persist")
		assert.False(t, node.IsRemoveAssigneeAllowed, "explicit false IsRemoveAssigneeAllowed must persist")
		assert.False(t, node.IsManualCCAllowed, "explicit false IsManualCCAllowed must persist")
	})
}

func TestHandleNodeDataApplyTo(t *testing.T) {
	data := &approval.HandleNodeData{
		BaseNodeData: approval.BaseNodeData{Name: "Handle"},
		TaskNodeData: approval.TaskNodeData{
			ExecutionType:     approval.ExecutionManual,
			IsOpinionRequired: true,
		},
	}

	node := newFreshNode(approval.NodeHandle)
	data.ApplyTo(node)

	assert.Equal(t, "Handle", node.Name, "ApplyTo should set Name")
	assert.Equal(t, approval.ExecutionManual, node.ExecutionType, "ApplyTo should propagate ExecutionType")
	assert.True(t, node.IsOpinionRequired, "ApplyTo should set IsOpinionRequired")
	assert.Equal(t, approval.NodeHandle, data.Kind(), "Kind() should return NodeHandle")
}

func TestHandleNodeDataDefaultsApprovalMethodAndPassRule(t *testing.T) {
	// HandleNodeData.ApplyTo must default ApprovalMethod=Sequential and PassRule=Any
	// on a fresh node (neither field set in the incoming data).
	t.Run("DefaultsAppliedWhenNodeIsEmpty", func(t *testing.T) {
		data := &approval.HandleNodeData{
			BaseNodeData: approval.BaseNodeData{Name: "H"},
		}
		node := newFreshNode(approval.NodeHandle)
		data.ApplyTo(node)

		assert.Equal(t, approval.ApprovalSequential, node.ApprovalMethod,
			"HandleNodeData.ApplyTo should default ApprovalMethod to Sequential")
		assert.Equal(t, approval.PassAny, node.PassRule,
			"HandleNodeData.ApplyTo should default PassRule to PassAny")
	})

	t.Run("DefaultsNotAppliedWhenDataProvidesValues", func(t *testing.T) {
		// When TaskNodeData carries ExecutionType (which sets node.ExecutionType via
		// applyTaskNodeData) but does NOT carry ApprovalMethod, the handle node
		// defaults still apply (only ApprovalMethod and PassRule are defaulted).
		data := &approval.HandleNodeData{
			BaseNodeData: approval.BaseNodeData{Name: "H"},
			TaskNodeData: approval.TaskNodeData{ExecutionType: approval.ExecutionAutoPass},
		}
		node := newFreshNode(approval.NodeHandle)
		data.ApplyTo(node)

		assert.Equal(t, approval.ApprovalSequential, node.ApprovalMethod,
			"Default ApprovalMethod should be applied regardless of other fields")
		assert.Equal(t, approval.PassAny, node.PassRule,
			"Default PassRule should be applied regardless of other fields")
		assert.Equal(t, approval.ExecutionAutoPass, node.ExecutionType,
			"ExecutionType from TaskNodeData should be propagated")
	})
}

func TestCCNodeDataApplyTo(t *testing.T) {
	perms := map[string]approval.Permission{"fieldA": approval.PermissionVisible}
	data := &approval.CCNodeData{
		BaseNodeData:          approval.BaseNodeData{Name: "CC"},
		IsReadConfirmRequired: true,
		FieldPermissions:      perms,
	}

	node := newFreshNode(approval.NodeCC)
	data.ApplyTo(node)

	assert.Equal(t, "CC", node.Name, "ApplyTo should set Name")
	assert.True(t, node.IsReadConfirmRequired, "ApplyTo should set IsReadConfirmRequired")
	assert.Equal(t, perms, node.FieldPermissions, "ApplyTo should set FieldPermissions")
	assert.Equal(t, approval.NodeCC, data.Kind(), "Kind() should return NodeCC")
}

func TestConditionNodeDataApplyTo(t *testing.T) {
	branches := []approval.ConditionBranch{
		{ID: "b1", Label: "Branch A", IsDefault: false},
		{ID: "b2", Label: "Default", IsDefault: true},
	}
	data := &approval.ConditionNodeData{
		BaseNodeData: approval.BaseNodeData{Name: "Condition"},
		Branches:     branches,
	}

	node := newFreshNode(approval.NodeCondition)
	data.ApplyTo(node)

	assert.Equal(t, "Condition", node.Name, "ApplyTo should set Name")
	assert.Equal(t, branches, node.Branches, "ApplyTo should set Branches")
	assert.Equal(t, approval.NodeCondition, data.Kind(), "Kind() should return NodeCondition")
}

func TestTaskNodeDataGetAssigneesAndCCs(t *testing.T) {
	assignees := []approval.AssigneeDefinition{
		{Kind: approval.AssigneeUser, IDs: []string{"u1"}, SortOrder: 1},
	}
	ccs := []approval.CCDefinition{
		{Kind: approval.CCUser, IDs: []string{"cc1"}},
	}
	data := &approval.TaskNodeData{
		Assignees: assignees,
		CCs:       ccs,
	}

	assert.Equal(t, assignees, data.GetAssignees(), "GetAssignees should return Assignees slice")
	assert.Equal(t, ccs, data.GetCCs(), "GetCCs should return CCs slice")
}
