package facade_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// --- Host contracts the approval module needs supplied ---

type StubAssigneeService struct{}

func (*StubAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

func (*StubAssigneeService) GetDepartmentLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

func (*StubAssigneeService) GetRoleUsers(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

type StubUserInfoResolver struct{}

func (*StubUserInfoResolver) ResolveUsers(_ context.Context, userIDs []string) (map[string]approval.UserInfo, error) {
	users := make(map[string]approval.UserInfo, len(userIDs))
	for _, id := range userIDs {
		users[id] = approval.UserInfo{ID: id, Name: id}
	}

	return users, nil
}

type StubPrincipalDepartmentResolver struct{}

func (*StubPrincipalDepartmentResolver) Resolve(context.Context, *security.Principal) (departmentID, departmentName *string, err error) {
	return nil, nil, nil
}

type CountingInstanceNoGenerator struct {
	counter atomic.Int64
}

func (g *CountingInstanceNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	return fmt.Sprintf("%s-%04d", flowCode, g.counter.Add(1)), nil
}

// ServiceIntegrationTestSuite drives approval.Service against the real module
// — command pipeline, engine, Postgres — the way a host routine would: no HTTP,
// no principal, the caller's own orm.DB handle.
type ServiceIntegrationTestSuite struct {
	apptest.Suite

	ctx context.Context
	db  orm.DB
	bus cqrs.Bus
	svc approval.Service

	activeFlow   string
	inactiveFlow string
}

func TestServiceIntegration(t *testing.T) {
	suite.Run(t, new(ServiceIntegrationTestSuite))
}

const (
	tenant    = "default"
	applicant = "applicant-1"
	approver  = "approver-1"
)

var (
	applicantInfo = approval.UserInfo{ID: applicant, Name: "Applicant"}
	approverInfo  = approval.UserInfo{ID: approver, Name: "Approver"}
	caller        = approval.CallerContext{TenantID: tenant}
)

func (s *ServiceIntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()
	pg := testx.NewPostgresContainer(s.ctx, s.T())

	cfg := &config.ApprovalConfig{AutoMigrate: true}
	cfg.ApplyDefaults()

	s.SetupApp(
		fx.Replace(pg.DataSource, cfg),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return new(StubAssigneeService) }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return new(StubUserInfoResolver) }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDepartmentResolver { return new(StubPrincipalDepartmentResolver) }, fx.As(new(approval.PrincipalDepartmentResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return new(CountingInstanceNoGenerator) }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Populate(&s.db, &s.bus, &s.svc),
	)

	category := &approval.FlowCategory{TenantID: tenant, Code: "svc-cat", Name: "Service Test", IsActive: true}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert the test category")

	s.activeFlow = s.publishFlow("svc-active", category.ID)
	s.inactiveFlow = s.publishFlow("svc-inactive", category.ID)
	s.setFlowActive(s.inactiveFlow, false)
}

func (s *ServiceIntegrationTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *ServiceIntegrationTestSuite) TearDownTest() {
	for _, model := range []any{
		(*approval.ActionLog)(nil), (*approval.UrgeRecord)(nil), (*approval.CCRecord)(nil),
		(*approval.Task)(nil), (*approval.NodeVisit)(nil), (*approval.FormSnapshot)(nil),
		(*approval.Instance)(nil),
	} {
		_, err := s.db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(s.ctx)
		s.Require().NoError(err, "Should clean runtime rows for %T", model)
	}
}

// publishFlow creates, deploys and publishes Start → Approval(approver-1) → End
// through the flow commands — design-time work the Service deliberately does
// not cover — and returns the flow code.
func (s *ServiceIntegrationTestSuite) publishFlow(code, categoryID string) string {
	flow, err := cqrs.Send[command.CreateFlowCmd, *approval.Flow](s.ctx, s.bus, command.CreateFlowCmd{
		TenantID:               tenant,
		Code:                   code,
		Name:                   code,
		CategoryID:             categoryID,
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		Caller:                 caller,
	})
	s.Require().NoError(err, "Should create flow %s", code)

	version, err := cqrs.Send[command.DeployFlowCmd, *approval.FlowVersion](s.ctx, s.bus, command.DeployFlowCmd{
		FlowID:      flow.ID,
		StorageMode: approval.StorageJSON,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "Start"})},
				{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
					Name:           "Approve",
					Assignees:      []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{approver}, SortOrder: 1}},
					ExecutionType:  approval.ExecutionManual,
					ApprovalMethod: approval.ApprovalSequential,
					PassRule:       approval.PassAll,
				})},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "End"})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "approval-1"},
				{ID: "edge-2", Source: "approval-1", Target: "end-1"},
			},
		},
		Caller: caller,
	})
	s.Require().NoError(err, "Should deploy flow %s", code)

	_, err = cqrs.Send[command.PublishVersionCmd, cqrs.Unit](s.ctx, s.bus, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin",
		Caller:     caller,
	})
	s.Require().NoError(err, "Should publish flow %s", code)

	return code
}

func (s *ServiceIntegrationTestSuite) setFlowActive(code string, active bool) {
	var flow approval.Flow

	err := s.db.NewSelect().Model(&flow).Where(func(cb orm.ConditionBuilder) { cb.Equals("code", code) }).Scan(s.ctx)
	s.Require().NoError(err, "Should load flow %s", code)

	_, err = cqrs.Send[command.ToggleFlowActiveCmd, cqrs.Unit](s.ctx, s.bus, command.ToggleFlowActiveCmd{
		FlowID: flow.ID, IsActive: active, Caller: caller,
	})
	s.Require().NoError(err, "Should toggle flow %s", code)
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}

func (s *ServiceIntegrationTestSuite) countInstances(id string) int64 {
	count, err := s.db.NewSelect().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("id", id) }).Count(s.ctx)
	s.Require().NoError(err, "Should count instances")

	return count
}

func (s *ServiceIntegrationTestSuite) pendingTask(instanceID string) approval.Task {
	// Scanned as a slice rather than a single row: scanning into one struct
	// takes the first match and reports no error when several exist, so a
	// regression opening a second seat would surface as an unrelated assertion
	// failing further down instead of here.
	var tasks []approval.Task

	err := s.db.NewSelect().Model(&tasks).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID).Equals("status", approval.TaskPending)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should load the instance's pending tasks")
	s.Require().Len(tasks, 1, "Instance should have exactly one pending task")

	return tasks[0]
}

func startInput(flowCode string) approval.StartInstanceInput {
	return approval.StartInstanceInput{
		TenantID:  tenant,
		FlowCode:  flowCode,
		Applicant: applicantInfo,
		FormData:  map[string]any{},
		Caller:    caller,
	}
}

// TestStartJoinsCallerTransaction is the reason the Service takes a db handle:
// an instance started inside the caller's RunInTx must vanish with the
// caller's rollback, which can only hold if the pipeline joined that
// transaction instead of committing one of its own.
func (s *ServiceIntegrationTestSuite) TestStartJoinsCallerTransaction() {
	errAbort := errors.New("business write failed")

	var started *approval.Instance

	err := s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
		instance, err := s.svc.StartInstance(ctx, tx, startInput(s.activeFlow))
		if err != nil {
			return err
		}

		started = instance

		return errAbort
	})
	s.Require().ErrorIs(err, errAbort, "The caller's own error should come back unchanged")
	s.Require().NotNil(started, "StartInstance should have returned the instance before the rollback")
	s.Equal(approval.InstanceRunning, started.Status, "The instance should have been running inside the transaction")
	s.Zero(s.countInstances(started.ID), "The rollback must take the approval instance with it")
}

// TestLifecycleOnPlainHandle drives start → approve on the injected primary
// handle: the pipeline opens and commits its own transaction per operation,
// and a second host routine can act on the resulting task.
func (s *ServiceIntegrationTestSuite) TestLifecycleOnPlainHandle() {
	instance, err := s.svc.StartInstance(s.ctx, s.db, startInput(s.activeFlow))
	s.Require().NoError(err, "StartInstance on the plain handle should succeed")
	s.EqualValues(1, s.countInstances(instance.ID), "The instance should be committed")

	task := s.pendingTask(instance.ID)
	s.Equal(approver, task.AssigneeID, "The approval node should have opened a task on the configured approver")

	err = s.db.RunInTx(s.ctx, func(ctx context.Context, tx orm.DB) error {
		return s.svc.ApproveTask(ctx, tx, approval.ApproveTaskInput{
			TaskID: task.ID, Operator: approverInfo, Opinion: "approved by routine", Caller: caller,
		})
	})
	s.Require().NoError(err, "ApproveTask inside a committing RunInTx should succeed")

	var reloaded approval.Instance

	err = s.db.NewSelect().Model(&reloaded).Where(func(cb orm.ConditionBuilder) { cb.Equals("id", instance.ID) }).Scan(s.ctx)
	s.Require().NoError(err, "Should reload the instance")
	s.Equal(approval.InstanceApproved, reloaded.Status, "Approving the only task should complete the instance")
}

// TestErrorsAreMatchable pins what moving the sentinels into the public
// package buys a host: errors.Is against approval.Err* works on what the
// Service returns.
func (s *ServiceIntegrationTestSuite) TestErrorsAreMatchable() {
	s.Run("InactiveFlow", func() {
		_, err := s.svc.StartInstance(s.ctx, s.db, startInput(s.inactiveFlow))
		s.Require().ErrorIs(err, approval.ErrFlowNotActive, "Starting an inactive flow should be ErrFlowNotActive")
	})

	s.Run("ZeroCallerFailsClosed", func() {
		in := startInput(s.activeFlow)
		in.Caller = approval.CallerContext{}

		_, err := s.svc.StartInstance(s.ctx, s.db, in)
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "A zero CallerContext must be denied as not-found, never waved through")
	})

	s.Run("WithdrawByStranger", func() {
		instance, err := s.svc.StartInstance(s.ctx, s.db, startInput(s.activeFlow))
		s.Require().NoError(err, "StartInstance should succeed")

		err = s.svc.WithdrawInstance(s.ctx, s.db, approval.WithdrawInstanceInput{
			InstanceID: instance.ID, Operator: approverInfo, Reason: "not mine", Caller: caller,
		})
		s.Require().ErrorIs(err, approval.ErrNotApplicant, "Only the applicant may withdraw")
	})
}
