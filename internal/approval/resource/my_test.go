package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// MyPendingCountsTestSuite verifies that approval/my get_pending_counts threads
// the client-supplied tenantId into the underlying query. It runs on the
// default in-memory SQLite datasource so the full RPC path is covered.
type MyPendingCountsTestSuite struct {
	apptest.Suite

	ctx context.Context
	db  orm.DB
}

func TestMyPendingCounts(t *testing.T) {
	suite.Run(t, new(MyPendingCountsTestSuite))
}

func (s *MyPendingCountsTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.SetupApp(
		fx.Replace(
			&security.JWTConfig{Secret: security.DefaultJWTSecret, Audience: "test_app"},
			newApprovalConfig(),
		),
		fx.Provide(func() context.Context { return s.ctx }),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return &MockUserInfoResolver{} }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDepartmentResolver { return &MockPrincipalDepartmentResolver{} }, fx.As(new(approval.PrincipalDepartmentResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Decorate(
			fx.Annotate(func() security.PermissionChecker { return &MockPermissionChecker{} }, fx.As(new(security.PermissionChecker))),
		),
		fx.Populate(&s.db),
	)
}

func (s *MyPendingCountsTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// TestTenantIdScopesPendingCounts seeds a single pending task in tenant "t1"
// for the calling user, then checks that the tenantId param actually narrows
// the count: the owning tenant and the unscoped call see it, a different tenant
// does not.
func (s *MyPendingCountsTestSuite) TestTenantIdScopesPendingCounts() {
	cleanAllApprovalData(s.ctx, s.db)
	s.seedPendingTask("t1", "counter")

	token := s.GenerateToken(newTenantUser("counter", "Counter", "user"))

	pendingCount := func(params map[string]any) float64 {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/my", Action: "get_pending_counts", Version: "v1"},
			Params:     params,
		}, token)
		s.Require().Equal(http.StatusOK, resp.StatusCode, "get_pending_counts should succeed")

		body := s.ReadResult(resp)
		s.Require().True(body.IsOk(), "get_pending_counts should report success")

		return s.ReadDataAsMap(body.Data)["pendingTaskCount"].(float64)
	}

	s.Equal(float64(1), pendingCount(nil), "an unscoped call should count the pending task")
	s.Equal(float64(1), pendingCount(map[string]any{"tenantId": "t1"}), "the owning tenant should count the pending task")
	s.Equal(float64(0), pendingCount(map[string]any{"tenantId": "t2"}), "a different tenant must not count the task — proving tenantId threads into the query")
}

// seedPendingTask creates the minimal flow → version → node → instance → task
// chain (FK constraints are enforced on SQLite) with one pending task assigned
// to assignee in the given tenant.
func (s *MyPendingCountsTestSuite) seedPendingTask(tenant, assignee string) {
	category := &approval.FlowCategory{TenantID: tenant, Code: "mpc-cat", Name: "MPC Category"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "seed flow category")

	flow := &approval.Flow{
		TenantID:              tenant,
		CategoryID:            category.ID,
		Code:                  "mpc-flow",
		Name:                  "MPC Flow",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Test",
		IsActive:              true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "seed flow")

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "seed flow version")

	node := &approval.FlowNode{FlowVersionID: version.ID, Key: "mpc-node", Kind: approval.NodeApproval, Name: "MPC Node"}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "seed flow node")

	instance := &approval.Instance{
		TenantID: tenant, FlowID: flow.ID, FlowVersionID: version.ID,
		Title: "MPC Instance", InstanceNo: "MPC-001", ApplicantID: "applicant", Status: approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "seed instance")

	task := &approval.Task{
		TenantID: tenant, InstanceID: instance.ID, NodeID: node.ID,
		AssigneeID: assignee, SortOrder: 1, Status: approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "seed pending task")
}
