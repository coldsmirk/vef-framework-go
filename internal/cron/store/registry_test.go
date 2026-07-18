package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
)

func TestNewRegistry(t *testing.T) {
	t.Run("IndexesAndSortsNames", func(t *testing.T) {
		registry := mustRegistry(t, noopHandler("b.job"), noopHandler("a.job"))

		assert.Equal(t, []string{"a.job", "b.job"}, registry.Names(), "names must be sorted")

		handler, ok := registry.Lookup("a.job")
		require.True(t, ok, "registered handlers must resolve")
		assert.Equal(t, "a.job", handler.Name(), "lookup must return the matching handler")

		_, ok = registry.Lookup("missing")
		assert.False(t, ok, "unregistered names must not resolve")
		assert.False(t, registry.IsEmpty(), "a populated registry is not empty")
	})

	t.Run("DuplicateNameFails", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler("same"), noopHandler("same")})
		assert.ErrorIs(t, err, ErrJobHandlerDuplicate, "duplicate job names must fail construction")
	})

	t.Run("EmptyNameFails", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler("")})
		assert.ErrorIs(t, err, ErrJobHandlerNameEmpty, "a blank job name must fail construction")
	})

	t.Run("AllReturnsNameOrder", func(t *testing.T) {
		registry := mustRegistry(t, noopHandler("z"), noopHandler("a"))

		handlers := registry.All()
		require.Len(t, handlers, 2, "all handlers must be returned")
		assert.Equal(t, "a", handlers[0].Name(), "handlers must come back in name order")
		assert.Equal(t, "z", handlers[1].Name(), "handlers must come back in name order")
	})

	t.Run("EmptyRegistry", func(t *testing.T) {
		registry := mustRegistry(t)

		assert.True(t, registry.IsEmpty(), "a registry without handlers is empty")
		assert.Empty(t, registry.Names(), "no names are registered")
	})
}

func TestSeedDefaultSchedules(t *testing.T) {
	db := newStoreDB(t)
	seeded := cron.NewJobHandler("report.daily",
		func(context.Context, cron.Execution) error { return nil },
		cron.WithDefaultSchedule(cron.ScheduleSpec{Trigger: cron.Expr("0 2 * * *", "Asia/Shanghai")}))
	registry := mustRegistry(t, seeded, noopHandler("plain.job"))
	manager := NewScheduleManager(db, true, registry, NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(new(captureBus))))

	require.NoError(t, SeedDefaultSchedules(context.Background(), manager, registry),
		"seeding should succeed")

	schedule, err := manager.Get(context.Background(), "report.daily")
	require.NoError(t, err, "the seeded schedule must exist")
	assert.Equal(t, "report.daily", schedule.JobName, "the job name must default to the handler name")
	assert.Equal(t, "0 2 * * *", schedule.Expr, "the shipped trigger must persist")
	require.NotNil(t, schedule.NextFireAt, "the seeded schedule must be armed")

	schedules, err := manager.List(context.Background(), cron.ScheduleFilter{})
	require.NoError(t, err, "listing should succeed")
	assert.Len(t, schedules, 1, "handlers without a default schedule must seed nothing")

	// Re-seeding — a restart, or a peer booting concurrently — must not
	// duplicate or overwrite.
	require.NoError(t, manager.Pause(context.Background(), "report.daily"), "pausing should succeed")
	require.NoError(t, SeedDefaultSchedules(context.Background(), manager, registry),
		"re-seeding should succeed")

	schedule, err = manager.Get(context.Background(), "report.daily")
	require.NoError(t, err, "the schedule must still exist")
	assert.False(t, schedule.IsEnabled, "re-seeding must never overwrite operator state")
}
