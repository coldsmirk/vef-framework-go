package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// newStoreDB creates a migrated SQLite store database.
func newStoreDB(t *testing.T) orm.DB {
	t.Helper()

	db := testx.NewTestDB(t)
	require.NoError(t, migration.Migrate(context.Background(), db, config.SQLite),
		"Cron store migration should provision the tables")

	return db
}

// ClaimDue drives one claim batch and returns only the executable fires —
// the shape most claim assertions consume.
func (c *claimer) ClaimDue(ctx context.Context, limit int) ([]claimedFire, error) {
	batch, err := c.claimDueBatch(ctx, limit)

	return batch.fires, err
}

// stubEngineClock pins the engine's clock and its claimer's shared copy.
func stubEngineClock(engine *Engine, at time.Time) {
	engine.now = fixedNow(at)
	engine.claimer.now = engine.now
}

// mustRegistry builds a registry from handlers, failing the test on error.
func mustRegistry(t *testing.T, handlers ...cron.JobHandler) *Registry {
	t.Helper()

	registry, err := NewRegistry(handlers)
	require.NoError(t, err, "Registry construction should succeed")

	return registry
}

// newTestManager builds a schedule manager over the given store, wired to a
// real engine so mutations take the same wake path production does.
func newTestManager(db orm.DB, registry *Registry, now func() time.Time) *scheduleManager {
	return &scheduleManager{
		db:       db,
		registry: registry,
		engine:   NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus())),
		now:      now,
	}
}

// noopHandler is a handler that succeeds without doing anything.
func noopHandler(name string) cron.JobHandler {
	return cron.NewJobHandler(name, func(context.Context, cron.Execution) error { return nil })
}

// fastStoreConfig returns an enabled store config with test-friendly pacing.
func fastStoreConfig() *config.CronStoreConfig {
	return &config.CronStoreConfig{
		Enabled:           true,
		PollInterval:      20 * time.Millisecond,
		BatchSize:         8,
		MaxConcurrent:     4,
		HeartbeatInterval: 25 * time.Millisecond,
		AbandonedAfter:    50 * time.Millisecond,
	}
}

// scheduleFixture builds an enabled interval schedule due at the given time.
func scheduleFixture(name, jobName string, due time.Time) *cron.Schedule {
	created := timex.DateTime(due.Add(-time.Hour))

	schedule := &cron.Schedule{
		Name:              name,
		JobName:           jobName,
		Kind:              cron.TriggerInterval,
		EveryMs:           time.Minute.Milliseconds(),
		MisfirePolicy:     cron.MisfireFireNow,
		ConcurrencyPolicy: cron.ConcurrencyForbid,
		IsEnabled:         true,
		AnchorAtUnixMs:    due.Add(-time.Hour).UnixMilli(),
		NextFireAtUnixMs:  unixMsPtr(due),
	}
	schedule.CreatedAt = created
	schedule.UpdatedAt = created

	return schedule
}

// insertSchedule persists a fixture schedule.
func insertSchedule(t *testing.T, db orm.DB, schedule *cron.Schedule) *cron.Schedule {
	t.Helper()

	_, err := db.NewInsert().Model(schedule).Exec(context.Background())
	require.NoError(t, err, "Schedule fixture insert should succeed")

	return schedule
}

// insertRunningRun persists a running journal fixture with the given logical
// fire and heartbeat instants.
func insertRunningRun(t *testing.T, db orm.DB, schedule *cron.Schedule, scheduledAt, heartbeatAt time.Time) *cron.Run {
	t.Helper()

	run := &cron.Run{
		ScheduleID:        schedule.ID,
		ScheduleName:      schedule.Name,
		JobName:           schedule.JobName,
		ScheduledAtUnixMs: scheduledAt.UnixMilli(),
		ClaimedAtUnixMs:   scheduledAt.UnixMilli(),
		StartedAtUnixMs:   unixMsPtr(scheduledAt),
		HeartbeatAtUnixMs: unixMsPtr(heartbeatAt),
		Status:            cron.RunRunning,
		NodeID:            "node-dead",
	}

	_, err := db.NewInsert().Model(run).Exec(context.Background())
	require.NoError(t, err, "Running run fixture insert should succeed")

	return run
}

// loadRuns returns every journal row of the schedule, oldest first.
func loadRuns(t *testing.T, db orm.DB, scheduleID string) []cron.Run {
	t.Helper()

	var runs []cron.Run

	require.NoError(t, db.NewSelect().
		Model(&runs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("schedule_id", scheduleID) }).
		OrderBy("scheduled_at_unix_ms", "id").
		Scan(context.Background()),
		"Loading journal rows should succeed")

	return runs
}

// loadFireRequests returns the schedule's pending explicit fires in logical order.
func loadFireRequests(t *testing.T, db orm.DB, scheduleID string) []fireRequest {
	t.Helper()

	var requests []fireRequest

	require.NoError(t, db.NewSelect().
		Model(&requests).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("schedule_id", scheduleID) }).
		OrderBy("scheduled_at_unix_ms", "id").
		Scan(context.Background()),
		"Loading fire requests should succeed")

	return requests
}

func insertFireRequest(t *testing.T, db orm.DB, request *fireRequest) *fireRequest {
	t.Helper()

	_, err := db.NewInsert().Model(request).Exec(context.Background())
	require.NoError(t, err, "Fire request fixture insert should succeed")

	return request
}

// reloadSchedule returns the schedule's current row state.
func reloadSchedule(t *testing.T, db orm.DB, id string) *cron.Schedule {
	t.Helper()

	schedule := new(cron.Schedule)
	schedule.ID = id

	require.NoError(t, db.NewSelect().Model(schedule).WherePK().Scan(context.Background()),
		"Reloading the schedule should succeed")

	return schedule
}

// fixedNow returns a deterministic clock.
func fixedNow(at time.Time) func() time.Time {
	return func() time.Time { return at }
}
