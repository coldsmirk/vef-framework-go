package command_test

import (
	"context"
	"slices"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &EventOrderTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// EventOrderTestSuite pins the publication order of approval domain events.
//
// The collector is one ordered buffer shared by three producers — the command
// handler, the node service, and the engine's recursive traversal — and the
// engine emits from deep inside the call it is given. A handler that batches
// its own events for the end therefore lets the engine's overtake them, which
// is how approval.instance.completed used to reach subscribers before the
// approval.task.approved that caused it and before the
// approval.instance.created of the instance it completed. Every producer now
// emits at the moment the event occurs; these tests are the fence.
type EventOrderTestSuite struct {
	suite.Suite

	ctx context.Context
	db  orm.DB
	bus *eventtest.FakeBus

	autoComplete *FlowFixture
	allOf        *FlowFixture
	anyOf        *FlowFixture

	start   cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	reject  cqrs.Handler[command.RejectTaskCmd, cqrs.Unit]
}

func (s *EventOrderTestSuite) SetupSuite() {
	s.autoComplete = deployAndPublishFlow(s.T(), s.ctx, s.db, "order-auto", simpleFlowDef())
	s.allOf = deployAndPublishFlow(s.T(), s.ctx, s.db, "order-all", approvalFlowDef())
	s.anyOf = deployAndPublishFlow(s.T(), s.ctx, s.db, "order-any", eventOrderAnyPassFlowDef())

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	s.bus = eventtest.NewFakeBus()

	s.start = wrapWithBusAndDB(s.db, s.bus, command.NewStartInstanceHandler(
		s.db, eng, &MockInstanceNoGenerator{}, validSvc,
		binding.NewNoopRefProvider(),
		binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil),
		nil,
	))
	s.approve = wrapWithBusAndDB(s.db, s.bus, command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.reject = wrapWithBusAndDB(s.db, s.bus, command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
}

func (s *EventOrderTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *EventOrderTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// eventOrderAnyPassFlowDef builds start → approval(parallel, any, two
// assignees) → end. One approval satisfies the pass rule, so the peer's task is
// canceled and the instance completes — all three facts in a single action.
func eventOrderAnyPassFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
			{
				ID:   "approval-1",
				Kind: approval.NodeApproval,
				Data: mustMarshal(approval.ApprovalNodeData{
					Name: "或签",
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"any-1", "any-2"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
					ApprovalMethod:      approval.ApprovalParallel,
					PassRule:            approval.PassAny,
				}),
			},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "e1", Source: "start-1", Target: "approval-1"},
			{ID: "e2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// capturedTypes returns the event types the bus has seen, in publication order.
func (s *EventOrderTestSuite) capturedTypes() []string {
	captured := s.bus.Captured()

	out := make([]string, 0, len(captured))
	for _, e := range captured {
		out = append(out, e.EventType())
	}

	return out
}

// requireEmittedInOrder asserts every named event type was published exactly
// once and that they appear in the given relative order. Exactly-once matters
// as much as the order: the node service now emits its own events, so a caller
// that also re-adds the returned slice would publish each of them twice.
func (s *EventOrderTestSuite) requireEmittedInOrder(types ...string) {
	seen := s.capturedTypes()

	previous := -1

	for _, want := range types {
		at := slices.Index(seen, want)
		s.Require().GreaterOrEqualf(at, 0, "%s should have been published, got %v", want, seen)
		s.Require().Lenf(s.bus.CapturedByType(want), 1, "%s should be published exactly once, got %v", want, seen)
		s.Require().Greaterf(at, previous, "%s should follow the event that caused it, got %v", want, seen)

		previous = at
	}
}

func (s *EventOrderTestSuite) startOn(flowCode, applicant string) *approval.Instance {
	instance, err := s.start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  flowCode,
		Applicant: approval.UserInfo{ID: applicant, Name: applicant},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoErrorf(err, "Should start an instance of %s", flowCode)

	return instance
}

func (s *EventOrderTestSuite) pendingTaskID(instanceID, assignee string) string {
	var task approval.Task

	s.Require().NoErrorf(s.db.NewSelect().Model(&task).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("assignee_id", assignee).
				Equals("status", string(approval.TaskPending))
		}).Scan(s.ctx), "Should find the pending task of %s", assignee)

	return task.ID
}

// TestInstanceCreatedPrecedesCompletion covers a flow whose start node reaches
// the end node without stopping: creation and completion both happen inside
// start_instance, so a subscriber would otherwise be told the instance finished
// before being told it exists.
func (s *EventOrderTestSuite) TestInstanceCreatedPrecedesCompletion() {
	s.bus.Reset()

	instance := s.startOn("order-auto-flow", "creator")
	s.Require().Equal(approval.InstanceApproved, instance.Status, "Start-to-end flow should finish as approved")

	s.requireEmittedInOrder(
		approval.EventTypeInstanceCreated,
		approval.EventTypeInstanceCompleted,
	)
}

// TestApprovalPrecedesCompletion covers the decision that ends the flow: the
// approval is the cause, the completion its consequence.
func (s *EventOrderTestSuite) TestApprovalPrecedesCompletion() {
	instance := s.startOn("order-all-flow", "applicant-all")

	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   s.pendingTaskID(instance.ID, "user-1"),
		Operator: approval.UserInfo{ID: "user-1", Name: "user-1"},
		Opinion:  "ok",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "First approval should succeed")

	// Reset after the non-completing approval so the final action's events
	// stand alone — the queue advance would otherwise contribute a second
	// task.approved and blur the exactly-once assertion.
	s.bus.Reset()

	_, err = s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   s.pendingTaskID(instance.ID, "user-2"),
		Operator: approval.UserInfo{ID: "user-2", Name: "user-2"},
		Opinion:  "ok",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Final approval should succeed")

	s.requireEmittedInOrder(
		approval.EventTypeTaskApproved,
		approval.EventTypeInstanceCompleted,
	)
}

// TestCancellationPrecedesCompletion covers the three-fact action: one approval
// satisfies an any-rule node, which cancels the peer's task and then completes
// the instance. The cancellation is emitted by the node service and the
// completion by the engine one call deeper, so their order is the one the two
// producers most easily get wrong.
func (s *EventOrderTestSuite) TestCancellationPrecedesCompletion() {
	instance := s.startOn("order-any-flow", "applicant-any")

	s.bus.Reset()

	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   s.pendingTaskID(instance.ID, "any-1"),
		Operator: approval.UserInfo{ID: "any-1", Name: "any-1"},
		Opinion:  "ok",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "A single approval should satisfy the any rule")

	s.requireEmittedInOrder(
		approval.EventTypeTaskApproved,
		approval.EventTypeTaskCanceled,
		approval.EventTypeInstanceCompleted,
	)
}

// TestRejectionPrecedesCompletion covers the rejection lane, where the node
// service — not the engine — publishes the completion event.
func (s *EventOrderTestSuite) TestRejectionPrecedesCompletion() {
	instance := s.startOn("order-all-flow", "applicant-reject")

	s.bus.Reset()

	_, err := s.reject.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   s.pendingTaskID(instance.ID, "user-1"),
		Operator: approval.UserInfo{ID: "user-1", Name: "user-1"},
		Opinion:  "no",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Rejection should succeed")

	s.requireEmittedInOrder(
		approval.EventTypeTaskRejected,
		approval.EventTypeTaskCanceled,
		approval.EventTypeInstanceCompleted,
	)
}
