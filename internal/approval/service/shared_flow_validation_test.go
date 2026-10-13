package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
)

// sharedRuleService registers, beside the built-ins, a host assignee kind
// drawn from a custom catalog, a host kind picking users, and a re-registered
// built-in "role" kind whose host resolver reads a tenant-neutral catalog.
func sharedRuleService() *service.FlowDefinitionService {
	assigneeKinds := append(strategy.BuiltinAssigneeKinds(),
		approval.KindDescriptor[approval.AssigneeKind]{Kind: "expert_panel", Label: "Expert panel", Selection: approval.SelectionCustom},
		approval.KindDescriptor[approval.AssigneeKind]{Kind: "named_reviewers", Label: "Named reviewers", Selection: approval.SelectionUser},
	)

	for i, descriptor := range assigneeKinds {
		if descriptor.Kind == approval.AssigneeRole {
			assigneeKinds[i].Selection = approval.SelectionCustom
		}
	}

	return service.NewFlowDefinitionService(
		service.WithAssigneeKinds(assigneeKinds...),
		service.WithCCKinds(strategy.BuiltinCCKinds()...),
	)
}

func approvalNodeWith(task approval.TaskNodeData) map[string]approval.NodeData {
	return map[string]approval.NodeData{"approval-1": &approval.ApprovalNodeData{TaskNodeData: task}}
}

func TestValidateSharedFlowNodes(t *testing.T) {
	t.Parallel()

	svc := sharedRuleService()

	t.Run("AcceptsKindsResolvedFromTheInstance", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{
				{Kind: approval.AssigneeSelf},
				{Kind: approval.AssigneeSuperior},
				{Kind: approval.AssigneeDepartmentLeader},
				{Kind: approval.AssigneeFormField, FormField: new("approver")},
			},
			CCs: []approval.CCDefinition{{Kind: approval.CCFormField, FormField: new("watchers")}},
		}))
		assert.NoError(t, err, "Applicant-relative and form-driven kinds run in every tenant")
	})

	t.Run("AcceptsHostCatalogKind", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: "expert_panel", IDs: []string{"panel-1"}}},
		}))
		assert.NoError(t, err, "A host kind drawing from a custom catalog decides its own tenancy")
	})

	t.Run("JudgesOverriddenBuiltinByItsDescriptor", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeRole, IDs: []string{"finance"}}},
		}))
		assert.NoError(t, err, "A host resolver replacing the role kind with a catalog input is not a role picker")
	})

	t.Run("RejectsHostKindPickingUsers", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: "named_reviewers", IDs: []string{"user-1"}}},
		}))
		require.Error(t, err, "A host kind picking users names one tenant's people")
		assert.Contains(t, err.Error(), `"named_reviewers" in node "approval-1"`, "The error should locate the rule")
	})

	t.Run("RejectsBuiltinPeoplePickers", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []approval.AssigneeKind{approval.AssigneeUser, approval.AssigneeDepartment} {
			err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
				Assignees: []approval.AssigneeDefinition{{Kind: kind, IDs: []string{"x"}}},
			}))
			assert.Error(t, err, "Built-in kind %s picks specific people", kind)
		}

		for _, kind := range []approval.CCKind{approval.CCUser, approval.CCRole, approval.CCDepartment} {
			err := svc.ValidateSharedFlowNodes(map[string]approval.NodeData{
				"cc-1": &approval.CCNodeData{CCs: []approval.CCDefinition{{Kind: kind, IDs: []string{"x"}}}},
			})
			assert.Error(t, err, "Built-in CC kind %s picks specific people", kind)
		}
	})

	t.Run("RejectsNamedPeopleOnHandleNodes", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(map[string]approval.NodeData{
			"handle-1": &approval.HandleNodeData{
				Assignees:           []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf}},
				EmptyAssigneeAction: approval.EmptyAssigneeTransferSpecified,
				FallbackUserIDs:     []string{"user-1"},
			},
		})
		assert.Error(t, err, "Handle nodes route work like approval nodes do")
	})

	t.Run("AllowsListsNoActionUses", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees:           []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf}},
			EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
			TimeoutAction:       approval.TimeoutActionNotify,
			AdminUserIDs:        []string{"admin-1"},
			FallbackUserIDs:     []string{"user-1"},
		}))
		assert.NoError(t, err, "A leftover list hands nothing to anyone while no action routes to it")
	})

	t.Run("AllowsInertTimeoutAction", func(t *testing.T) {
		t.Parallel()

		err := svc.ValidateSharedFlowNodes(approvalNodeWith(approval.TaskNodeData{
			Assignees:     []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf}},
			TimeoutAction: approval.TimeoutActionTransferAdmin,
		}))
		assert.NoError(t, err, "A node without a deadline never escalates, so its timeout action hands nothing over")
	})

	t.Run("ReportsTheFirstNodeInKeyOrder", func(t *testing.T) {
		t.Parallel()

		offending := &approval.ApprovalNodeData{
			EmptyAssigneeAction: approval.EmptyAssigneeTransferAdmin,
			AdminUserIDs:        []string{"admin-1"},
		}
		err := svc.ValidateSharedFlowNodes(map[string]approval.NodeData{"node-b": offending, "node-a": offending})
		require.Error(t, err, "Handing a task to node admins names specific people")
		assert.Contains(t, err.Error(), `node "node-a"`, "Validation should walk nodes in a stable order")
	})
}
