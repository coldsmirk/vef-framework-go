package store_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	cronstore "github.com/coldsmirk/vef-framework-go/internal/cron/store"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ModuleBus is the inert event bus used by isolated module lifecycle tests.
type ModuleBus struct{}

func (*ModuleBus) Publish(context.Context, event.Event, ...event.PublishOption) error {
	return nil
}

func (*ModuleBus) PublishBatch(context.Context, []event.Event, ...event.PublishOption) error {
	return nil
}

func (*ModuleBus) Subscribe(string, event.Handler, ...event.SubscribeOption) (event.Unsubscribe, error) {
	return func() {}, nil
}

func moduleTestOptions(
	db orm.DB,
	cfg *config.CronConfig,
	handlers ...cron.JobHandler,
) []fx.Option {
	options := []fx.Option{
		fx.NopLogger,
		fx.Provide(func() orm.DB { return db }),
		fx.Provide(func() event.Bus { return new(ModuleBus) }),
		fx.Supply(
			cfg,
			&config.DataSourcesConfig{Map: map[string]config.DataSourceConfig{
				config.PrimaryDataSourceName: {Kind: config.SQLite},
			}},
		),
		cronstore.Module,
	}

	for _, handler := range handlers {
		options = append(options, fx.Provide(fx.Annotate(
			func() cron.JobHandler { return handler },
			fx.ResultTags(`group:"vef:cron:job_handlers"`),
		)))
	}

	return options
}

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
				AbandonedAfter:    2 * time.Second,
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
		require.NoError(t, err, "The shipped default schedule must be seeded at boot")
		assert.Equal(t, "boot.probe", seeded.JobName, "The seeded schedule must reference its handler")

		require.NoError(t, manager.TriggerNow(context.Background(), "boot.probe"),
			"A manual fire should be accepted")
		require.Eventually(t, func() bool { return executions.Load() >= 1 },
			5*time.Second, 20*time.Millisecond, "The DI-built engine must execute the fire")

		runs, err := manager.ListRuns(context.Background(), cron.RunFilter{ScheduleName: "boot.probe"})
		require.NoError(t, err, "Listing runs should succeed")
		require.NotEmpty(t, runs, "The fire must be journaled")
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
			"With the store off the manager must report the capability disabled")
	})
}

func TestModuleLifecycle(t *testing.T) {
	t.Run("RejectsOutdatedSchemaBeforeStarting", func(t *testing.T) {
		db := testx.NewTestDB(t)
		cfg := &config.CronConfig{Store: config.CronStoreConfig{Enabled: true}}
		app := fxtest.New(t, moduleTestOptions(db, cfg)...)

		err := app.Start(context.Background())
		require.ErrorIs(t, err, migration.ErrSchemaOutdated,
			"An enabled store must fail before seeding or starting against an outdated schema")
	})

	t.Run("PassesRemainingStopBudgetToEngine", func(t *testing.T) {
		handlerStarted := make(chan struct{})
		priorHookDone := make(chan struct{})
		db := testx.NewTestDB(t)
		require.NoError(t, migration.Migrate(context.Background(), db, config.SQLite),
			"The lifecycle fixture schema should migrate")

		handler := cron.NewJobHandler("orders.sync", func(ctx context.Context, _ cron.Execution) error {
			close(handlerStarted)
			<-ctx.Done()

			return ctx.Err()
		})
		cfg := &config.CronConfig{Store: config.CronStoreConfig{
			Enabled:           true,
			PollInterval:      20 * time.Millisecond,
			HeartbeatInterval: 25 * time.Millisecond,
			AbandonedAfter:    5 * time.Second,
		}}

		var manager cron.ScheduleManager

		options := moduleTestOptions(db, cfg, handler)
		options = append(options,
			fx.Populate(&manager),
			fx.Invoke(func(lifecycle fx.Lifecycle) {
				lifecycle.Append(fx.StopHook(func(context.Context) error {
					time.Sleep(150 * time.Millisecond)
					close(priorHookDone)

					return nil
				}))
			}),
		)
		app := fxtest.New(t, options...)
		t.Cleanup(app.RequireStop)
		app.RequireStart()

		schedule, err := manager.Create(context.Background(), cron.ScheduleSpec{
			Name:    "budget-cutoff",
			JobName: "orders.sync",
			Trigger: cron.Once(time.Now().Add(20 * time.Millisecond)),
		})
		require.NoError(t, err, "Creating the schedule should succeed")

		select {
		case <-handlerStarted:
		case <-time.After(5 * time.Second):
			t.Fatal("The handler must start before the lifecycle stops")
		}

		stopCtx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		defer cancel()

		require.NoError(t, app.Stop(stopCtx),
			"The cron lifecycle hook should reserve time to cancel and journal the run")
		assert.NoError(t, stopCtx.Err(), "The cron hook should finish inside the shared lifecycle budget")

		select {
		case <-priorHookDone:
		default:
			t.Fatal("The later hook must consume part of the shared stop budget first")
		}

		runs, err := manager.ListRuns(context.Background(), cron.RunFilter{ScheduleName: schedule.Name})
		require.NoError(t, err, "The interrupted run should remain queryable")
		require.Len(t, runs, 1, "The interrupted fire must stay journaled")
		assert.Equal(t, cron.RunCanceled, runs[0].Status,
			"The engine should journal cancellation before the lifecycle returns")
	})
}
