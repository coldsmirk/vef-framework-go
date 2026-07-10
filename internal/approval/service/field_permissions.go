package service

import (
	"cmp"
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// ValidateFieldPermissions cross-checks every node's FieldPermissions matrix
// against the version's parsed form fields: every value must be a defined
// Permission, every key must name a top-level form field (a flow with no
// form has zero fields, so any entry is automatically a dangling reference),
// and CC nodes are further restricted to visible/hidden — the same subset
// the designer's CcFieldPermission type offers, since a CC node only
// observes the form and can neither unlock editing nor demand a value.
//
// The structural per-node checks (enum validity of every other node field)
// live in validateNodeConfig; this pass adds what only the form schema can
// answer, so — like ValidateConditionAggregates — it runs at deploy, the one
// place the flow definition and the form fields meet.
func (*FlowDefinitionService) ValidateFieldPermissions(nodeData map[string]approval.NodeData, fields []approval.FormFieldDefinition) error {
	fieldKeys := formFieldKeySet(fields)

	for nodeID, data := range nodeData {
		permissions, isCC := fieldPermissionsOf(data)

		hasRequired := false
		for key, perm := range permissions {
			if err := validateFieldPermissionEntry(nodeID, key, perm, isCC, fieldKeys); err != nil {
				return err
			}

			if perm == approval.PermissionRequired {
				hasRequired = true
			}
		}

		// The write path enforces "a node passes ⇒ its required fields are filled"
		// on approve / handle, but the timeout scanner's auto_pass finishes tasks
		// without that check — so a required field permission on an auto_pass node
		// is an unenforceable contract and must be unrepresentable. Validated
		// against the resolved timeout action (deploy normalization defaults an
		// omitted value), mirroring the sibling checks in validateNodeConfig.
		if hasRequired && resolvedTimeoutAction(data) == approval.TimeoutActionAutoPass {
			return fmt.Errorf("%w: node %q", errRequiredPermissionAutoPass, nodeID)
		}
	}

	return nil
}

// resolvedTimeoutAction returns a task node's timeout action with deploy
// normalization applied (an omitted value resolves to DefaultTimeoutAction, as
// ApplyTo does). Node kinds with no timeout surface — CC, start, end, condition
// — return the empty action.
func resolvedTimeoutAction(data approval.NodeData) approval.TimeoutAction {
	switch typed := data.(type) {
	case *approval.ApprovalNodeData:
		return cmp.Or(typed.TimeoutAction, approval.DefaultTimeoutAction)
	case *approval.HandleNodeData:
		return cmp.Or(typed.TimeoutAction, approval.DefaultTimeoutAction)
	default:
		return ""
	}
}

// fieldPermissionsOf extracts a node's field-permission matrix and reports
// whether the node is a CC node. Approval and handle nodes share
// TaskNodeData.FieldPermissions; CC nodes carry their own field of the same
// shape. Start, end, and condition nodes have no form-visible surface — their
// data types declare no FieldPermissions field at all — so they fall through
// to a nil map, which the caller's loop naturally skips.
func fieldPermissionsOf(data approval.NodeData) (permissions map[string]approval.Permission, isCC bool) {
	switch typed := data.(type) {
	case *approval.ApprovalNodeData:
		return typed.FieldPermissions, false
	case *approval.HandleNodeData:
		return typed.FieldPermissions, false
	case *approval.CCNodeData:
		return typed.FieldPermissions, true
	default:
		return nil, false
	}
}

// validateFieldPermissionEntry checks one (key, permission) pair from a
// node's FieldPermissions matrix against the enum, the form schema, and —
// for CC nodes — the narrowed visible/hidden vocabulary.
func validateFieldPermissionEntry(nodeID, key string, perm approval.Permission, isCC bool, fieldKeys collections.Set[string]) error {
	if !perm.IsValid() {
		return fmt.Errorf("%w: %q for field %q in node %q", errInvalidFieldPermission, perm, key, nodeID)
	}

	if !fieldKeys.Contains(key) {
		return fmt.Errorf("%w: %q in node %q", errFieldPermissionKeyUnknown, key, nodeID)
	}

	if isCC && perm != approval.PermissionVisible && perm != approval.PermissionHidden {
		return fmt.Errorf("%w: %q for field %q in node %q", errCCFieldPermissionNotAllowed, perm, key, nodeID)
	}

	return nil
}

// formFieldKeySet collects the top-level form field keys as a lookup set.
// Column keys inside table fields are a separate namespace (see
// tableFieldIndex in condition_aggregates.go) — FieldPermissions only ever
// keys off top-level fields, mirroring the designer's flat permission table.
func formFieldKeySet(fields []approval.FormFieldDefinition) collections.Set[string] {
	keys := collections.NewHashSetWithCapacity[string](len(fields))
	for _, field := range fields {
		keys.Add(field.Key)
	}

	return keys
}
