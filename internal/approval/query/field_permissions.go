package query

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
)

// permissionRank orders the field-permission lattice used by the viewer-scoped
// projection. The viewer's effective permission on each field is the MAXIMUM
// over every participation context they hold, ranked hidden < visible <
// editable < required.
//
// Write strength (editable / required) may only be contributed by a context
// whose form edits the write path actually accepts: the viewer's own PENDING
// task, or the applicant resubmitting a reopened instance. Every read-only
// context — a concluded or still-queued task, a CC delivery — is clamped to at
// most visible, so a viewer is never told a field is editable when no command
// would take their edit. hidden is the floor (the field is removed from the
// payload) and never grants visibility on its own.
var permissionRank = map[approval.Permission]int{
	approval.PermissionHidden:   0,
	approval.PermissionVisible:  1,
	approval.PermissionEditable: 2,
	approval.PermissionRequired: 3,
}

// strongerPermission returns whichever permission ranks higher in the lattice,
// implementing the per-field merge (max over contexts).
func strongerPermission(a, b approval.Permission) approval.Permission {
	if permissionRank[b] > permissionRank[a] {
		return b
	}

	return a
}

// resolveViewerFieldPermissions materializes the viewer's interactivity
// permission for every top-level form field, merging every participation
// context the viewer holds under the permissionRank lattice. The result carries
// an entry for each key in bundle.FormFields (table fields count as one key) so
// the client applies it verbatim; it is nil when the flow has no form fields.
//
// Fail-closed: a viewer with zero recognized contexts (own task, CC delivery,
// applicant) contributes nothing, so the hidden seed stands and they see no
// field. Reachability is guarded by IsInstanceParticipant in the handler, whose
// participant set (applicant / assignee / CC) MUST stay a subset of the contexts
// recognized here — if the two ever diverge, the failure mode is now "sees
// nothing" instead of the former "sees everything".
func resolveViewerFieldPermissions(bundle *instanceDetailBundle, userID string) map[string]approval.Permission {
	if len(bundle.FormFields) == 0 {
		return nil
	}

	keys := make([]string, len(bundle.FormFields))
	for i := range bundle.FormFields {
		keys[i] = bundle.FormFields[i].Key
	}

	nodePerms := make(map[string]map[string]approval.Permission, len(bundle.FlowNodes))
	for i := range bundle.FlowNodes {
		nodePerms[bundle.FlowNodes[i].ID] = bundle.FlowNodes[i].FieldPermissions
	}

	// Seed every field at hidden — the identity for the max merge below and the
	// fail-closed floor: a viewer with no recognized context overwrites nothing,
	// so the map stays all-hidden and they see no field.
	result := make(map[string]approval.Permission, len(keys))
	for _, k := range keys {
		result[k] = approval.PermissionHidden
	}

	// contributeNodeMap merges a node's FieldPermissions into every form field.
	// An absent node-map key defaults to visible; node-map keys with no matching
	// form field are ignored (the loop ranges over form fields only). When
	// downgrade is set, editable / required are clamped to visible (read-only
	// contexts cannot grant write); hidden stays hidden.
	contributeNodeMap := func(nodeID string, downgrade bool) {
		perms := nodePerms[nodeID]
		for _, k := range keys {
			p := approval.PermissionVisible
			if got, ok := perms[k]; ok {
				p = got
			}

			if downgrade && permissionRank[p] > permissionRank[approval.PermissionVisible] {
				p = approval.PermissionVisible
			}

			result[k] = strongerPermission(result[k], p)
		}
	}

	contributeUniform := func(p approval.Permission) {
		for _, k := range keys {
			result[k] = strongerPermission(result[k], p)
		}
	}

	// Task contexts: the viewer's own tasks. A pending task grants the node's
	// permissions at full strength — the only task context whose edits the write
	// path accepts; any other status (including queued Waiting) is read-only.
	for i := range bundle.Tasks {
		if bundle.Tasks[i].AssigneeID != userID {
			continue
		}

		contributeNodeMap(bundle.Tasks[i].NodeID, bundle.Tasks[i].Status != approval.TaskPending)
	}

	// CC contexts: instance-level manual CC (nil node) sees the whole form; a
	// node-anchored CC gets that node's map read-downgraded (historical data may
	// carry editable / required on CC nodes — treat as visible).
	for i := range bundle.CCRecords {
		cc := &bundle.CCRecords[i]
		if cc.CCUserID != userID {
			continue
		}

		if cc.NodeID == nil {
			contributeUniform(approval.PermissionVisible)
		} else {
			contributeNodeMap(*cc.NodeID, true)
		}
	}

	// Applicant context: the start node carries no permission config by design,
	// so the applicant sees the whole form — editable while the instance can be
	// resubmitted (that path accepts the full form), visible otherwise.
	if bundle.Instance.ApplicantID == userID {
		applicantPerm := approval.PermissionVisible
		if engine.InstanceStateMachine.CanTransition(bundle.Instance.Status, approval.InstanceRunning) {
			applicantPerm = approval.PermissionEditable
		}

		contributeUniform(applicantPerm)
	}

	return result
}

// stripHiddenFormData returns a copy of formData with every key the viewer may
// not see (resolved permission hidden) removed. Keys absent from perms — legacy
// form data with no schema entry — are kept, since permissions govern schema
// fields only. The input map is never mutated; a nil input yields nil.
func stripHiddenFormData(formData map[string]any, perms map[string]approval.Permission) map[string]any {
	if formData == nil {
		return nil
	}

	filtered := make(map[string]any, len(formData))
	for k, v := range formData {
		if perms[k] == approval.PermissionHidden {
			continue
		}

		filtered[k] = v
	}

	return filtered
}
