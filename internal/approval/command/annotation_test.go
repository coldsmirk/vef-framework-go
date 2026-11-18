package command_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func requireNoteStatus(t *testing.T, ctx context.Context, db orm.DB, id string, status approval.AnnotationStatus) {
	t.Helper()

	a := approval.Annotation{ID: id}
	require.NoError(t, db.NewSelect().Model(&a).WherePK().Scan(ctx), "Should reload the annotation")
	require.Equal(t, status, a.Status, "The annotation should have followed the seat lifecycle")

	if status == approval.AnnotationPublished {
		require.NotNil(t, a.PublishedAt, "Published notes have a publication time")
	} else {
		require.Nil(t, a.PublishedAt, "Unpublished notes have no publication time")
	}
}

type anchorHandler struct {
	testBusinessHandler

	validate func(*approval.AnchorContext) error
}

func (h anchorHandler) ValidateAnchor(_ context.Context, _ orm.DB, ac *approval.AnchorContext) error {
	return h.validate(ac)
}

func (s *ApproveTaskTestSuite) TestAnnotationDrafts() {
	fields := []approval.FormFieldDefinition{
		{Key: "title", Kind: approval.FieldInput},
		{Key: "secret", Kind: approval.FieldInput},
		{Key: "lines", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "price", Kind: approval.FieldNumber}}},
	}

	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, map[string]approval.Permission{"secret": approval.PermissionHidden}, fields, nil, "routed-writer")
	editor := service.NewAnnotationEditor(service.NewTaskService(), service.NewInstanceService(nil), nil)
	otherTenant := approval.CallerContext{TenantID: "outside"}
	draft := approval.SaveAnnotationInput{TaskID: task.ID, Content: "Check the price", Operator: task.Assignee(), Caller: otherTenant}
	s.Run("RoutedOwner", func() {
		in := draft
		in.Anchor = json.RawMessage(`{"field":"lines","row":0,"column":"price"}`)
		a, err := editor.Save(s.ctx, s.db, in)
		s.Require().NoError(err, "A cross-tenant routed task owner can save a draft")
		s.Equal(&task.VisitID, a.VisitID, "The draft binds its exact visit")
		s.Equal(approval.AudienceParticipants, a.Audience.Scope, "Audience defaults to all participants")
		s.False(a.CreatedAt.IsZero(), "The returned view includes its persisted creation time")
		s.Equal(task.Assignee(), a.Author, "The author is snapshotted")
		in.ID = a.ID
		in.Content = "Updated"
		a, err = editor.Save(s.ctx, s.db, in)
		s.Require().NoError(err, "The routed owner can edit their draft")
		s.Equal("Updated", a.Content, "Save returns the revised view")

		outsider := in
		outsider.Operator.ID = "stranger"
		_, err = editor.Save(s.ctx, s.db, outsider)
		s.ErrorIs(err, approval.ErrAnnotationNotFound, "An unrouted foreign editor cannot discover a draft")
		err = editor.Delete(s.ctx, s.db, approval.DeleteAnnotationInput{ID: a.ID, Operator: outsider.Operator, Caller: otherTenant})
		s.ErrorIs(err, approval.ErrAnnotationNotFound, "An unrouted foreign deleter cannot discover a draft")

		outsider.ID = ""
		_, err = editor.Save(s.ctx, s.db, outsider)
		s.ErrorIs(err, approval.ErrAnnotationNotFound, "Unrouted users cannot start an annotation across tenants")
		err = editor.Delete(s.ctx, s.db, approval.DeleteAnnotationInput{ID: a.ID, Operator: in.Operator, Caller: otherTenant})
		s.Require().NoError(err, "The routed owner can delete a valid draft")
		_, err = editor.Save(s.ctx, s.db, in)
		s.ErrorIs(err, approval.ErrAnnotationNotFound, "Deleted drafts no longer exist")
	})
	s.Run("InvalidAnchorsAndAudiences", func() {
		for _, anchor := range []string{`{"field":"missing"}`, `{"field":"secret"}`, `{"field":"title","row":0}`, `{"field":"lines","row":-1}`, `{"field":"lines","column":"absent"}`, `{"field":"title","junk":1}`} {
			in := draft
			in.Anchor = json.RawMessage(anchor)
			_, err := editor.Save(s.ctx, s.db, in)
			s.ErrorIs(err, approval.ErrAnnotationAnchorInvalid, "Should reject invalid anchor %s", anchor)
		}

		for _, aud := range []approval.AnnotationAudience{{Scope: "everyone"}, {Scope: approval.AudienceParticipants, UserIDs: []string{"x"}}, {Scope: approval.AudienceNodes, NodeIDs: []string{"foreign-node"}}, {Scope: approval.AudienceUsers}, {Scope: approval.AudienceUsers, UserIDs: []string{" "}}} {
			in := draft
			in.Audience = &aud
			_, err := editor.Save(s.ctx, s.db, in)
			s.ErrorIs(err, approval.ErrAnnotationAudienceInvalid, "Scope and audience selectors must agree")
		}

		in := draft
		in.Content = strings.Repeat("a", 4001)
		_, err := editor.Save(s.ctx, s.db, in)
		s.Error(err, "Draft content must fit in 4000 runes")
	})
	s.Run("CurrentHolder", func() {
		in := draft
		in.Operator.ID = "another"
		in.Caller = approval.SystemCaller
		_, err := editor.Save(s.ctx, s.db, in)
		s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "Only the current assignee may annotate")
		a, err := editor.Save(s.ctx, s.db, draft)
		s.Require().NoError(err, "The pending owner can annotate")
		_, err = s.db.NewUpdate().Model(task).Set("status", approval.TaskWaiting).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should queue the seat")
		_, err = editor.Save(s.ctx, s.db, draft)
		s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "A queued assignee cannot create drafts")
		err = editor.Delete(s.ctx, s.db, approval.DeleteAnnotationInput{ID: a.ID, Caller: draft.Caller, Operator: draft.Operator})
		s.ErrorIs(err, approval.ErrAnnotationNotEditable, "A queued seat cannot edit its old drafts")
		_, err = s.db.NewUpdate().Model(task).Set("status", approval.TaskPending).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should restore the seat")
		_, err = s.db.NewUpdate().Model(inst).Set("current_node_id", nil).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should move off this node")
		_, err = editor.Save(s.ctx, s.db, draft)
		s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "A task on a former node cannot create drafts")
		_, err = s.db.NewUpdate().Model(inst).Set("current_node_id", task.NodeID).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should restore the current node")
	})
	s.Run("BusinessAnchors", func() {
		_, err := s.db.NewUpdate().Model(inst).Set("business_kind", "order").WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should bind a business handler")

		in := draft
		in.Anchor = json.RawMessage(`{"offset":12}`)
		edit := service.NewAnnotationEditor(service.NewTaskService(), service.NewInstanceService(nil), testRegistry(testBusinessHandler{}))
		_, err = edit.Save(s.ctx, s.db, in)
		s.ErrorIs(err, approval.ErrAnnotationAnchorInvalid, "An opaque anchor needs a capable handler")

		veto := errors.New("unknown offset")
		edit = service.NewAnnotationEditor(service.NewTaskService(), service.NewInstanceService(nil), testRegistry(anchorHandler{validate: func(ac *approval.AnchorContext) error {
			s.Equal(inst.ID, ac.Instance.ID, "Handler should receive the locked instance")
			s.Equal(task.ID, ac.Task.ID, "Handler should receive the held task")
			s.Equal(in.Operator, ac.Author, "Handler should receive the author")

			if string(ac.Anchor) == `{"offset":12}` {
				return veto
			}

			return nil
		}}))
		_, err = edit.Save(s.ctx, s.db, in)
		s.ErrorIs(err, approval.ErrAnnotationAnchorInvalid, "A host veto should surface the anchor code")
		s.ErrorIs(err, veto, "Preserve the host cause")

		in.Anchor = json.RawMessage(`{"offset":1}`)
		_, err = edit.Save(s.ctx, s.db, in)
		s.NoError(err, "The host can accept a business-specific anchor")
	})
}

func (s *EventOrderTestSuite) TestAnnotationViews() {
	inst := s.startOn("order-any-flow", "applicant")
	taskID := s.pendingTaskID(inst.ID, "any-1")
	peerID := s.pendingTaskID(inst.ID, "any-2")
	editor := service.NewAnnotationEditor(service.NewTaskService(), service.NewInstanceService(nil), nil)
	_, err := s.db.NewUpdate().Model((*approval.Task)(nil)).Set("delegator_id", "delegator").Where(func(cb orm.ConditionBuilder) { cb.PKEquals(peerID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should bind a delegated participant")

	byContent := map[string]string{}
	save := func(task, user, content string, aud *approval.AnnotationAudience) {
		a, err := editor.Save(s.ctx, s.db, approval.SaveAnnotationInput{TaskID: task, Content: content, Audience: aud, Operator: approval.UserInfo{ID: user}, Caller: approval.SystemCaller})
		s.Require().NoError(err, "Should save the note")

		byContent[content] = a.ID
	}
	save(taskID, "any-1", "all", nil)
	save(taskID, "any-1", "applicant", &approval.AnnotationAudience{Scope: approval.AudienceApplicant})
	save(taskID, "any-1", "node", &approval.AnnotationAudience{Scope: approval.AudienceNodes, NodeIDs: []string{*inst.CurrentNodeID}})
	save(taskID, "any-1", "user", &approval.AnnotationAudience{Scope: approval.AudienceUsers, UserIDs: []string{"delegator"}})
	save(peerID, "any-2", "peer draft", nil)

	my := query.NewGetMyInstanceDetailHandler(s.db, nil, service.NewFormGlobalsService(nil))
	admin := query.NewGetAdminInstanceDetailHandler(s.db, service.NewFormGlobalsService(nil), nil)
	names := func(rows []approval.AnnotationView) []string {
		out := make([]string, 0, len(rows))
		for _, a := range rows {
			s.NotEmpty(a.NodeName, "Every note is enriched with its node name")
			out = append(out, a.Content)
		}

		return out
	}
	view := func(user string) []string {
		d, err := my.Handle(s.ctx, query.GetMyInstanceDetailQuery{InstanceID: inst.ID, Viewer: approval.UserInfo{ID: user}})
		s.Require().NoError(err, "A participant can view the instance")

		return names(d.Annotations)
	}
	s.ElementsMatch([]string{"peer draft"}, view("any-2"), "Other authors' drafts must stay private")
	d, err := admin.Handle(s.ctx, query.GetAdminInstanceDetailQuery{InstanceID: inst.ID, Caller: approval.SystemCaller})
	s.Require().NoError(err, "Admin can read the detail")
	s.Empty(d.Annotations, "Admin view contains no drafts")
	_, err = s.approve.Handle(s.ctx, command.ApproveTaskCmd{TaskID: taskID, Operator: approval.UserInfo{ID: "any-1"}, Caller: approval.SystemCaller})
	s.Require().NoError(err, "The first decision should cancel the peer")
	requireNoteStatus(s.T(), s.ctx, s.db, byContent["peer draft"], approval.AnnotationDiscarded)
	s.ElementsMatch([]string{"all", "node", "peer draft"}, view("any-2"), "The owner can recover their discarded draft")
	s.ElementsMatch([]string{"all", "applicant"}, view("applicant"), "Applicant sees the applicant audience")
	s.ElementsMatch([]string{"all", "node", "user"}, view("delegator"), "Delegation counts as node participation, including historical tasks")
	s.ElementsMatch([]string{"all", "node", "user", "applicant"}, view("any-1"), "Author can read all their published notes")
	d, err = admin.Handle(s.ctx, query.GetAdminInstanceDetailQuery{InstanceID: inst.ID, Caller: approval.SystemCaller})
	s.Require().NoError(err, "Admin can view publication")
	s.Len(d.Annotations, 4, "Admin sees all and only published notes")
	_, err = s.db.NewUpdate().Model((*approval.Task)(nil)).Set("assignee_id", "new-owner").Where(func(cb orm.ConditionBuilder) { cb.PKEquals(peerID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should remove the former assignee's only participation")
	_, err = my.Handle(s.ctx, query.GetMyInstanceDetailQuery{InstanceID: inst.ID, Viewer: approval.UserInfo{ID: "any-2"}})
	s.ErrorIs(err, approval.ErrAccessDenied, "A discarded draft cannot grant its author access to the instance")
	s.ElementsMatch([]string{"all", "node"}, view("new-owner"), "A later participant on an audience node sees its published notes")
	err = editor.Delete(s.ctx, s.db, approval.DeleteAnnotationInput{ID: byContent["all"], Caller: approval.SystemCaller, Operator: approval.UserInfo{ID: "any-1"}})
	s.ErrorIs(err, approval.ErrAnnotationNotEditable, "Published notes are immutable")
}

func seedTaskDraft(t *testing.T, ctx context.Context, db orm.DB, task *approval.Task) string {
	t.Helper()

	a := &approval.Annotation{
		InstanceID: task.InstanceID,
		TenantID:   task.TenantID,
		NodeID:     task.NodeID,
		TaskID:     &task.ID,
		VisitID:    &task.VisitID,
		Author:     task.Assignee(),
		Status:     approval.AnnotationDraft,
		Audience:   approval.AnnotationAudience{Scope: approval.AudienceParticipants},
		Content:    "Unfinished",
	}
	_, err := db.NewInsert().Model(a).Exec(ctx)
	require.NoError(t, err, "Should persist the held seat's draft")

	return a.ID
}

func (s *EventOrderTestSuite) TestResponseDraftRules() {
	for _, end := range []string{"withdraw", "terminate"} {
		s.Run(end, func() {
			inst := s.startOn("order-any-flow", "applicant")
			taskID := s.pendingTaskID(inst.ID, "any-1")
			tasks := service.NewTaskService()
			instances := service.NewInstanceService(nil)
			editor := service.NewAnnotationEditor(tasks, instances, nil)
			ctx, db := s.ctx, s.db
			author := approval.UserInfo{ID: "any-1"}

			setPublishedFormFields(s.T(), ctx, db, inst.FlowVersionID, []approval.FormFieldDefinition{{Key: "amount", Kind: approval.FieldNumber}})
			defer setPublishedFormFields(s.T(), ctx, db, inst.FlowVersionID, nil)

			parent, err := editor.Save(
				ctx,
				db,
				approval.SaveAnnotationInput{
					TaskID:   taskID,
					Operator: author,
					Anchor:   json.RawMessage(`{"field":"amount"}`),
					Content:  "Clarify",
					Audience: &approval.AnnotationAudience{Scope: approval.AudienceApplicant},
					Caller:   approval.SystemCaller,
				},
			)
			s.Require().NoError(err, "Should save a note for the applicant")
			hidden, err := editor.Save(
				ctx,
				db,
				approval.SaveAnnotationInput{
					TaskID:   taskID,
					Operator: author,
					Content:  "Private",
					Audience: &approval.AnnotationAudience{Scope: approval.AudienceUsers, UserIDs: []string{"any-2"}},
					Caller:   approval.SystemCaller,
				},
			)
			s.Require().NoError(err, "Should save a restricted note")

			response := approval.SaveAnnotationInput{
				ParentID:   parent.ID,
				Resolution: new(approval.ResolutionDeclined),
				Content:    "Cannot change",
				Operator:   inst.Applicant(),
				Caller:     approval.SystemCaller,
			}
			_, err = editor.Save(ctx, db, response)
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "Unpublished parent drafts cannot receive a response")
			s.Require().NoError(tasks.PublishDrafts(ctx, db, inst, &taskID, author), "Should publish both parent notes")
			_, err = editor.Save(ctx, db, response)
			s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "Only a returned instance accepts responses")
			_, err = db.NewUpdate().Model(inst).Set("status", approval.InstanceReturned).Set("current_node_id", s.anyOf.NodeIDs["start-1"]).WherePK().Exec(ctx)
			s.Require().NoError(err, "Should establish a returned instance")

			in := response
			in.ParentID = hidden.ID
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "The applicant cannot respond to a hidden parent")

			in = response
			in.Operator = author
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "Only the applicant can reply")

			in.Operator.ID = "unrelated"
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "An unrelated caller cannot discover a private parent by replying")

			in = response
			in.Content = ""
			_, err = editor.Save(ctx, db, in)
			s.Error(err, "Declining a suggestion requires an explanation")

			in = response
			in.Resolution = nil
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationNotAllowed, "Responses require a resolution")

			in = response
			in.Audience = &approval.AnnotationAudience{Scope: approval.AudienceParticipants}
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationAudienceInvalid, "A response cannot widen its audience")

			in = response
			in.Anchor = json.RawMessage(`{"field":"other"}`)
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationAnchorInvalid, "A response cannot redefine its anchor")

			in = response
			_, err = db.NewInsert().Model(&approval.CCRecord{InstanceID: inst.ID, CCUserID: inst.ApplicantID, CCUserName: inst.ApplicantName}).Exec(ctx)
			s.Require().NoError(err, "Should route a copy of the instance to its applicant")

			in.Caller = approval.CallerContext{TenantID: "elsewhere"}
			_, err = editor.Save(ctx, db, in)
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "Applicants respond only in the instance tenant")
			note, err := editor.Save(ctx, db, response)
			s.Require().NoError(err, "The applicant can respond to the visible parent")
			s.JSONEq(`{"field":"amount"}`, string(note.Anchor), "Responses inherit the parent's field anchor")

			foreign := response
			foreign.ID = note.ID
			foreign.Caller = approval.CallerContext{TenantID: "elsewhere"}
			_, err = editor.Save(ctx, db, foreign)
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "Response edits stay within the instance tenant")
			err = editor.Delete(ctx, db, approval.DeleteAnnotationInput{ID: note.ID, Operator: inst.Applicant(), Caller: foreign.Caller})
			s.ErrorIs(err, approval.ErrAnnotationNotFound, "Response deletes stay within the instance tenant")
			s.Equal(parent.Audience, note.Audience, "Responses inherit the parent's audience")
			// Publish one response to check inheritance, and leave a second in draft.
			s.Require().NoError(tasks.PublishDrafts(ctx, db, inst, nil, inst.Applicant()), "Should publish the first response")

			my := query.NewGetMyInstanceDetailHandler(db, nil, service.NewFormGlobalsService(nil))
			for _, tc := range []struct {
				user    string
				visible bool
			}{{"any-1", true}, {"any-2", false}, {"applicant", true}} {
				d, err := my.Handle(ctx, query.GetMyInstanceDetailQuery{InstanceID: inst.ID, Viewer: approval.UserInfo{ID: tc.user}})
				s.Require().NoError(err, "The participant may view the instance")

				found := false
				for _, a := range d.Annotations {
					found = found || a.ID == note.ID
				}

				s.Equal(tc.visible, found, "The parent author and applicant share the response")
			}

			draft, err := editor.Save(ctx, db, response)
			s.Require().NoError(err, "Should create the remaining response draft")

			if end == "withdraw" {
				_, err = command.NewWithdrawInstanceHandler(db, tasks, instances).
					Handle(ctx, command.WithdrawInstanceCmd{InstanceID: inst.ID, Operator: inst.Applicant(), Caller: approval.SystemCaller})
			} else {
				_, err = command.NewTerminateInstanceHandler(db, tasks, instances).
					Handle(ctx, command.TerminateInstanceCmd{InstanceID: inst.ID, Operator: approval.UserInfo{ID: "admin"}, Caller: approval.SystemCaller})
			}

			s.Require().NoError(err, "Closing the returned instance should succeed")
			requireNoteStatus(s.T(), ctx, db, draft.ID, approval.AnnotationDiscarded)
			requireNoteStatus(s.T(), ctx, db, note.ID, approval.AnnotationPublished)
			err = editor.Delete(ctx, db, approval.DeleteAnnotationInput{ID: draft.ID, Operator: inst.Applicant(), Caller: approval.SystemCaller})
			s.ErrorIs(err, approval.ErrAnnotationNotEditable, "Discarded responses are immutable")
		})
	}
}
