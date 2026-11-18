package command_test

import (
	"context"
	"errors"
	"sync"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/voiding"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type projectionVoider struct {
	testBusinessHandler

	order     []string
	veto      error
	from, to  approval.InstanceStatus
	projected string
}

func (h *projectionVoider) PlanVoid(context.Context, orm.DB, *approval.VoidContext) (*approval.VoidPlan, error) {
	h.order = append(h.order, "plan")

	return &approval.VoidPlan{Reversals: []string{"Release funds"}, FollowUps: []string{"Notify bank"}}, nil
}

func (h *projectionVoider) Void(ctx context.Context, db orm.DB, vc *approval.VoidContext) error {
	h.order = append(h.order, string(vc.Instance.Status))
	_, err := db.NewRaw("UPDATE biz_void_projection SET released = 1 WHERE id = ?", *vc.Instance.BusinessRef).Exec(ctx)

	return err
}

func (*projectionVoider) OnInstanceCreated(context.Context, orm.DB, *approval.Instance) error {
	return nil
}

func (h *projectionVoider) OnInstanceTransition(ctx context.Context, db orm.DB, inst *approval.Instance, from, to approval.InstanceStatus) error {
	h.order = append(h.order, "hook")

	h.from, h.to = from, to
	if err := db.NewRaw("SELECT approval_status FROM biz_void_projection WHERE id = ? AND released = 1", *inst.BusinessRef).Scan(ctx, &h.projected); err != nil {
		return err
	}

	return h.veto
}

func (s *EventOrderTestSuite) TestVoidProjectionAndEvent() {
	target := s.startOn("order-auto-flow", "owner")
	flow := &approval.Flow{ID: target.FlowID, VoidPolicy: approval.VoidDirect}
	_, err := s.db.NewUpdate().Model(flow).Select("void_policy").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should enable direct void on the live flow")

	target.BusinessRef = new("payment")
	target.BusinessKind = new(approval.BusinessKind("order"))
	_, err = s.db.NewUpdate().Model(target).Select("business_ref", "business_kind").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should seed the business reference")
	_, err = s.db.NewRaw(`CREATE TABLE biz_void_projection (
 id VARCHAR(64) PRIMARY KEY, released INTEGER NOT NULL, approval_status VARCHAR(32),
 apv_instance_id VARCHAR(32), apv_started_at TIMESTAMP, apv_finished_at TIMESTAMP)`).Exec(s.ctx)
	s.Require().NoError(err, "Should create the host payment table")
	_, err = s.db.NewRaw("INSERT INTO biz_void_projection (id,released) VALUES (?, 0)", *target.BusinessRef).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the payment")

	projection := businessBindingForTable("biz_void_projection")
	projection.StatusMapping = map[approval.InstanceStatus]string{approval.InstanceVoided: "reversed"}
	projector := binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil)
	version := &approval.FlowVersion{ID: target.FlowVersionID, BusinessBinding: projection, BusinessKind: target.BusinessKind}
	s.Require().NoError(projector.Bind(s.ctx, s.db, flow, version, target), "Approved instance should own the projection")
	initial := &approval.BusinessProjection{ID: *target.BusinessProjectionID}
	s.Require().NoError(s.db.NewSelect().Model(initial).WherePK().Scan(s.ctx), "Should load the projection as persisted at approval")

	host := new(projectionVoider)
	rt, err := voiding.NewRuntime([]approval.BusinessHandler{host}, service.NewFlowDefinitionService(), projector, nil, s.bus)
	s.Require().NoError(err, "The host kind and request kind should register together")
	h := wrapWithBusAndDB(s.db, s.bus, command.NewVoidInstanceHandler(s.db, rt.Core))
	in := approval.VoidInstanceInput{
		InstanceID:  target.ID,
		Reason:      "Duplicate payment",
		Operator:    approval.UserInfo{ID: "treasurer", Name: "Treasurer"},
		Attachments: []string{"receipt"},
		Caller:      approval.SystemCaller,
	}
	run := func() error {
		return s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
			_, err := h.Handle(contextx.SetDB(ctx, tx), command.VoidInstanceCmd{VoidInstanceInput: in})

			return err
		})
	}

	s.bus.Reset()

	veto := errors.New("host veto")
	host.veto = veto
	s.ErrorIs(run(), veto, "A hook can veto after reversal and projection")

	var released int
	s.Require().NoError(s.db.NewRaw("SELECT released FROM biz_void_projection WHERE id = ?", *target.BusinessRef).Scan(s.ctx, &released), "Should inspect host state")
	s.Zero(released, "The host write must roll back with the failed transition")
	s.Empty(s.bus.Captured(), "The veto publishes no event")

	host.order = nil
	host.veto = nil

	s.Require().NoError(run(), "The same target can be retried")
	s.Equal([]string{"plan", "approved", "hook"}, host.order, "Plan and reversal precede the status transition and scoped hook")
	s.Equal(approval.InstanceApproved, host.from, "Hook observes the original state")
	s.Equal(approval.InstanceVoided, host.to, "Hook observes the voided state")
	s.Equal("reversed", host.projected, "Hook observes the released payment and projected status in the transaction")
	s.requireEmittedInOrder(approval.EventTypeInstanceVoided)
	s.Empty(s.bus.CapturedByType(approval.EventTypeInstanceCompleted), "Void must not repeat the original completion event")
	ev := s.bus.CapturedByType(approval.EventTypeInstanceVoided)[0].(*approval.InstanceVoidedEvent)
	s.Equal(in.Operator, ev.Operator, "Event identifies the direct voider")
	s.Equal(in.Reason, ev.Reason, "Event records the reason")
	s.Nil(ev.RequestInstanceID, "Direct void has no request")
	s.Equal([]string{"Notify bank"}, ev.FollowUps, "The event includes manual follow-ups")
	s.ErrorIs(run(), approval.ErrInstanceNotVoidable, "Serial retries cannot double-reverse the same target")

	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", target.ID).Equals("action", approval.ActionVoid)
	}).Scan(s.ctx), "Should load reversal logs")
	s.Require().Len(logs, 1, "One successful reversal has one log")
	s.Equal(in.Operator, logs[0].Operator(), "Action log identifies the operator")
	s.Equal(in.Attachments, logs[0].Attachments, "Action log retains the supplied evidence")

	var records []approval.VoidRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", target.ID) }).Scan(s.ctx), "Should load the record")
	s.Require().Len(records, 1, "A retry does not duplicate the reversal record")
	s.Equal([]string{"Release funds"}, records[0].Reversals, "Record pins the executed plan")

	var p approval.BusinessProjection

	p.ID = *target.BusinessProjectionID
	s.Require().NoError(s.db.NewSelect().Model(&p).WherePK().Scan(s.ctx), "Should load the projection")
	s.True(p.DesiredFinishedAt.Equal(*initial.DesiredFinishedAt), "Projection keeps the approval completion time")
	metrics, err := query.NewGetMetricsHandler(s.db).Handle(s.ctx, query.GetMetricsQuery{TenantID: target.TenantID})
	s.Require().NoError(err, "Should read metrics after voiding")
	s.Equal(1, metrics.InstanceCounts["voided"], "Metrics include voided instances")
	s.GreaterOrEqual(metrics.AvgCompletionSeconds, float64(0), "Average includes the voided instance's approval duration")
	next := s.startOn("order-auto-flow", "owner")
	next.BusinessRef = target.BusinessRef
	s.Require().NoError(projector.Bind(s.ctx, s.db, flow, version, next), "A new instance can take over a voided owner")
	s.Require().NoError(s.db.NewSelect().Model(&p).WherePK().Scan(s.ctx), "Should reload the newly claimed projection")
	s.Equal(next.ID, p.OwnerInstanceID, "New instance owns the same business target")
}

func (s *EventOrderTestSuite) TestVoidFormsAndUnsupportedKinds() {
	target := s.startOn("order-auto-flow", "owner")
	_, err := s.db.NewUpdate().Model((*approval.Flow)(nil)).Set("void_policy", approval.VoidDirect).Where(func(cb orm.ConditionBuilder) { cb.PKEquals(target.FlowID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should set the live form policy")
	core := voiding.NewCore(service.NewInstanceService(nil), testRegistry(testBusinessHandler{}), s.bus)
	in := approval.VoidInstanceInput{InstanceID: target.ID, Caller: approval.SystemCaller, Reason: "Closed twice", Operator: target.Applicant()}
	run := func() error {
		return s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error { return core.Direct(ctx, tx, in) })
	}
	setKind := func(kind *approval.BusinessKind) {
		_, err := s.db.NewUpdate().Model(target).Set("business_kind", kind).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should select the version-pinned kind")
	}
	setKind(new(approval.BusinessKind("order")))
	s.ErrorIs(run(), approval.ErrVoidNotAllowed, "A kind without a voider must be refused even after a live policy change")
	setKind(nil)

	plan, err := core.Preview(s.ctx, s.db, in, approval.VoidDirect, nil)
	s.Require().NoError(err, "A form preview needs no business provider")
	s.False(plan.Plan.IsBlocked(), "An ordinary form has an empty plan")
	s.bus.Reset()
	s.Require().NoError(run(), "A plain form can be voided")
	s.requireEmittedInOrder(approval.EventTypeInstanceVoided)
	running := s.startOn("order-all-flow", "applicant")
	in.InstanceID = running.ID

	s.ErrorIs(run(), approval.ErrInstanceNotVoidable, "A running instance must complete before it can be voided")
}

func (s *EventOrderTestSuite) TestVoidRequestEventOrder() {
	target := s.startOn("order-auto-flow", "target-owner")
	fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "auto-void", simpleFlowDef())

	const code = "auto-void-flow"

	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("void_policy", approval.VoidApproval).
		Set("void_flow_code", code).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(target.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Target requires approval through the named request")
	_, err = s.db.NewUpdate().
		Model((*approval.FlowVersion)(nil)).
		Set("business_kind", approval.BusinessKindVoidRequest).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fixture.VersionID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should attach the built-in request handler to the published request")
	rt, err := voiding.NewRuntime(nil, service.NewFlowDefinitionService(), nil, nil, s.bus)
	s.Require().NoError(err, "Should build the nested transaction runtime")
	h := wrapWithBusAndDB(
		s.db,
		s.bus,
		command.NewStartInstanceHandler(s.db, buildTestEngineWithHooks(s.db, rt.Hooks), &MockInstanceNoGenerator{counter: 9000}, service.NewValidationService(nil), rt.Registry, nil, nil),
	)
	operator := approval.UserInfo{ID: "requester", Name: "Requester"}

	s.bus.Reset()

	var req *approval.Instance

	err = s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
		var err error

		req, err = h.Handle(
			contextx.SetDB(ctx, tx),
			command.StartInstanceCmd{Caller: approval.SystemCaller, FlowCode: code, Applicant: operator, FormData: map[string]any{"targetInstanceId": target.ID, "reason": "Duplicate"}},
		)

		return err
	})
	s.Require().NoError(err, "Approval of the request should void the target atomically")
	s.requireEmittedInOrder(approval.EventTypeInstanceCreated, approval.EventTypeInstanceVoided, approval.EventTypeInstanceCompleted)
	ev := s.bus.CapturedByType(approval.EventTypeInstanceVoided)[0].(*approval.InstanceVoidedEvent)
	s.Equal(target.ID, ev.InstanceID, "The void event identifies the reversed target")
	s.Equal(&req.ID, ev.RequestInstanceID, "The event links to the authorizing request")
	s.Equal(operator, ev.Operator, "The request applicant is the void operator")
	completion := s.bus.CapturedByType(approval.EventTypeInstanceCompleted)[0].(*approval.InstanceCompletedEvent)
	s.Equal(req.ID, completion.InstanceID, "Only the request completes")
}

func (s *EventOrderTestSuite) TestConcurrentDirectVoids() {
	target := s.startOn("order-auto-flow", "owner")
	_, err := s.db.NewUpdate().Model((*approval.Flow)(nil)).Set("void_policy", approval.VoidDirect).Where(func(cb orm.ConditionBuilder) { cb.PKEquals(target.FlowID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should enable direct void")
	core := voiding.NewCore(service.NewInstanceService(nil), nil, s.bus)
	s.bus.Reset()

	in := approval.VoidInstanceInput{InstanceID: target.ID, Caller: approval.SystemCaller, Reason: "Duplicate", Operator: target.Applicant()}
	start := make(chan struct{})

	var (
		group    sync.WaitGroup
		outcomes [2]error
	)
	for i := range outcomes {
		group.Go(func() {
			<-start

			outcomes[i] = s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error { return core.Direct(ctx, tx, in) })
		})
	}

	close(start)
	group.Wait()

	successes := 0
	for _, err := range outcomes {
		if err == nil {
			successes++
		}
	}

	s.Equal(1, successes, "Only one concurrent transaction can reverse the target: %v", outcomes)
	n, err := s.db.NewSelect().Model((*approval.VoidRecord)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", target.ID).Equals("status", approval.VoidVoided)
	}).Count(s.ctx)
	s.Require().NoError(err, "Should count committed reversals")
	s.Equal(int64(1), n, "One reversal record survives")
	s.requireEmittedInOrder(approval.EventTypeInstanceVoided)
}
