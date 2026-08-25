package binding

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	internalorm "github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// SpyBus records publish options used by worker failure notifications.
type SpyBus struct {
	capturedTypes  []string
	capturedGroups []string
	publishErrs    []error
	publishCalls   []event.PublishConfig
}

func (b *SpyBus) Subscribe(eventType string, _ event.Handler, opts ...event.SubscribeOption) (event.Unsubscribe, error) {
	cfg := event.ApplySubscribeOptions(opts)

	b.capturedTypes = append(b.capturedTypes, eventType)
	b.capturedGroups = append(b.capturedGroups, cfg.Group)

	return func() {}, nil
}

func (b *SpyBus) Publish(_ context.Context, _ event.Event, opts ...event.PublishOption) error {
	b.publishCalls = append(b.publishCalls, event.ApplyPublishOptions(opts))
	if len(b.publishErrs) == 0 {
		return nil
	}

	err := b.publishErrs[0]
	b.publishErrs = b.publishErrs[1:]

	return err
}

func (*SpyBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

// StatementQuietMark records whether one executed statement carried the ORM
// quiet-SQL-log mark.
type StatementQuietMark struct {
	Query string
	Quiet bool
}

// QuietMarkRecordingHook captures the quiet-SQL-log mark of every statement so
// tests can assert which context the worker used for which table.
type QuietMarkRecordingHook struct {
	statements []StatementQuietMark
}

func (*QuietMarkRecordingHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *QuietMarkRecordingHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	h.statements = append(h.statements, StatementQuietMark{Query: event.Query, Quiet: orm.IsQuietSQLLog(ctx)})
}

// marksFor returns the recorded marks of every statement naming the table.
func (h *QuietMarkRecordingHook) marksFor(table string) []bool {
	var marks []bool

	for _, statement := range h.statements {
		if strings.Contains(statement.Query, table) {
			marks = append(marks, statement.Quiet)
		}
	}

	return marks
}

func bindingFailureInstance() *approval.Instance {
	instance := &approval.Instance{TenantID: "tenant-1", FlowID: "flow-1", Status: approval.InstanceApproved}
	instance.ID = "inst-1"

	return instance
}

func TestWorkerPublishFailure(t *testing.T) {
	t.Run("UsesTxWhenDatabaseAvailable", func(t *testing.T) {
		bus := &SpyBus{}
		worker := NewWorker(testx.NewTestDB(t), bus, NewWriter(), nil)

		err := worker.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			bindingFailureInstance(), approval.BindingTriggerCompleted, approval.InstanceApproved, "biz_table", "boom",
		))

		require.NoError(t, err, "Binding failure should publish through a short transaction when DB is available")
		require.Len(t, bus.publishCalls, 1, "Binding failure should publish once to avoid outbox plus memory double delivery")
		assert.NotNil(t, bus.publishCalls[0].Tx, "Publish options should include Tx so routing selects only outbox")
	})

	t.Run("FallsBackWhenNoTransactionalRoute", func(t *testing.T) {
		bus := &SpyBus{publishErrs: []error{event.ErrTxRequired}}
		worker := NewWorker(testx.NewTestDB(t), bus, NewWriter(), nil)

		err := worker.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			bindingFailureInstance(), approval.BindingTriggerCompleted, approval.InstanceApproved, "biz_table", "boom",
		))

		require.NoError(t, err, "Binding failure should fall back to non-transactional publish when no Tx route exists")
		require.Len(t, bus.publishCalls, 2, "Binding failure should retry once without Tx after ErrTxRequired")
		assert.NotNil(t, bus.publishCalls[0].Tx, "First publish attempt should include Tx")
		assert.Nil(t, bus.publishCalls[1].Tx, "Fallback publish should not include Tx")
	})

	t.Run("PublishesDirectlyWithoutDatabase", func(t *testing.T) {
		bus := &SpyBus{}
		worker := NewWorker(nil, bus, NewWriter(), nil)

		err := worker.publishFailure(t.Context(), approval.NewInstanceBindingFailedEvent(
			bindingFailureInstance(), approval.BindingTriggerCompleted, approval.InstanceApproved, "biz_table", "boom",
		))

		require.NoError(t, err, "Binding failure should publish directly when DB is unavailable")
		require.Len(t, bus.publishCalls, 1, "Binding failure should publish exactly once")
		assert.Nil(t, bus.publishCalls[0].Tx, "Publish options should not include Tx when DB is unavailable")
	})
}

func TestWorkerEventuallyConvergesAfterFailure(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Approval migration should prepare projection storage")

		_, err := env.DB.NewRaw(`CREATE TABLE binding_worker_order (
			id VARCHAR(64) PRIMARY KEY,
			approval_status VARCHAR(32),
			apv_instance_id VARCHAR(32)
		)`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should create the business table")

		instanceIDColumn := "apv_instance_id"
		startedAt := timex.Now()
		finishedAt := timex.Now()
		projection := &approval.BusinessProjection{
			TenantID:        "tenant-1",
			FlowID:          "flow-1",
			FlowVersionID:   "version-1",
			OwnerInstanceID: "instance-1",
			TargetHash:      "binding-worker-order-1",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "binding_worker_order",
				KeyColumns:       []string{"id"},
				StatusColumn:     "approval_status",
				InstanceIDColumn: &instanceIDColumn,
				StatusMapping: map[approval.InstanceStatus]string{
					approval.InstanceApproved: "accepted",
				},
			},
			RecordKey:         []byte(`[{"column":"id","kind":"string","value":"order-1"}]`),
			DesiredStatus:     approval.InstanceApproved,
			DesiredStartedAt:  startedAt,
			DesiredFinishedAt: &finishedAt,
			DesiredRevision:   1,
			Status:            approval.BindingProjectionPending,
		}
		_, err = env.DB.NewInsert().Model(projection).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should insert a pending projection")

		cfg := &config.ApprovalConfig{BusinessBinding: config.ApprovalBusinessBindingConfig{
			Consistency: config.ApprovalBindingEventual,
			BatchSize:   10,
		}}
		bus := new(SpyBus)
		worker := NewWorker(env.DB, bus, NewWriter(), cfg)

		processed, err := worker.ProcessPending(env.Ctx)
		require.NoError(t, err, "A business write failure should be persisted instead of escaping the worker")
		require.Equal(t, 1, processed, "Worker should process the claimed projection")

		failed := new(approval.BusinessProjection)
		failed.ID = projection.ID
		require.NoError(t, env.DB.NewSelect().Model(failed).WherePK().Scan(env.Ctx),
			"Failed projection should remain queryable")
		assert.Equal(t, approval.BindingProjectionFailed, failed.Status,
			"Missing business target should move the projection to failed")
		assert.Equal(t, 1, failed.AttemptCount, "Failed attempt should be counted")
		assert.NotNil(t, failed.NextAttemptAt, "Failed projection should schedule a retry")
		require.NotNil(t, failed.LastError, "Failed projection should retain the write error")
		assert.NotEmpty(t, *failed.LastError, "Failed projection should retain a non-empty write error")
		assert.Empty(t, bus.publishCalls,
			"A missing owner instance should not produce an incomplete failure event")

		_, err = env.DB.NewRaw(`INSERT INTO binding_worker_order (id, approval_status)
			VALUES ('order-1', 'submitted')`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should create the previously missing business target")

		_, err = env.DB.NewUpdate().
			Model((*approval.BusinessProjection)(nil)).
			Set("next_attempt_at", nil).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(projection.ID) }).
			Exec(env.Ctx)
		require.NoError(t, err, "Test setup should make the failed projection immediately retryable")

		processed, err = worker.ProcessPending(env.Ctx)
		require.NoError(t, err, "Retry should converge after the business target becomes available")
		require.Equal(t, 1, processed, "Worker should reclaim the retryable projection")

		applied := new(approval.BusinessProjection)
		applied.ID = projection.ID
		require.NoError(t, env.DB.NewSelect().Model(applied).WherePK().Scan(env.Ctx),
			"Applied projection should remain queryable")
		assert.Equal(t, approval.BindingProjectionApplied, applied.Status,
			"Successful retry should mark the projection applied")
		assert.Equal(t, applied.DesiredRevision, applied.AppliedRevision,
			"Successful retry should apply the latest desired revision")
		require.NotNil(t, applied.AppliedOwnerInstanceID,
			"Successful retry should record the applied owner")
		assert.Equal(t, projection.OwnerInstanceID, *applied.AppliedOwnerInstanceID,
			"Applied owner should match the desired owner")
		assert.Zero(t, applied.AttemptCount, "Successful convergence should clear the attempt count")
		assert.Nil(t, applied.NextAttemptAt, "Successful convergence should clear the retry schedule")
		assert.Nil(t, applied.LastError, "Successful convergence should clear the last error")

		var status, owner string
		require.NoError(t, env.DB.NewRaw(`SELECT approval_status, apv_instance_id
			FROM binding_worker_order WHERE id = 'order-1'`).Scan(env.Ctx, &status, &owner),
			"Business target should be readable after convergence")
		assert.Equal(t, "accepted", status,
			"Worker should project the persisted binding snapshot's mapped status")
		assert.Equal(t, projection.OwnerInstanceID, owner,
			"Worker should stamp the owning approval instance as the fencing token")
	})
}

func TestWorkerPersistsSQLFailureAfterSavepointRollback(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Approval migration should prepare projection storage")

		_, err := env.DB.NewRaw(`CREATE TABLE binding_worker_sql_error (
			id VARCHAR(64) PRIMARY KEY,
			apv_instance_id VARCHAR(32)
		)`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should create the business table")

		_, err = env.DB.NewRaw(`INSERT INTO binding_worker_sql_error (id)
			VALUES ('order-1')`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should seed the business target")

		instanceIDColumn := "apv_instance_id"
		projection := &approval.BusinessProjection{
			TenantID:        "tenant-1",
			FlowID:          "flow-1",
			FlowVersionID:   "version-1",
			OwnerInstanceID: "instance-1",
			TargetHash:      "binding-worker-sql-error",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "binding_worker_sql_error",
				KeyColumns:       []string{"id"},
				StatusColumn:     "missing_approval_status",
				InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:        []byte(`[{"column":"id","kind":"string","value":"order-1"}]`),
			DesiredStatus:    approval.InstanceRunning,
			DesiredStartedAt: timex.Now(),
			DesiredRevision:  1,
			Status:           approval.BindingProjectionPending,
		}
		_, err = env.DB.NewInsert().Model(projection).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should insert the pending projection")

		worker := NewWorker(env.DB, new(SpyBus), NewWriter(), nil)
		processed, err := worker.ProcessPending(env.Ctx)
		require.NoError(t, err,
			"A host SQL error should roll back its savepoint and persist the failed projection")
		require.Equal(t, 1, processed, "Worker should finish the claimed projection attempt")

		reloaded := new(approval.BusinessProjection)
		reloaded.ID = projection.ID
		require.NoError(t, env.DB.NewSelect().Model(reloaded).WherePK().Scan(env.Ctx),
			"Failed projection should remain queryable after the host SQL error")
		assert.Equal(t, approval.BindingProjectionFailed, reloaded.Status,
			"Host SQL error should persist failed convergence state")
		assert.Equal(t, 1, reloaded.AttemptCount,
			"Host SQL error should increment the durable attempt count")
		require.NotNil(t, reloaded.LastError, "Host SQL error should be retained for operators")
		assert.NotEmpty(t, *reloaded.LastError, "Retained host SQL error should not be blank")
	})
}

func TestWorkerClaimBatchUsesDialectIndependentAttemptOrder(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Approval migration should prepare projection storage")

		now := timex.Now()
		failedCreated := now.Add(-3 * time.Hour)
		failedDue := now.Add(-2 * time.Hour)
		pendingCreated := now.Add(-time.Hour)
		instanceIDColumn := "apv_instance_id"
		bindingConfig := &approval.BusinessBindingConfig{
			TableName:        "projection_order_target",
			KeyColumns:       []string{"id"},
			StatusColumn:     "approval_status",
			InstanceIDColumn: &instanceIDColumn,
		}

		failed := &approval.BusinessProjection{
			CreatedAt:        failedCreated,
			UpdatedAt:        failedCreated,
			TenantID:         "tenant-1",
			FlowID:           "flow-1",
			FlowVersionID:    "version-1",
			OwnerInstanceID:  "instance-failed",
			TargetHash:       "projection-order-failed",
			Consistency:      config.ApprovalBindingEventual,
			Binding:          bindingConfig,
			RecordKey:        []byte(`[{"column":"id","kind":"string","value":"failed"}]`),
			DesiredStatus:    approval.InstanceRunning,
			DesiredStartedAt: now,
			DesiredRevision:  1,
			Status:           approval.BindingProjectionFailed,
			NextAttemptAt:    &failedDue,
		}
		_, err := env.DB.NewInsert().Model(failed).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should insert the due failed projection")

		pending := &approval.BusinessProjection{
			CreatedAt:        pendingCreated,
			UpdatedAt:        pendingCreated,
			TenantID:         "tenant-1",
			FlowID:           "flow-1",
			FlowVersionID:    "version-1",
			OwnerInstanceID:  "instance-pending",
			TargetHash:       "projection-order-pending",
			Consistency:      config.ApprovalBindingEventual,
			Binding:          bindingConfig,
			RecordKey:        []byte(`[{"column":"id","kind":"string","value":"pending"}]`),
			DesiredStatus:    approval.InstanceRunning,
			DesiredStartedAt: now,
			DesiredRevision:  1,
			Status:           approval.BindingProjectionPending,
		}
		_, err = env.DB.NewInsert().Model(pending).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should insert the legacy NULL-scheduled projection")

		cfg := &config.ApprovalConfig{BusinessBinding: config.ApprovalBusinessBindingConfig{BatchSize: 1}}
		worker := NewWorker(env.DB, new(SpyBus), NewWriter(), cfg)

		// Capture the persisted pre-claim update time: scanned timestamps and
		// in-memory timex.Now() values live in different clock domains (the
		// column round-trips as a zone-less wall clock), so the refresh
		// assertion below must compare persisted against persisted.
		preClaim := new(approval.BusinessProjection)
		preClaim.ID = failed.ID
		require.NoError(t, env.DB.NewSelect().Model(preClaim).WherePK().Scan(env.Ctx),
			"Inserted projection should be queryable before the claim")

		claimed, err := worker.claimBatch(env.Ctx)
		require.NoError(t, err, "Worker should claim one eligible projection")
		require.Len(t, claimed, 1, "Configured batch size should limit the claim")
		assert.Equal(t, failed.ID, claimed[0].ID,
			"The oldest effective attempt time should win regardless of NULL ordering rules")

		reloaded := new(approval.BusinessProjection)
		reloaded.ID = failed.ID
		require.NoError(t, env.DB.NewSelect().Model(reloaded).WherePK().Scan(env.Ctx),
			"Claimed projection should remain queryable")
		assert.True(t, reloaded.UpdatedAt.Unwrap().After(preClaim.UpdatedAt.Unwrap()),
			"Claiming a projection should refresh its operator-facing update time")
	})
}

// TestWorkerRunKeepsHostTableWritesOutOfTheQuietSQLLog pins the mark boundary of
// the polling loop: its own projection bookkeeping repeats every tick and stays
// demoted, while the write it applies to the host's business row keeps the log
// level the synchronous lane gives that same statement from a request handler.
func TestWorkerRunKeepsHostTableWritesOutOfTheQuietSQLLog(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"Approval migration should prepare projection storage")

		_, err := env.DB.NewRaw(`CREATE TABLE binding_worker_quiet_mark (
			id VARCHAR(64) PRIMARY KEY,
			approval_status VARCHAR(32),
			apv_instance_id VARCHAR(32)
		)`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should create the business table")

		_, err = env.DB.NewRaw(`INSERT INTO binding_worker_quiet_mark (id) VALUES ('order-1')`).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should seed the business target")

		instanceIDColumn := "apv_instance_id"
		projection := &approval.BusinessProjection{
			TenantID:        "tenant-1",
			FlowID:          "flow-1",
			FlowVersionID:   "version-1",
			OwnerInstanceID: "instance-1",
			TargetHash:      "binding-worker-quiet-mark",
			Consistency:     config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName:        "binding_worker_quiet_mark",
				KeyColumns:       []string{"id"},
				StatusColumn:     "approval_status",
				InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:        []byte(`[{"column":"id","kind":"string","value":"order-1"}]`),
			DesiredStatus:    approval.InstanceRunning,
			DesiredStartedAt: timex.Now(),
			DesiredRevision:  1,
			Status:           approval.BindingProjectionPending,
		}
		_, err = env.DB.NewInsert().Model(projection).Exec(env.Ctx)
		require.NoError(t, err, "Test setup should insert the pending projection")

		// The hook has to be installed before orm.New derives its named-arg
		// clone, so the worker runs against a second handle on the same pool.
		hook := new(QuietMarkRecordingHook)
		dialect, err := internalorm.DialectFor(env.DS.Kind)
		require.NoError(t, err, "Test setup should resolve the bun dialect")

		bunDB := bun.NewDB(env.RawDB, dialect, bun.WithDiscardUnknownColumns())
		bunDB.AddQueryHook(hook)

		NewWorker(internalorm.New(bunDB), new(SpyBus), NewWriter(), nil).Run(env.Ctx)

		reloaded := new(approval.BusinessProjection)
		reloaded.ID = projection.ID
		require.NoError(t, env.DB.NewSelect().Model(reloaded).WherePK().Scan(env.Ctx),
			"Projection should remain queryable after the run")
		require.Equal(t, approval.BindingProjectionApplied, reloaded.Status,
			"The run must apply the projection so the host write really happened")

		businessMarks := hook.marksFor("binding_worker_quiet_mark")
		require.NotEmpty(t, businessMarks, "The worker should have touched the host business table")
		assert.NotContains(t, businessMarks, true,
			"Host business statements must not inherit the polling loop's quiet mark")

		projectionMarks := hook.marksFor("apv_business_projection")
		require.NotEmpty(t, projectionMarks, "The worker should have touched its own projection table")
		assert.Contains(t, projectionMarks, true,
			"The worker's own bookkeeping must stay quiet")
	})
}
