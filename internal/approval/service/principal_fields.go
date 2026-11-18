package service

import (
	"fmt"
	"github.com/coldsmirk/vef-framework-go/approval"
	"strings"
)

// ValidatePrincipalFields checks selection rules whose resolver reads user IDs
// from a deployed field, including host-defined kinds with that selection mode.
func (s *FlowDefinitionService) ValidatePrincipalFields(nodes map[string]approval.NodeData, fields []approval.FormFieldDefinition) error {
	keys := formFieldKeySet(fields)
	check := func(node string, mode approval.SelectionMode, field *string) error {
		if !mode.RequiresFormField() {
			return nil
		}
		name := ""
		if field != nil {
			name = strings.TrimSpace(*field)
		}
		if !keys.Contains(name) {
			return fmt.Errorf("%w: %q in node %q", errUnknownPrincipalField, name, node)
		}
		return nil
	}
	for id, node := range nodes {
		if rules, ok := node.(interface {
			// GetAssignees returns the task's assignee definitions.
			GetAssignees() []approval.AssigneeDefinition
		}); ok {
			for _, r := range rules.GetAssignees() {
				if err := check(id, s.assigneeKinds[r.Kind], r.FormField); err != nil {
					return err
				}
			}
		}
		if rules, ok := node.(interface {
			// GetCCs returns the node's CC recipient definitions.
			GetCCs() []approval.CCDefinition
		}); ok {
			for _, r := range rules.GetCCs() {
				if err := check(id, s.ccKinds[r.Kind], r.FormField); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
