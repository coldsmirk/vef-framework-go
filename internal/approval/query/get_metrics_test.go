package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	outboxmodel "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	ioutbox "github.com/coldsmirk/vef-framework-go/internal/event/transport/outbox"
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
	// The baseFactory already ran the approval migration; run the outbox
	// migration so sys_event_outbox exists for PendingBindingFailures queries.
	s.Require().NoError(
		ioutbox.Migrate(s.ctx, s.db, s.env.DS.Kind),
		"Should run outbox migration",
	)

	s.handler = query.NewGetMetricsHandler(s.db)

	// ── Fixtures ──────────────────────────────────────────────────────────

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "met-flow", 1)

	// Two tenant-t1 instances: one running (non-final), one approved (final,
	// with finished_at set so AvgCompletionSeconds is computable).
	now := timex.Now()

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
		FinishedAt:    &now,
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
		FinishedAt:    &now,
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
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	// Two undelivered binding-failure outbox records.
	bindingRecords := []outboxmodel.Record{
		{
			EventID:    "evt-bf-001",
			EventType:  approval.EventTypeInstanceBindingFailed,
			Source:     "test",
			Payload:    []byte(`{"instanceId":"inst-1","tenantId":"t1"}`),
			Status:     outboxmodel.StatusPending,
			OccurredAt: now,
		},
		{
			EventID:    "evt-bf-002",
			EventType:  approval.EventTypeInstanceBindingFailed,
			Source:     "test",
			Payload:    []byte(`{"instanceId":"inst-2","tenantId":"t1"}`),
			Status:     outboxmodel.StatusFailed,
			OccurredAt: now,
		},
		// A completed binding-failure record should NOT be counted.
		{
			EventID:    "evt-bf-003",
			EventType:  approval.EventTypeInstanceBindingFailed,
			Source:     "test",
			Payload:    []byte(`{"instanceId":"inst-3","tenantId":"t1"}`),
			Status:     outboxmodel.StatusCompleted,
			OccurredAt: now,
		},
	}
	for i := range bindingRecords {
		_, err := s.db.NewInsert().Model(&bindingRecords[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert outbox record")
	}
}

func (s *GetMetricsTestSuite) TearDownSuite() {
	// Remove outbox records before cleaning approval data.
	_, _ = s.db.NewDelete().
		Model((*outboxmodel.Record)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)

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

	// AvgCompletionSeconds: 2 completed instances (approved + rejected) exist.
	// Both have finished_at == created_at (insertion time), so the avg is near 0
	// but must not be the sentinel -1.
	s.Assert().GreaterOrEqual(metrics.AvgCompletionSeconds, float64(0), "AvgCompletionSeconds should be >= 0 when completed instances exist")

	// PendingBindingFailures: 2 undelivered (pending + failed), 1 completed excluded.
	s.Assert().Equal(2, metrics.PendingBindingFailures, "Should count 2 pending binding failures")
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

	// AvgCompletionSeconds for t1: 1 approved instance with finished_at.
	s.Assert().GreaterOrEqual(metrics.AvgCompletionSeconds, float64(0), "AvgCompletionSeconds should be >= 0 for t1")
}

func (s *GetMetricsTestSuite) TestEmptyTenantReturnsNegativeOneAvg() {
	// A tenant with no completed instances should return the -1 sentinel.
	emptyTenant := "empty-tenant"
	metrics, err := s.handler.Handle(s.ctx, query.GetMetricsQuery{TenantID: emptyTenant})
	s.Require().NoError(err, "Should query empty-tenant metrics without error")

	s.Assert().Equal(-1.0, metrics.AvgCompletionSeconds, "Should return -1 sentinel when no completed instances exist")
	s.Assert().Empty(metrics.InstanceCounts, "Should return empty instance counts for unknown tenant")
	s.Assert().Equal(0, metrics.TimeoutTaskCount, "Should return 0 timeout tasks for unknown tenant")
}
