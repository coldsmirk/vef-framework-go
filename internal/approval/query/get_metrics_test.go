package query_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetMetricsTestSuite{ctx: env.Ctx, db: env.DB, env: env}
	})
}

// GetMetricsTestSuite tests GetMetricsHandler.
type GetMetricsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	env     *testx.DBEnv
	handler *query.GetMetricsHandler
}

func (s *GetMetricsTestSuite) SetupSuite() {
	s.handler = query.NewGetMetricsHandler(s.db)

	// ── Fixtures ──────────────────────────────────────────────────────────

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "met-flow", 1)

	// Two tenant-t1 instances: one running (non-final), one approved (final,
	// with finished_at set so AvgCompletionSeconds is computable).
	now := timex.Now()
	// finishedAt is deterministically ~1h after creation so finished_at -
	// created_at is a clearly-positive completion duration. Reusing now would
	// place finished_at at (or just before) the audit-hook created_at, which
	// MySQL's integer TIMESTAMPDIFF truncates to a flaky sub-second negative.
	finishedAt := timex.Of(time.Now().Add(time.Hour))

	running := &approval.Instance{
		TenantID:      "t1",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Running Instance",
		InstanceNo:    "MET-001",
		ApplicantID:   "user-x",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(running).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance")

	approved := &approval.Instance{
		TenantID:      "t1",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Approved Instance",
		InstanceNo:    "MET-002",
		ApplicantID:   "user-x",
		Status:        approval.InstanceApproved,
		FinishedAt:    &finishedAt,
	}
	_, err = s.db.NewInsert().Model(approved).Exec(s.ctx)
	s.Require().NoError(err, "Should insert approved instance")

	// Tenant-t2 instance (cross-tenant snapshot visibility, tenant-scoped invisibility).
	t2inst := &approval.Instance{
		TenantID:      "t2",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "T2 Instance",
		InstanceNo:    "MET-003",
		ApplicantID:   "user-y",
		Status:        approval.InstanceRejected,
		FinishedAt:    &finishedAt,
	}
	_, err = s.db.NewInsert().Model(t2inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert t2 instance")

	// Two pending tasks for t1 running instance; one is timed-out.
	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: running.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskPending, IsTimeout: true},
		{TenantID: "t1", InstanceID: running.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 2, Status: approval.TaskPending, IsTimeout: false},
		{TenantID: "t2", InstanceID: t2inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-c", SortOrder: 1, Status: approval.TaskApproved},
	}
	for i := range tasks {
		tasks[i].VisitID = ensureActiveVisit(s.T(), s.ctx, s.db, tasks[i].TenantID, tasks[i].InstanceID, tasks[i].NodeID).ID
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	instanceIDColumn := "approval_instance_id"

	projections := []approval.BusinessProjection{
		{
			TenantID:        "t1",
			FlowID:          fix.FlowID,
			FlowVersionID:   fix.VersionID,
			OwnerInstanceID: running.ID,
			TargetHash:      "metrics-pending-target",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "business_order",
				KeyColumns:       []string{"id"},
				StatusColumn:     "approval_status",
				InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:        []byte(`[{"column":"id","kind":"string","value":"order-1"}]`),
			DesiredStatus:    approval.InstanceRunning,
			DesiredStartedAt: now,
			DesiredRevision:  2,
			AppliedRevision:  1,
			Status:           approval.BindingProjectionPending,
		},
		{
			TenantID:        "t1",
			FlowID:          fix.FlowID,
			FlowVersionID:   fix.VersionID,
			OwnerInstanceID: approved.ID,
			TargetHash:      "metrics-failed-target",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "business_order",
				KeyColumns:       []string{"id"},
				StatusColumn:     "approval_status",
				InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:         []byte(`[{"column":"id","kind":"string","value":"order-2"}]`),
			DesiredStatus:     approval.InstanceApproved,
			DesiredStartedAt:  now,
			DesiredFinishedAt: &finishedAt,
			DesiredRevision:   3,
			AppliedRevision:   2,
			Status:            approval.BindingProjectionFailed,
		},
		{
			TenantID:        "t2",
			FlowID:          fix.FlowID,
			FlowVersionID:   fix.VersionID,
			OwnerInstanceID: t2inst.ID,
			TargetHash:      "metrics-applied-target",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "business_order",
				KeyColumns:       []string{"id"},
				StatusColumn:     "approval_status",
				InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:         []byte(`[{"column":"id","kind":"string","value":"order-3"}]`),
			DesiredStatus:     approval.InstanceRejected,
			DesiredStartedAt:  now,
			DesiredFinishedAt: &finishedAt,
			DesiredRevision:   1,
			AppliedRevision:   1,
			Status:            approval.BindingProjectionApplied,
		},
	}
	for i := range projections {
		_, err := s.db.NewInsert().Model(&projections[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert business projection")
	}
}

func (s *GetMetricsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetMetricsTestSuite) TestCrossTenantSnapshot() {
	metrics, err := s.handler.Handle(s.ctx, query.GetMetricsQuery{})
	s.Require().NoError(err, "Should query cross-tenant metrics without error")

	// Instance counts span both tenants.
	s.Assert().Equal(1, metrics.InstanceCounts[string(approval.InstanceRunning)], "Should count 1 running instance")
	s.Assert().Equal(1, metrics.InstanceCounts[string(approval.InstanceApproved)], "Should count 1 approved instance")
	s.Assert().Equal(1, metrics.InstanceCounts[string(approval.InstanceRejected)], "Should count 1 rejected instance")

	// Task counts span both tenants.
	s.Assert().GreaterOrEqual(metrics.TaskCounts[string(approval.TaskPending)], 2, "Should count at least 2 pending tasks")

	// Timeout task count: only the t1 timed-out pending task.
	s.Assert().Equal(1, metrics.TimeoutTaskCount, "Should count 1 timeout task")

	// AvgCompletionSeconds: 2 completed instances (approved + rejected) exist,
	// each finished ~1h after creation, so the cross-tenant average is ~3600s.
	s.Assert().InDelta(3600, metrics.AvgCompletionSeconds, 120, "AvgCompletionSeconds should be ~3600s (≈1h) across completed instances")

	s.Assert().Equal(1, metrics.BusinessProjectionCounts[string(approval.BindingProjectionPending)], "Should count 1 pending projection")
	s.Assert().Equal(1, metrics.BusinessProjectionCounts[string(approval.BindingProjectionFailed)], "Should count 1 failed projection")
	s.Assert().Equal(1, metrics.BusinessProjectionCounts[string(approval.BindingProjectionApplied)], "Should count 1 applied projection")
	s.Assert().Equal(2, metrics.PendingBusinessProjections, "Should count pending and failed eventual projections whose desired revision is unapplied")
	s.Assert().Equal(1, metrics.PendingBindingFailures, "Should count the failed projection target")
}

func (s *GetMetricsTestSuite) TestTenantScopedMetrics() {
	t1 := "t1"
	metrics, err := s.handler.Handle(s.ctx, query.GetMetricsQuery{TenantID: t1})
	s.Require().NoError(err, "Should query tenant-scoped metrics without error")

	// Only t1 instances are visible.
	s.Assert().Equal(1, metrics.InstanceCounts[string(approval.InstanceRunning)], "Should count 1 running instance for t1")
	s.Assert().Equal(1, metrics.InstanceCounts[string(approval.InstanceApproved)], "Should count 1 approved instance for t1")
	s.Assert().Equal(0, metrics.InstanceCounts[string(approval.InstanceRejected)], "Should count 0 rejected instances for t1 (belongs to t2)")

	// Timeout task count: only t1 timed-out tasks.
	s.Assert().Equal(1, metrics.TimeoutTaskCount, "Should count 1 timeout task for t1")

	// AvgCompletionSeconds for t1: 1 approved instance finished ~1h after creation.
	s.Assert().InDelta(3600, metrics.AvgCompletionSeconds, 120, "AvgCompletionSeconds should be ~3600s (≈1h) for t1")
	s.Assert().Equal(1, metrics.BusinessProjectionCounts[string(approval.BindingProjectionPending)], "Should count 1 pending projection for t1")
	s.Assert().Equal(1, metrics.BusinessProjectionCounts[string(approval.BindingProjectionFailed)], "Should count 1 failed projection for t1")
	s.Assert().Equal(0, metrics.BusinessProjectionCounts[string(approval.BindingProjectionApplied)], "Should exclude t2 applied projections")
	s.Assert().Equal(2, metrics.PendingBusinessProjections, "Should count 2 unapplied eventual projections for t1")
	s.Assert().Equal(1, metrics.PendingBindingFailures, "Should count 1 failed projection for t1")
}

func (s *GetMetricsTestSuite) TestEmptyTenantReturnsNegativeOneAvg() {
	// A tenant with no completed instances should return the -1 sentinel.
	emptyTenant := "empty-tenant"
	metrics, err := s.handler.Handle(s.ctx, query.GetMetricsQuery{TenantID: emptyTenant})
	s.Require().NoError(err, "Should query empty-tenant metrics without error")

	s.Assert().Equal(-1.0, metrics.AvgCompletionSeconds, "Should return -1 sentinel when no completed instances exist")
	s.Assert().Empty(metrics.InstanceCounts, "Should return empty instance counts for unknown tenant")
	s.Assert().Equal(0, metrics.TimeoutTaskCount, "Should return 0 timeout tasks for unknown tenant")
	s.Assert().Empty(metrics.BusinessProjectionCounts, "Should return empty projection counts for unknown tenant")
	s.Assert().Equal(0, metrics.PendingBusinessProjections, "Should return 0 pending projections for unknown tenant")
	s.Assert().Equal(0, metrics.PendingBindingFailures, "Should return 0 failed projections for unknown tenant")
}
