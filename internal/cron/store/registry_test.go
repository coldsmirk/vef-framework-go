package store

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
)

func TestNewRegistry(t *testing.T) {
	t.Run("IndexesAndSortsNames", func(t *testing.T) {
		registry := mustRegistry(t, noopHandler("b.job"), noopHandler("a.job"))

		assert.Equal(t, []string{"a.job", "b.job"}, registry.Names(), "Names must be sorted")

		handler, ok := registry.Lookup("a.job")
		require.True(t, ok, "Registered handlers must resolve")
		assert.Equal(t, "a.job", handler.Name(), "Lookup must return the matching handler")

		_, ok = registry.Lookup("missing")
		assert.False(t, ok, "Unregistered names must not resolve")
		assert.False(t, registry.IsEmpty(), "A populated registry is not empty")
	})

	t.Run("DuplicateNameFails", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler("same"), noopHandler("same")})
		assert.ErrorIs(t, err, ErrJobHandlerDuplicate, "Duplicate job names must fail construction")
	})

	t.Run("EmptyNameFails", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler("")})
		assert.ErrorIs(t, err, ErrJobHandlerNameEmpty, "A blank job name must fail construction")
	})

	t.Run("PersistedNameWidthFailsAtBoot", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler(strings.Repeat("j", maxJobNameLength+1))})
		assert.ErrorIs(t, err, ErrJobHandlerNameTooLong,
			"A job name wider than the persisted column must fail registry construction")
	})

	t.Run("PersistedNameWidthCountsCharacters", func(t *testing.T) {
		_, err := NewRegistry([]cron.JobHandler{noopHandler(strings.Repeat("名", maxJobNameLength))})
		require.NoError(t, err, "A multibyte job name at the character limit should register")

		_, err = NewRegistry([]cron.JobHandler{noopHandler(strings.Repeat("名", maxJobNameLength+1))})
		assert.ErrorIs(t, err, ErrJobHandlerNameTooLong,
			"A multibyte job name beyond the character limit should fail registry construction")
	})

	t.Run("AllReturnsNameOrder", func(t *testing.T) {
		registry := mustRegistry(t, noopHandler("z"), noopHandler("a"))

		handlers := registry.All()
		require.Len(t, handlers, 2, "All handlers must be returned")
		assert.Equal(t, "a", handlers[0].Name(), "Handlers must come back in name order")
		assert.Equal(t, "z", handlers[1].Name(), "Handlers must come back in name order")
	})

	t.Run("EmptyRegistry", func(t *testing.T) {
		registry := mustRegistry(t)

		assert.True(t, registry.IsEmpty(), "A registry without handlers is empty")
		assert.Empty(t, registry.Names(), "No names are registered")
	})
}

func TestSeedDefaultSchedules(t *testing.T) {
	db := newStoreDB(t)
	seeded := cron.NewJobHandler("report.daily",
		func(context.Context, cron.Execution) error { return nil },
		cron.WithDefaultSchedule(cron.ScheduleSpec{Trigger: cron.Expr("0 2 * * *", "Asia/Shanghai")}))
	registry := mustRegistry(t, seeded, noopHandler("plain.job"))
	manager := NewScheduleManager(db, true, registry, NewEngine(db, fastStoreConfig(), registry, NewRunEventPublisher(eventtest.NewFakeBus())))

	require.NoError(t, SeedDefaultSchedules(context.Background(), manager, registry),
		"Seeding should succeed")

	schedule, err := manager.Get(context.Background(), "report.daily")
	require.NoError(t, err, "The seeded schedule must exist")
	assert.Equal(t, "report.daily", schedule.JobName, "The job name must default to the handler name")
	assert.Equal(t, "0 2 * * *", schedule.Expr, "The shipped trigger must persist")
	require.NotNil(t, schedule.NextFireAtUnixMs, "The seeded schedule must be armed")

	schedules, err := manager.List(context.Background(), cron.ScheduleFilter{})
	require.NoError(t, err, "Listing should succeed")
	assert.Len(t, schedules, 1, "Handlers without a default schedule must seed nothing")

	// Re-seeding — a restart, or a peer booting concurrently — must not
	// duplicate or overwrite.
	require.NoError(t, manager.Pause(context.Background(), "report.daily"), "Pausing should succeed")
	require.NoError(t, SeedDefaultSchedules(context.Background(), manager, registry),
		"Re-seeding should succeed")

	schedule, err = manager.Get(context.Background(), "report.daily")
	require.NoError(t, err, "The schedule must still exist")
	assert.False(t, schedule.IsEnabled, "Re-seeding must never overwrite operator state")
}
