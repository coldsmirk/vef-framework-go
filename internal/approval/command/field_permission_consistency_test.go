package command_test

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
)

// TestProjectionWritePathConsistency proves the viewer projection (query
// resolveViewerFieldPermissions, surfaced by GetMyInstanceDetail) and the write
// filter (service FilterEditableFormData + required check, reached through
// ApproveTask) encode the SAME permission contract from a single node's map: a
// field the projection tells a pending approver they may edit/require is exactly
// a field the approve command accepts and merges, and every visible / hidden /
// absent field is dropped. Both sides run through their real entry points, so
// the two layers cannot silently drift.
func (s *ApproveTaskTestSuite) TestProjectionWritePathConsistency() {
	fields := []approval.FormFieldDefinition{
		{Key: "editKey", Kind: approval.FieldInput, Label: "Editable"},
		{Key: "reqKey", Kind: approval.FieldInput, Label: "Required"},
		{Key: "visKey", Kind: approval.FieldInput, Label: "Visible"},
		{Key: "hidKey", Kind: approval.FieldInput, Label: "Hidden"},
		{Key: "absentKey", Kind: approval.FieldInput, Label: "Absent"},
	}
	// absentKey is a form field but is deliberately omitted from the node map —
	// the projection defaults it to visible and the write path drops it (no
	// permission entry), so it exercises the "absent" branch on both sides.
	perms := map[string]approval.Permission{
		"editKey": approval.PermissionEditable,
		"reqKey":  approval.PermissionRequired,
		"visKey":  approval.PermissionVisible,
		"hidKey":  approval.PermissionHidden,
	}

	inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, perms, fields, nil, "consistency-approver")

	// A still-pending peer keeps the PassAll node running after the approval, so
	// no node completion / edge traversal is needed on this dedicated node.
	peer := &approval.Task{
		TenantID: "default", InstanceID: inst.ID, NodeID: task.NodeID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, task.NodeID).ID,
		AssigneeID: "consistency-peer", SortOrder: 2, Status: approval.TaskPending,
	}
	_, err := s.db.NewInsert().Model(peer).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending peer")

	// Projection side (real entry point): resolve the viewer's field permissions
	// while the task is still pending — capture before the approve consumes it.
	detailHandler := query.NewGetMyInstanceDetailHandler(s.db, service.NewTaskService())
	detail, err := detailHandler.Handle(s.ctx, query.GetMyInstanceDetailQuery{
		InstanceID: inst.ID,
		UserID:     "consistency-approver",
	})
	s.Require().NoError(err, "Pending approver should resolve instance detail")
	s.Require().Len(detail.FieldPermissions, len(fields), "Projection should carry an entry for every top-level field")

	// Write-path side (real entry point): submit a value for every key and let the
	// approve command filter / validate / merge them.
	submitted := map[string]any{
		"editKey":   "e-val",
		"reqKey":    "r-val",
		"visKey":    "v-val",
		"hidKey":    "h-val",
		"absentKey": "a-val",
	}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "consistency-approver", Name: "Approver"},
		Opinion:  "ok",
		FormData: submitted,
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Approve with all submitted fields should succeed (required field filled)")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance form data")

	// The consistency invariant: for every projected field, the two layers agree.
	// editable / required ⇒ the write path merged it; visible / hidden (and an
	// absent field defaulted to visible) ⇒ the write path dropped it.
	for key, perm := range detail.FieldPermissions {
		_, accepted := reloaded.FormData[key]

		switch perm {
		case approval.PermissionEditable, approval.PermissionRequired:
			s.Assert().Truef(accepted, "projection reports %q as %s, so the write path must accept and merge it", key, perm)
		case approval.PermissionVisible, approval.PermissionHidden:
			s.Assert().Falsef(accepted, "projection reports %q as %s, so the write path must drop it", key, perm)
		default:
			s.Failf("unexpected projected permission", "field %q resolved to unexpected permission %q", key, perm)
		}
	}

	// Pin the concrete projection outcomes too, so a future change that flips a
	// value is caught here — not only the derived agreement above (which a
	// matching change on both layers could keep green while breaking intent).
	s.Assert().Equal(approval.PermissionEditable, detail.FieldPermissions["editKey"], "editable node perm projects editable for a pending approver")
	s.Assert().Equal(approval.PermissionRequired, detail.FieldPermissions["reqKey"], "required node perm projects required")
	s.Assert().Equal(approval.PermissionVisible, detail.FieldPermissions["visKey"], "visible node perm projects visible")
	s.Assert().Equal(approval.PermissionHidden, detail.FieldPermissions["hidKey"], "hidden node perm projects hidden")
	s.Assert().Equal(approval.PermissionVisible, detail.FieldPermissions["absentKey"], "a key absent from the node map defaults to visible")
}
