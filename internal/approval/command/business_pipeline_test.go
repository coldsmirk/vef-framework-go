package command_test

import (
	"context"
	"errors"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/formeditor"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type pipelineUsers struct{}

func (pipelineUsers) ResolveUsers(context.Context, []string) (map[string]approval.UserInfo, error) {
	return map[string]approval.UserInfo{"resolved": {ID: "resolved", Name: "Reviewer"}}, nil
}

func (s *StartInstanceTestSuite) TestBusinessPipeline() {
	fields := []approval.FormFieldDefinition{{Key: "reviewer", Kind: approval.FieldInput, Label: "Reviewer", IsRequired: true}}
	failHost := errors.New("host veto")
	calls := 0

	var allocated string

	h := testBusinessHandler{
		descriptor: approval.BusinessKindDescriptor{Kind: "order", Label: "Order", FormFields: fields},
		submit: func(ctx context.Context, db orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
			calls++
			allocated = sc.InstanceID
			s.False(sc.IsResubmission(), "A start has no prior instance")
			s.Equal("raw", sc.FormData["reviewer"], "The host receives the submitted payload")

			_, err := db.NewInsert().Model(&approval.FlowCategory{TenantID: "default", Code: sc.InstanceID, Name: "Transactional business record"}).Exec(ctx)
			if err != nil {
				return nil, err
			}

			if veto, _ := sc.FormData["veto"].(bool); veto {
				return nil, failHost
			}

			return &approval.Submission{BusinessRef: "ref-" + sc.InstanceID, FormData: map[string]any{"reviewer": sc.FormData["output"], "private": "omit"}}, nil
		},
	}
	reg := testRegistry(h)
	flow := &approval.Flow{
		Code:                   "business-pipeline",
		Name:                   "Business Pipeline",
		TenantID:               "default",
		CategoryID:             s.fixture.CategoryID,
		IsActive:               true,
		IsAllInitiationAllowed: true,
		BusinessKind:           new(approval.BusinessKind("order")),
		InstanceTitleTemplate:  "Review by {{.formData.reviewer}}",
	}
	_, err := s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert the business flow")

	def := approvalFlowDef()
	def.Nodes[1].Data = mustMarshal(approval.ApprovalNodeData{Name: "Review", Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeFormField, FormField: new("reviewer")}}})
	def.Nodes = append(
		def.Nodes,
		approval.NodeDefinition{ID: "condition", Kind: approval.NodeCondition, Data: mustMarshal(approval.ConditionNodeData{Name: "Route", Branches: []approval.ConditionBranch{
			{
				ID:              "review",
				Label:           "Review",
				ConditionGroups: []approval.ConditionGroup{{Conditions: []approval.Condition{{Kind: approval.ConditionField, Subject: "reviewer", Operator: "eq", Value: "resolved"}}}},
			},
			{ID: "skip", Label: "Skip", IsDefault: true, Priority: 1},
		}})},
	)
	def.Edges[0].Target = "condition"
	def.Edges = append(
		def.Edges,
		approval.EdgeDefinition{ID: "review", Source: "condition", Target: "approval-1", SourceHandle: new("review")},
		approval.EdgeDefinition{ID: "skip", Source: "condition", Target: "end-1", SourceHandle: new("skip")},
	)
	deploy := command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), formeditor.NewParser(), reg)
	version, err := deploy.Handle(s.ctx, command.DeployFlowCmd{FlowID: flow.ID, FlowDefinition: def})
	s.Require().NoError(err, "Should validate field and scalar references against the business flow")
	_, err = command.NewPublishVersionHandler(s.db, nil, nil).Handle(s.ctx, command.PublishVersionCmd{VersionID: version.ID, OperatorID: "admin"})
	s.Require().NoError(err, "Should publish the business version")

	start := func(reg *business.Registry, eng *engine.FlowEngine, data map[string]any) (*approval.Instance, error) {
		var inst *approval.Instance

		err := s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
			var err error

			inst, err = command.NewStartInstanceHandler(s.db, eng, new(MockInstanceNoGenerator), service.NewValidationService(nil), reg, nil, nil).
				Handle(contextx.SetDB(ctx, tx), command.StartInstanceCmd{FlowCode: flow.Code, Applicant: approval.UserInfo{ID: "author"}, FormData: data, Caller: approval.SystemCaller})

			return err
		})

		return inst, err
	}
	eng := buildTestEngineWithUsers(s.db, nil, pipelineUsers{})
	s.Run("HandlerAndValidationRollBack", func() {
		for _, data := range []map[string]any{{"reviewer": "raw", "veto": true}, {"reviewer": "raw", "output": nil}} {
			_, err := start(reg, eng, data)
			s.Require().Error(err, "Veto or invalid snapshot must abort the start")
			count, err := s.db.NewSelect().Model((*approval.FlowCategory)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("code", allocated) }).Count(s.ctx)
			s.Require().NoError(err, "Should query host writes")
			s.Zero(count, "The host row must roll back")
			exists, err := s.db.NewSelect().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.PKEquals(allocated) }).Exists(s.ctx)
			s.Require().NoError(err, "Should query the instance")
			s.False(exists, "No partial instance may survive")
		}
	})
	s.Run("EngineRollback", func() {
		// A published approval node with an absent resolver simulates engine failure after insertion.
		_, err := s.db.NewUpdate().Model((*approval.FlowNodeAssignee)(nil)).Set("kind", "unregistered").Where(func(cb orm.ConditionBuilder) { cb.Equals("form_field", "reviewer") }).Exec(s.ctx)

		s.Require().NoError(err, "Should introduce an unavailable runtime resolver")
		defer func() {
			_, _ = s.db.NewUpdate().
				Model((*approval.FlowNodeAssignee)(nil)).
				Set("kind", approval.AssigneeFormField).
				Where(func(cb orm.ConditionBuilder) { cb.Equals("form_field", "reviewer") }).
				Exec(s.ctx)
		}()

		_, err = start(reg, buildTestEngineWithUsers(s.db, nil, pipelineUsers{}), map[string]any{"reviewer": "raw", "output": "resolved"})
		s.Require().Error(err, "An unavailable assignee resolver should fail engine traversal")
		exists, err := s.db.NewSelect().Model((*approval.FlowCategory)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("code", allocated) }).Exists(s.ctx)
		s.Require().NoError(err, "Should query business writes")
		s.False(exists, "Engine failure must roll back host writes")
	})
	s.Run("FrozenSnapshot", func() {
		_, err := start(nil, eng, nil)
		s.ErrorIs(err, approval.ErrBusinessKindNotRegistered, "Missing handlers fail closed")
		inst, err := start(reg, eng, map[string]any{"reviewer": "raw", "output": "resolved"})
		s.Require().NoError(err, "Should start the published flow")
		s.Equal(allocated, inst.ID, "The host and instance share the preallocated key")
		s.Equal("ref-"+allocated, *inst.BusinessRef, "The returned reference should be persisted")
		s.Equal(map[string]any{"reviewer": "resolved"}, inst.FormData, "Only the version's catalog should be retained")
		s.Equal("Review by resolved", inst.Title, "The title should use the approved snapshot")

		var task approval.Task

		err = s.db.NewSelect().Model(&task).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).Scan(s.ctx)
		s.Require().NoError(err, "The snapshot condition should route to the approval node")
		s.Equal("resolved", task.AssigneeID, "The assignee should come from the snapshot")
		_, err = s.db.NewUpdate().Model(flow).Set("business_kind", "future").WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should edit the mutable flow")

		val := service.NewValidationService(nil)
		sf, err := query.NewGetStartFormHandler(s.db, val).Handle(s.ctx, query.GetStartFormQuery{FlowCode: flow.Code, Applicant: inst.Applicant(), Caller: approval.SystemCaller})
		s.Require().NoError(err, "Should load start form")
		s.Equal(inst.BusinessKind, sf.BusinessKind, "Start form should select the deployed renderer")
		list, err := query.NewFindAvailableFlowsHandler(s.db, val).Handle(s.ctx, query.FindAvailableFlowsQuery{UserID: "author", Caller: approval.SystemCaller, Keyword: &flow.Name})
		s.Require().NoError(err, "Should list the flow")
		s.Require().Len(list.Items, 1, "The matching flow should be available")
		s.Equal(inst.BusinessKind, list.Items[0].BusinessKind, "List should use the published renderer")
	})
	_, err = s.db.NewUpdate().Model(version).Set("business_kind", nil).Set("business_binding", businessBindingForTable("old")).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should simulate a version published before kinds")
	_, err = start(reg, eng, nil)
	s.ErrorIs(err, approval.ErrBusinessKindRequired, "Legacy projections require redeployment")
	s.Equal(4, calls, "Only registered business starts may call the host")
}

func (s *ResubmitInstanceTestSuite) TestBusinessResubmit() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Returned",
		InstanceNo:    "BIZ-RESUBMIT",
		ApplicantID:   "author",
		Status:        approval.InstanceReturned,
		BusinessKind:  new(approval.BusinessKind("order")),
		BusinessRef:   new("original"),
		FormData:      map[string]any{"stale": "removed"},
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the returned business instance")

	fields := []approval.FormFieldDefinition{{Key: "title", Kind: approval.FieldInput, Label: "Title"}}

	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, fields)
	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	for _, ref := range []string{"changed", "original"} {
		reg := testRegistry(testBusinessHandler{submit: func(_ context.Context, _ orm.DB, sc *approval.SubmissionContext) (*approval.Submission, error) {
			s.True(sc.IsResubmission(), "Submit should identify a resubmission")
			s.Equal(inst.ID, sc.InstanceID, "Should reuse the instance ID")
			s.Equal("original", *sc.BusinessRef, "The reference is pinned to the instance")

			return &approval.Submission{BusinessRef: ref, FormData: map[string]any{"title": "replacement", "internal": "hidden"}}, nil
		}})
		h := command.NewResubmitInstanceHandler(s.db, buildTestEngine(s.db), service.NewValidationService(nil), service.NewInstanceService(nil), nil, reg)

		_, err = h.Handle(s.ctx, command.ResubmitInstanceCmd{InstanceID: inst.ID, Operator: inst.Applicant(), Caller: approval.SystemCaller})
		if ref != "original" {
			s.ErrorIs(err, business.ErrChangedReference, "Cannot transfer ownership on resubmit")

			continue
		}

		s.Require().NoError(err, "The pinned record should resume")

		inst = &approval.Instance{ID: inst.ID}
		s.Require().NoError(s.db.NewSelect().Model(inst).WherePK().Scan(s.ctx), "Should reload the resubmitted instance")
		s.Equal(map[string]any{"title": "replacement"}, inst.FormData, "Resubmission replaces the snapshot")
	}
}

func (s *ApproveTaskTestSuite) TestBusinessDecisions() {
	fields := []approval.FormFieldDefinition{{Key: "required", Kind: approval.FieldInput, Label: "Required"}}

	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	for _, which := range []approval.ActionType{approval.ActionApprove, approval.ActionHandle, approval.ActionReject, approval.ActionTransfer} {
		s.Run(string(which), func() {
			kind := approval.NodeApproval
			if which == approval.ActionHandle {
				kind = approval.NodeHandle
			}

			perms := map[string]approval.Permission{"required": approval.PermissionRequired}
			inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, kind, perms, fields, map[string]any{"required": "original"}, "biz-"+string(which))
			inst.BusinessKind = new(approval.BusinessKind("order"))
			_, err := s.db.NewUpdate().Model(inst).Select("business_kind").WherePK().Exec(s.ctx)
			s.Require().NoError(err, "Should configure a business instance")

			if which == approval.ActionTransfer {
				_, err = s.db.NewUpdate().
					Model((*approval.FlowNode)(nil)).
					Set("is_transfer_allowed", true).
					Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
					Exec(s.ctx)
				s.Require().NoError(err, "Should allow transfers")
			}

			calls := 0
			veto := errors.New("veto decision")
			reg := testRegistry(testBusinessHandler{action: func(ctx context.Context, db orm.DB, ac *approval.TaskActionContext) (*approval.TaskActionOutcome, error) {
				calls++

				s.Equal(which, ac.Action, "Should dispatch the actual human decision")
				s.Equal(task.ID, ac.Task.ID, "Should provide the locked task")
				s.Equal(inst.ID, ac.Instance.ID, "Should provide the locked instance")
				s.Equal(perms, ac.FieldPermissions, "Should pass the acting node permissions")
				s.Equal("opinion", ac.Opinion, "Should pass the operator opinion")
				s.Equal("raw", ac.FormData["payload"], "The business payload should reach the handler intact")

				if which == approval.ActionTransfer {
					s.Require().NotNil(ac.TransferTo, "Transfer has a recipient")
					s.Equal("other", ac.TransferTo.ID, "Should pass the resolved recipient")
				} else {
					s.Nil(ac.TransferTo, "Other decisions have no recipient")
				}

				_, err := db.NewUpdate().Model(inst).Set("title", "host edit").WherePK().Exec(ctx)
				if err != nil {
					return nil, err
				}

				if calls == 1 {
					return nil, veto
				}

				return &approval.TaskActionOutcome{FormData: map[string]any{"required": ""}}, nil
			}})
			svc := service.NewTaskService(service.WithBusinessRegistry(reg))
			valid := service.NewValidationService(nil)
			act := func(ctx context.Context) error {
				who := task.Assignee()

				raw := map[string]any{"payload": "raw"}
				switch which {
				case approval.ActionApprove, approval.ActionHandle:
					_, err := command.NewApproveTaskHandler(s.db, svc, nil, valid, nil).
						Handle(ctx, command.ApproveTaskCmd{TaskID: task.ID, Operator: who, Opinion: "opinion", FormData: raw, Caller: approval.SystemCaller})

					return err
				case approval.ActionReject:
					_, err := command.NewRejectTaskHandler(s.db, svc, nil, valid, nil).
						Handle(ctx, command.RejectTaskCmd{TaskID: task.ID, Operator: who, Opinion: "opinion", FormData: raw, Caller: approval.SystemCaller})

					return err
				default:
					_, err := command.NewTransferTaskHandler(s.db, svc, valid, nil, nil, nil).
						Handle(ctx, command.TransferTaskCmd{TaskID: task.ID, TransferToID: "other", Operator: who, Opinion: "opinion", FormData: raw, Caller: approval.SystemCaller})

					return err
				}
			}
			run := func() error {
				return s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error { return act(contextx.SetDB(ctx, tx)) })
			}
			s.ErrorIs(run(), veto, "A handler veto must fail the entire action")
			s.Require().NoError(s.db.NewSelect().Model(inst).WherePK().Scan(s.ctx), "Should reload the instance after veto")
			s.Equal("Form Field Test", inst.Title, "The host's write should roll back")

			if which == approval.ActionApprove || which == approval.ActionHandle {
				s.Error(run(), "Required permissions must be checked against the new snapshot")
			}

			s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should reload the held task")
			s.Equal(approval.TaskPending, task.Status, "A failed decision must leave the task pending")
		})
	}
}

func (s *ApproveTaskTestSuite) TestBusinessViews() {
	fields := []approval.FormFieldDefinition{{Key: "secret", Kind: approval.FieldInput, Label: "Secret"}}

	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	perms := map[string]approval.Permission{"secret": approval.PermissionHidden}
	inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, perms, fields, map[string]any{"secret": "restricted"}, "biz-viewer")
	inst.BusinessKind = new(approval.BusinessKind("order"))
	_, err := s.db.NewUpdate().Model(inst).Select("business_kind").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should bind the instance view")

	var seen *approval.ViewContext

	failure := errors.New("view unavailable")
	reg := testRegistry(testBusinessHandler{view: func(_ context.Context, _ orm.DB, vc *approval.ViewContext) (any, error) {
		seen = vc
		if vc.Viewer.Name == "failed" {
			return nil, failure
		}

		return vc.Viewer.ID, nil
	}})
	globals := service.NewFormGlobalsService(nil)
	my := query.NewGetMyInstanceDetailHandler(s.db, reg, globals)
	viewer := task.Assignee()
	req := query.GetMyInstanceDetailQuery{InstanceID: inst.ID, Viewer: viewer}
	detail, err := my.Handle(s.ctx, req)
	s.Require().NoError(err, "A participant may read their business view")
	s.Equal(inst.BusinessKind, detail.Instance.BusinessKind, "Detail should carry the pinned renderer")
	s.Equal(viewer.ID, detail.BusinessView, "Should return the host view")
	s.Equal(perms, seen.FieldPermissions, "View and instance masking must share effective permissions")
	s.True(seen.Participation.Has(approval.ParticipantAssignee), "The handler should see the viewer's role")
	s.True(seen.Participation.CanAct(task.ID), "The viewer should see their actionable task")

	req.Viewer.Name = "failed"
	_, err = my.Handle(s.ctx, req)
	s.ErrorIs(err, failure, "View failures should fail the detail request")

	req.Viewer.ID = "stranger"
	seen = nil
	_, err = my.Handle(s.ctx, req)
	s.ErrorIs(err, approval.ErrAccessDenied, "An outsider cannot request the host view")
	s.Nil(seen, "Deny access before running host code")
	admin := query.NewGetAdminInstanceDetailHandler(s.db, globals, reg)
	full, err := admin.Handle(s.ctx, query.GetAdminInstanceDetailQuery{InstanceID: inst.ID, Viewer: approval.UserInfo{ID: "admin"}, Caller: approval.SystemCaller})
	s.Require().NoError(err, "An authorized administrator may read the full business view")
	s.Equal("admin", full.BusinessView, "The host receives the administrative viewer")
	s.Nil(seen.FieldPermissions, "Administrative view is unrestricted")
	s.Nil(seen.Participation, "Administrative access is separate from participation")
}
