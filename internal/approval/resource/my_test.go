package resource_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Titles of the two instances MyTaskListFiltersTestSuite tells apart.
const (
	travelTitle   = "Travel reimbursement"
	purchaseTitle = "Purchase request"
)

// septemberAt returns a fixed local instant in September 2026 — local because
// the API parses the "2006-01-02 15:04:05" params it is compared against in
// the local zone.
func septemberAt(day, hour int) timex.DateTime {
	return timex.Of(time.Date(2026, time.September, day, hour, 0, 0, 0, time.Local))
}

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

// TestCountsTheCallersOwnTasks seeds a single pending task in tenant "t1" for
// the calling user, then checks that the counts follow the user alone: a "my"
// endpoint reads the caller's own work in every tenant, and a tenant sent along
// changes nothing.
func (s *MyPendingCountsTestSuite) TestCountsTheCallersOwnTasks() {
	cleanAllApprovalData(s.ctx, s.db)
	s.seedPendingTask("t1", "counter")

	token := s.GenerateToken(newTenantUser("counter", "Counter", "user"))

	pendingCount := func(params map[string]any) float64 {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Resource: "approval/my",
			Action:   "get_pending_counts",
			Version:  "v1",
			Params:   params,
		}, token)
		s.Require().Equal(http.StatusOK, resp.StatusCode, "get_pending_counts should succeed")

		body := s.ReadResult(resp)
		s.Require().True(body.IsOk(), "get_pending_counts should report success")

		return s.ReadDataAsMap(body.Data)["pendingTaskCount"].(float64)
	}

	s.Equal(float64(1), pendingCount(nil), "the caller's pending task should be counted")
	s.Equal(float64(1), pendingCount(map[string]any{"tenantId": "t2"}),
		"a tenant sent along must not narrow the caller's own counts")
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
		TenantID:      tenant,
		FlowID:        flow.ID,
		FlowVersionID: version.ID,
		Title:         "MPC Instance",
		InstanceNo:    "MPC-001",
		ApplicantID:   "applicant",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "seed instance")

	visit := &approval.NodeVisit{
		TenantID:   tenant,
		InstanceID: instance.ID,
		NodeID:     node.ID,
		Sequence:   1,
		Status:     approval.NodeVisitActive,
	}
	_, err = s.db.NewInsert().Model(visit).Exec(s.ctx)
	s.Require().NoError(err, "seed node visit")

	task := &approval.Task{
		TenantID:   tenant,
		InstanceID: instance.ID,
		NodeID:     node.ID,
		VisitID:    visit.ID,
		AssigneeID: assignee,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "seed pending task")
}

// MyTaskListFiltersTestSuite verifies that approval/my find_pending_tasks and
// find_completed_tasks thread every filter param into their queries. Each
// param alone keeps exactly one of two seeded instances, so a param that is
// dropped or wired to the wrong field returns the wrong rows. The filters' SQL
// semantics are covered per dialect by the query suites.
type MyTaskListFiltersTestSuite struct {
	apptest.Suite

	ctx            context.Context
	db             orm.DB
	token          string
	travelFlowID   string
	purchaseFlowID string
}

func TestMyTaskListFilters(t *testing.T) {
	suite.Run(t, new(MyTaskListFiltersTestSuite))
}

func (s *MyTaskListFiltersTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = setupResourceApp(&s.Suite)
	s.token = s.GenerateToken(newTenantUser("filterer", "Filterer", "user"))

	travel := &approval.Instance{
		TenantID:      testTenant,
		Title:         travelTitle,
		InstanceNo:    "MTF-001",
		ApplicantID:   "user-p",
		ApplicantName: "Alice Wang",
		Status:        approval.InstanceRunning,
	}
	travelNodeID, travelVisitID := s.seedInstance("mtf-travel", travel)
	s.travelFlowID = travel.FlowID

	purchase := &approval.Instance{
		TenantID:      testTenant,
		Title:         purchaseTitle,
		InstanceNo:    "MTF-002",
		ApplicantID:   "user-q",
		ApplicantName: "Bob Li",
		Status:        approval.InstanceApproved,
	}
	purchaseNodeID, purchaseVisitID := s.seedInstance("mtf-purchase", purchase)
	s.purchaseFlowID = purchase.FlowID

	travelFinishedAt, purchaseFinishedAt := septemberAt(1, 10), septemberAt(10, 10)

	for _, task := range []*approval.Task{
		{InstanceID: travel.ID, NodeID: travelNodeID, VisitID: travelVisitID, SortOrder: 1, Status: approval.TaskPending, CreatedAt: septemberAt(1, 10)},
		{InstanceID: travel.ID, NodeID: travelNodeID, VisitID: travelVisitID, SortOrder: 2, Status: approval.TaskApproved, FinishedAt: &travelFinishedAt},
		{InstanceID: purchase.ID, NodeID: purchaseNodeID, VisitID: purchaseVisitID, SortOrder: 1, Status: approval.TaskPending, IsTimeout: true, CreatedAt: septemberAt(10, 10)},
		{InstanceID: purchase.ID, NodeID: purchaseNodeID, VisitID: purchaseVisitID, SortOrder: 2, Status: approval.TaskRejected, FinishedAt: &purchaseFinishedAt},
	} {
		task.TenantID, task.AssigneeID = testTenant, "filterer"
		_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
		s.Require().NoError(err, "seed task")
	}
}

func (s *MyTaskListFiltersTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *MyTaskListFiltersTestSuite) TestFindPendingTasksFilters() {
	cases := []struct {
		name   string
		params map[string]any
		want   []string
	}{
		{"NoFilter", nil, []string{travelTitle, purchaseTitle}},
		{"Keyword", map[string]any{"keyword": "reimburse"}, []string{travelTitle}},
		{"ApplicantID", map[string]any{"applicantId": "user-q"}, []string{purchaseTitle}},
		{"ApplicantName", map[string]any{"applicantName": "Alice"}, []string{travelTitle}},
		{"FlowID", map[string]any{"flowId": s.purchaseFlowID}, []string{purchaseTitle}},
		{"IsTimeout", map[string]any{"isTimeout": true}, []string{purchaseTitle}},
		{"CreatedAtFrom", map[string]any{"createdAtFrom": "2026-09-05 00:00:00"}, []string{purchaseTitle}},
		{"CreatedAtTo", map[string]any{"createdAtTo": "2026-09-05 00:00:00"}, []string{travelTitle}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.ElementsMatch(tc.want, s.findTitles("find_pending_tasks", tc.params),
				"find_pending_tasks should honor the %s param", tc.name)
		})
	}
}

func (s *MyTaskListFiltersTestSuite) TestFindCompletedTasksFilters() {
	cases := []struct {
		name   string
		params map[string]any
		want   []string
	}{
		{"NoFilter", nil, []string{travelTitle, purchaseTitle}},
		{"Keyword", map[string]any{"keyword": "request"}, []string{purchaseTitle}},
		{"ApplicantID", map[string]any{"applicantId": "user-p"}, []string{travelTitle}},
		{"ApplicantName", map[string]any{"applicantName": "Bob"}, []string{purchaseTitle}},
		{"FlowID", map[string]any{"flowId": s.travelFlowID}, []string{travelTitle}},
		{"Status", map[string]any{"status": "rejected"}, []string{purchaseTitle}},
		{"InstanceStatus", map[string]any{"instanceStatus": "running"}, []string{travelTitle}},
		{"FinishedAtFrom", map[string]any{"finishedAtFrom": "2026-09-05 00:00:00"}, []string{purchaseTitle}},
		{"FinishedAtTo", map[string]any{"finishedAtTo": "2026-09-05 00:00:00"}, []string{travelTitle}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.ElementsMatch(tc.want, s.findTitles("find_completed_tasks", tc.params),
				"find_completed_tasks should honor the %s param", tc.name)
		})
	}
}

// findTitles calls an approval/my task-list action as the seeded user and
// returns the instance titles of the page it answers.
func (s *MyTaskListFiltersTestSuite) findTitles(action string, params map[string]any) []string {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/my",
		Action:   action,
		Version:  "v1",
		Params:   params,
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "%s should succeed", action)

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "%s should report success: %s", action, body.Message)

	items := s.ReadDataAsSlice(s.ReadDataAsMap(body.Data)["items"])
	titles := make([]string, len(items))

	for i, item := range items {
		titles[i], _ = s.ReadDataAsMap(item)["instanceTitle"].(string)
	}

	return titles
}

// seedInstance creates a category → flow → version → node chain for code,
// inserts inst on it with an open visit on the node, and returns the node and
// visit the instance's tasks bind to.
func (s *MyTaskListFiltersTestSuite) seedInstance(code string, inst *approval.Instance) (nodeID, visitID string) {
	category := &approval.FlowCategory{TenantID: testTenant, Code: code + "-cat", Name: code + " Category"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "seed flow category")

	flow := &approval.Flow{
		TenantID:              testTenant,
		CategoryID:            category.ID,
		Code:                  code,
		Name:                  code + " Flow",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Test",
		IsActive:              true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "seed flow")

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "seed flow version")

	node := &approval.FlowNode{FlowVersionID: version.ID, Key: code + "-node", Kind: approval.NodeApproval, Name: code + " Node"}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "seed flow node")

	inst.FlowID, inst.FlowVersionID = flow.ID, version.ID
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "seed instance")

	visit := &approval.NodeVisit{
		TenantID:   testTenant,
		InstanceID: inst.ID,
		NodeID:     node.ID,
		Sequence:   1,
		Status:     approval.NodeVisitActive,
	}
	_, err = s.db.NewInsert().Model(visit).Exec(s.ctx)
	s.Require().NoError(err, "seed node visit")

	return node.ID, visit.ID
}
