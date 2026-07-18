package store_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
)

// TestModuleBoot exercises the store through the full application graph: the
// production FX wiring, migration on start, default-schedule seeding, and a
// live fire through the DI-built engine.
func TestModuleBoot(t *testing.T) {
	t.Run("EnabledStoreSeedsAndFires", func(t *testing.T) {
		var executions atomic.Int32

		var manager cron.ScheduleManager

		_, cleanup := apptest.NewTestApp(t,
			fx.Replace(&config.CronConfig{Store: config.CronStoreConfig{
				Enabled:           true,
				AutoMigrate:       true,
				PollInterval:      20 * time.Millisecond,
				HeartbeatInterval: 25 * time.Millisecond,
				AbandonedAfter:    50 * time.Millisecond,
			}}),
			fx.Provide(fx.Annotate(
				func() cron.JobHandler {
					return cron.NewJobHandler("boot.probe",
						func(context.Context, cron.Execution) error {
							executions.Add(1)

							return nil
						},
						cron.WithDefaultSchedule(cron.ScheduleSpec{Trigger: cron.Every(time.Minute)}))
				},
				fx.ResultTags(`group:"vef:cron:job_handlers"`),
			)),
			fx.Populate(&manager),
		)
		defer cleanup()

		seeded, err := manager.Get(context.Background(), "boot.probe")
		require.NoError(t, err, "the shipped default schedule must be seeded at boot")
		assert.Equal(t, "boot.probe", seeded.JobName, "the seeded schedule must reference its handler")

		require.NoError(t, manager.TriggerNow(context.Background(), "boot.probe"),
			"a manual fire should be accepted")
		require.Eventually(t, func() bool { return executions.Load() >= 1 },
			5*time.Second, 20*time.Millisecond, "the DI-built engine must execute the fire")

		runs, err := manager.ListRuns(context.Background(), cron.RunFilter{ScheduleName: "boot.probe"})
		require.NoError(t, err, "listing runs should succeed")
		require.NotEmpty(t, runs, "the fire must be journaled")
	})

	t.Run("DisabledStoreDegradesGracefully", func(t *testing.T) {
		var manager cron.ScheduleManager

		_, cleanup := apptest.NewTestApp(t,
			fx.Provide(fx.Annotate(
				func() cron.JobHandler {
					return cron.NewJobHandler("idle.probe",
						func(context.Context, cron.Execution) error { return nil })
				},
				fx.ResultTags(`group:"vef:cron:job_handlers"`),
			)),
			fx.Populate(&manager),
		)
		defer cleanup()

		_, err := manager.Get(context.Background(), "anything")
		require.ErrorIs(t, err, cron.ErrStoreDisabled,
			"with the store off the manager must report the capability disabled")
	})
}
