package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
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

// CaptureBus is an event.Bus recording published events.
type CaptureBus struct {
	mu     sync.Mutex
	events []event.Event
}

func (b *CaptureBus) Publish(_ context.Context, evt event.Event, _ ...event.PublishOption) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, evt)

	return nil
}

func (b *CaptureBus) PublishBatch(ctx context.Context, evts []event.Event, opts ...event.PublishOption) error {
	for _, evt := range evts {
		if err := b.Publish(ctx, evt, opts...); err != nil {
			return err
		}
	}

	return nil
}

func (*CaptureBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

// Published returns a snapshot of the captured events.
func (b *CaptureBus) Published() []event.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]event.Event(nil), b.events...)
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
		engine:   NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(new(CaptureBus))),
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
		NextFireAtUnixMs:  unixMillisPtr(due),
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
		StartedAtUnixMs:   unixMillisPtr(scheduledAt),
		HeartbeatAtUnixMs: unixMillisPtr(heartbeatAt),
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
