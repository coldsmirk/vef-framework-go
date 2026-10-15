package service

import (
	"fmt"
	"maps"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// ValidateSharedFlowNodes enforces what a shared flow (approval.Flow.IsShared)
// adds to a valid definition: it routes no task and no CC to specific people.
// A shared flow runs in every tenant, while a user — and the users behind a
// role or a department — belongs to one, so a rule naming them would deliver
// every other tenant's applications to that tenant's people. That covers the
// assignee and CC rules and the actions handing a task to node admins or
// fallback users. Rules resolved from the applicant (superior, department
// leader, the applicant), from the form, or from a host catalog stay
// available; so do flow admins and initiator rules, which grant nothing
// outside the tenant an instance runs in.
//
// It reads the registered descriptors, so a host kind is judged by the input it
// declares exactly as a built-in is: one that selects users, roles or
// departments is rejected, one resolving from the context or a custom catalog
// is not.
func (s *FlowDefinitionService) ValidateSharedFlowNodes(parsed map[string]approval.NodeData) error {
	for _, nodeID := range slices.Sorted(maps.Keys(parsed)) {
		switch typed := parsed[nodeID].(type) {
		case *approval.ApprovalNodeData:
			if err := s.validateSharedTaskNode(nodeID, &typed.TaskNodeData); err != nil {
				return err
			}

		case *approval.HandleNodeData:
			if err := s.validateSharedTaskNode(nodeID, &typed.TaskNodeData); err != nil {
				return err
			}

		case *approval.CCNodeData:
			if err := s.validateSharedCCs(nodeID, typed.CCs); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *FlowDefinitionService) validateSharedTaskNode(nodeID string, data *approval.TaskNodeData) error {
	for _, assignee := range data.Assignees {
		if selectsPeople(s.assigneeKinds[assignee.Kind]) {
			return fmt.Errorf("%w: %q in node %q", errSharedAssigneeNamesPeople, assignee.Kind, nodeID)
		}
	}

	// The node admin and fallback lists name people, but only these actions
	// hand work to them; judging the actions rather than the lists never
	// rejects a leftover list the designer no longer shows.
	switch data.EmptyAssigneeAction {
	case approval.EmptyAssigneeTransferSpecified:
		return fmt.Errorf("%w: node %q", errSharedFallbackUsers, nodeID)
	case approval.EmptyAssigneeTransferAdmin:
		return fmt.Errorf("%w: node %q", errSharedNodeAdmins, nodeID)
	}

	// A node without a deadline never times out, so its timeout action is
	// inert.
	if data.TimeoutHours > 0 && data.TimeoutAction == approval.TimeoutActionTransferAdmin {
		return fmt.Errorf("%w: node %q", errSharedTimeoutToAdmins, nodeID)
	}

	return s.validateSharedCCs(nodeID, data.CCs)
}

func (s *FlowDefinitionService) validateSharedCCs(nodeID string, ccs []approval.CCDefinition) error {
	for _, cc := range ccs {
		if selectsPeople(s.ccKinds[cc.Kind]) {
			return fmt.Errorf("%w: %q in node %q", errSharedCCNamesPeople, cc.Kind, nodeID)
		}
	}

	return nil
}

// selectsPeople reports whether a kind's designer input picks specific users,
// roles or departments — entities that belong to one tenant.
func selectsPeople(mode approval.SelectionMode) bool {
	switch mode {
	case approval.SelectionUser, approval.SelectionRole, approval.SelectionDepartment:
		return true
	default:
		return false
	}
}
