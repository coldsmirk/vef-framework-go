package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ManagerSuite exercises the schedule manager against a migrated store.
type ManagerSuite struct {
	suite.Suite

	db      orm.DB
	manager *scheduleManager
	now     time.Time
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) SetupTest() {
	s.db = newStoreDB(s.T())
	s.now = time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)

	registry := mustRegistry(s.T(), noopHandler("orders.sync"), noopHandler("report.daily"))
	engine := NewEngine(s.db, fastStoreConfig(), registry, NewRunEventPublisher(new(captureBus)))

	s.manager = &scheduleManager{
		db:       s.db,
		registry: registry,
		engine:   engine,
		now:      func() time.Time { return s.now },
	}
}

func (*ManagerSuite) ctx() context.Context {
	return context.Background()
}

func (*ManagerSuite) validSpec(name string) cron.ScheduleSpec {
	return cron.ScheduleSpec{
		Name:    name,
		JobName: "orders.sync",
		Trigger: cron.Every(time.Minute),
	}
}

func (s *ManagerSuite) TestCreate() {
	s.Run("PersistsAndArms", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("sync"))
		s.Require().NoError(err, "creating a valid spec should succeed")

		s.Equal(cron.MisfireFireNow, schedule.MisfirePolicy, "the misfire policy must default")
		s.Equal(cron.ConcurrencyForbid, schedule.ConcurrencyPolicy, "the concurrency policy must default")
		s.True(schedule.IsEnabled, "enablement must default to true")
		s.Require().NotNil(schedule.NextFireAt, "an enabled schedule must be armed")
		s.True(schedule.NextFireAt.Unwrap().Equal(s.now.Add(time.Minute)),
			"the first fire lands one interval after creation")
	})

	s.Run("DuplicateName", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("dup"))
		s.Require().NoError(err, "the first create should succeed")

		_, err = s.manager.Create(s.ctx(), s.validSpec("dup"))
		s.Require().ErrorIs(err, cron.ErrScheduleExists, "a taken name must be rejected")
	})

	s.Run("ValidationFailures", func() {
		cases := []struct {
			name    string
			mutate  func(*cron.ScheduleSpec)
			wantErr error
		}{
			{"blank name", func(spec *cron.ScheduleSpec) { spec.Name = "  " }, cron.ErrScheduleInvalid("")},
			{"unregistered job", func(spec *cron.ScheduleSpec) { spec.JobName = "ghost" }, cron.ErrJobNotRegistered},
			{"bad trigger", func(spec *cron.ScheduleSpec) { spec.Trigger = cron.Expr("nope", "") }, cron.ErrTriggerInvalid("")},
			{"bad misfire policy", func(spec *cron.ScheduleSpec) { spec.MisfirePolicy = "later" }, cron.ErrScheduleInvalid("")},
			{"bad concurrency policy", func(spec *cron.ScheduleSpec) { spec.ConcurrencyPolicy = "queue" }, cron.ErrScheduleInvalid("")},
			{"negative timeout", func(spec *cron.ScheduleSpec) { spec.Timeout = -time.Second }, cron.ErrScheduleInvalid("")},
			{
				"inverted window",
				func(spec *cron.ScheduleSpec) {
					starts := s.now.Add(time.Hour)
					ends := s.now
					spec.StartsAt, spec.EndsAt = &starts, &ends
				},
				cron.ErrScheduleInvalid(""),
			},
		}

		for _, tc := range cases {
			s.Run(tc.name, func() {
				spec := s.validSpec("invalid")
				tc.mutate(&spec)

				_, err := s.manager.Create(s.ctx(), spec)
				s.Require().ErrorIs(err, tc.wantErr, "the spec fault must map to its outward error")
			})
		}
	})

	s.Run("DisabledSpecStaysUnarmed", func() {
		disabled := false
		spec := s.validSpec("dormant")
		spec.Enabled = &disabled

		schedule, err := s.manager.Create(s.ctx(), spec)
		s.Require().NoError(err, "creating a disabled schedule should succeed")
		s.False(schedule.IsEnabled, "the schedule must be created paused")
		s.Nil(schedule.NextFireAt, "a paused schedule carries no next fire")
	})

	s.Run("RejectsInvalidRawParams", func() {
		spec := s.validSpec("raw")
		spec.Params = json.RawMessage(`{broken`)

		_, err := s.manager.Create(s.ctx(), spec)
		s.Require().ErrorIs(err, cron.ErrScheduleInvalid(""), "malformed raw params must be rejected")
	})
}

func (s *ManagerSuite) TestUpdate() {
	s.Run("ReshapesAndRearms", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("reshape"))
		s.Require().NoError(err, "the fixture create should succeed")

		spec := s.validSpec("reshape")
		spec.Trigger = cron.Expr("0 2 * * *", "Asia/Shanghai")
		spec.JobName = "report.daily"

		updated, err := s.manager.Update(s.ctx(), "reshape", spec)
		s.Require().NoError(err, "updating should succeed")
		s.Equal(cron.TriggerCron, updated.Kind, "the trigger kind must change")
		s.Equal("report.daily", updated.JobName, "the job must change")
		s.Require().NotNil(updated.NextFireAt, "the schedule must re-arm from now")
	})

	s.Run("Rename", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("old-name"))
		s.Require().NoError(err, "the fixture create should succeed")

		spec := s.validSpec("new-name")

		_, err = s.manager.Update(s.ctx(), "old-name", spec)
		s.Require().NoError(err, "renaming should succeed")

		_, err = s.manager.Get(s.ctx(), "old-name")
		s.Require().ErrorIs(err, cron.ErrScheduleNotFound, "the old name must be gone")

		renamed, err := s.manager.Get(s.ctx(), "new-name")
		s.Require().NoError(err, "the new name must resolve")
		s.Equal("new-name", renamed.Name, "the row must carry the new name")
	})

	s.Run("RenameOntoTakenName", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("a"))
		s.Require().NoError(err, "fixture a should be created")
		_, err = s.manager.Create(s.ctx(), s.validSpec("b"))
		s.Require().NoError(err, "fixture b should be created")

		_, err = s.manager.Update(s.ctx(), "a", s.validSpec("b"))
		s.Require().ErrorIs(err, cron.ErrScheduleExists, "renaming onto a taken name must conflict")
	})

	s.Run("UnknownSchedule", func() {
		_, err := s.manager.Update(s.ctx(), "ghost", s.validSpec("ghost"))
		s.Require().ErrorIs(err, cron.ErrScheduleNotFound, "updating a missing schedule must fail")
	})
}

func (s *ManagerSuite) TestPauseResume() {
	s.Run("PauseDisarms", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("pausable"))
		s.Require().NoError(err, "the fixture create should succeed")

		s.Require().NoError(s.manager.Pause(s.ctx(), "pausable"), "pausing should succeed")

		paused, err := s.manager.Get(s.ctx(), "pausable")
		s.Require().NoError(err, "the paused schedule must load")
		s.False(paused.IsEnabled, "pause must disable")
		s.Nil(paused.NextFireAt, "pause must disarm")
	})

	s.Run("ResumeWithMissedFireNowCatchesUp", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("catchup"))
		s.Require().NoError(err, "the fixture create should succeed")
		s.Require().NoError(s.manager.Pause(s.ctx(), "catchup"), "pausing should succeed")

		// Resume far past the next occurrence: fire_now runs one immediate catch-up.
		s.now = s.now.Add(2 * time.Hour)
		s.Require().NoError(s.manager.Resume(s.ctx(), "catchup"), "resuming should succeed")

		resumed, err := s.manager.Get(s.ctx(), "catchup")
		s.Require().NoError(err, "the resumed schedule must load")
		s.True(resumed.IsEnabled, "resume must re-enable")
		s.Require().NotNil(resumed.NextFireAt, "resume must re-arm")
		s.True(resumed.NextFireAt.AsLocal().Equal(s.now), "fire_now must schedule an immediate catch-up")
	})

	s.Run("ResumeWithMissedSkipWaits", func() {
		spec := s.validSpec("patient")
		spec.MisfirePolicy = cron.MisfireSkip

		_, err := s.manager.Create(s.ctx(), spec)
		s.Require().NoError(err, "the fixture create should succeed")
		s.Require().NoError(s.manager.Pause(s.ctx(), "patient"), "pausing should succeed")

		s.now = s.now.Add(2 * time.Hour)
		s.Require().NoError(s.manager.Resume(s.ctx(), "patient"), "resuming should succeed")

		resumed, err := s.manager.Get(s.ctx(), "patient")
		s.Require().NoError(err, "the resumed schedule must load")
		s.Require().NotNil(resumed.NextFireAt, "resume must re-arm")
		s.True(resumed.NextFireAt.AsLocal().After(s.now), "skip must wait for the next regular fire")
	})

	s.Run("ResumeEnabledIsIdempotent", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("already"))
		s.Require().NoError(err, "the fixture create should succeed")

		before, err := s.manager.Get(s.ctx(), "already")
		s.Require().NoError(err, "the schedule must load")

		s.Require().NoError(s.manager.Resume(s.ctx(), "already"), "resuming an enabled schedule is a no-op")

		after, err := s.manager.Get(s.ctx(), "already")
		s.Require().NoError(err, "the schedule must reload")
		s.True(before.NextFireAt.Equal(*after.NextFireAt), "a no-op resume must not move the fire")
	})
}

func (s *ManagerSuite) TestTriggerNow() {
	s.Run("PullsTheFireToNow", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("manual"))
		s.Require().NoError(err, "the fixture create should succeed")

		s.Require().NoError(s.manager.TriggerNow(s.ctx(), "manual"), "triggering should succeed")

		triggered, err := s.manager.Get(s.ctx(), "manual")
		s.Require().NoError(err, "the schedule must load")
		s.True(triggered.NextFireAt.AsLocal().Equal(s.now), "the fire must move to now")
	})

	s.Run("PausedScheduleRefuses", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("held"))
		s.Require().NoError(err, "the fixture create should succeed")
		s.Require().NoError(s.manager.Pause(s.ctx(), "held"), "pausing should succeed")

		err = s.manager.TriggerNow(s.ctx(), "held")
		s.Require().ErrorIs(err, cron.ErrScheduleDisabled, "a paused schedule must refuse manual fires")
	})

	s.Run("AlreadyDueIsANoOp", func() {
		schedule := insertSchedule(s.T(), s.db, scheduleFixture("due", "orders.sync", s.now.Add(-time.Minute)))

		s.Require().NoError(s.manager.TriggerNow(s.ctx(), "due"), "triggering should succeed")

		after := reloadSchedule(s.T(), s.db, schedule.ID)
		s.True(after.NextFireAt.AsLocal().Equal(s.now.Add(-time.Minute)),
			"an already-due fire must stay where it is")
	})
}

func (s *ManagerSuite) TestDeleteAndQueries() {
	s.Run("DeleteKeepsRuns", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("doomed"))
		s.Require().NoError(err, "the fixture create should succeed")

		run := &cron.Run{
			ScheduleID:   schedule.ID,
			ScheduleName: schedule.Name,
			JobName:      schedule.JobName,
			ScheduledAt:  timex.DateTime(s.now),
			Status:       cron.RunSucceeded,
		}
		_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
		s.Require().NoError(err, "the journal fixture insert should succeed")

		s.Require().NoError(s.manager.Delete(s.ctx(), "doomed"), "deleting should succeed")
		s.Require().ErrorIs(s.manager.Delete(s.ctx(), "doomed"), cron.ErrScheduleNotFound,
			"a second delete must report absence")

		runs, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{ScheduleName: "doomed"})
		s.Require().NoError(err, "listing runs should succeed")
		s.Len(runs, 1, "journal rows must survive schedule deletion")
	})

	s.Run("ListFilters", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("list-a"))
		s.Require().NoError(err, "fixture list-a should be created")

		reportSpec := s.validSpec("list-b")
		reportSpec.JobName = "report.daily"
		_, err = s.manager.Create(s.ctx(), reportSpec)
		s.Require().NoError(err, "fixture list-b should be created")
		s.Require().NoError(s.manager.Pause(s.ctx(), "list-b"), "pausing list-b should succeed")

		byJob, err := s.manager.List(s.ctx(), cron.ScheduleFilter{JobName: "report.daily"})
		s.Require().NoError(err, "listing by job should succeed")
		s.Require().Len(byJob, 1, "only the matching job's schedule must return")
		s.Equal("list-b", byJob[0].Name, "the filter must match by job name")

		enabled := true
		byEnabled, err := s.manager.List(s.ctx(), cron.ScheduleFilter{Enabled: &enabled})
		s.Require().NoError(err, "listing by enablement should succeed")
		s.Require().Len(byEnabled, 1, "only the enabled schedule must return")
		s.Equal("list-a", byEnabled[0].Name, "the filter must match by enablement")
	})

	s.Run("ListRunsFiltersAndBounds", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("journal"))
		s.Require().NoError(err, "the fixture create should succeed")

		statuses := []cron.RunStatus{cron.RunSucceeded, cron.RunFailed, cron.RunMissed}
		for i, status := range statuses {
			run := &cron.Run{
				ScheduleID:   schedule.ID,
				ScheduleName: schedule.Name,
				JobName:      schedule.JobName,
				ScheduledAt:  timex.DateTime(s.now.Add(time.Duration(i) * time.Minute)),
				Status:       status,
			}
			_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
			s.Require().NoError(err, "the journal fixture insert should succeed")
		}

		failed, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Statuses: []cron.RunStatus{cron.RunFailed}})
		s.Require().NoError(err, "filtering by status should succeed")
		s.Require().Len(failed, 1, "only the failed run must return")
		s.Equal(cron.RunFailed, failed[0].Status, "the status filter must hold")

		since := s.now.Add(30 * time.Second)
		until := s.now.Add(90 * time.Second)
		windowed, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Since: &since, Until: &until})
		s.Require().NoError(err, "filtering by window should succeed")
		s.Require().Len(windowed, 1, "only the in-window run must return")
		s.Equal(cron.RunFailed, windowed[0].Status, "the window must select the middle run")

		bounded, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Limit: 2})
		s.Require().NoError(err, "limiting should succeed")
		s.Len(bounded, 2, "the limit must bound the page")
	})
}

func TestDisabledScheduleManager(t *testing.T) {
	manager := NewScheduleManager(nil, false, mustRegistry(t), nil)
	ctx := context.Background()

	calls := map[string]func() error{
		"Create": func() error {
			_, err := manager.Create(ctx, cron.ScheduleSpec{})

			return err
		},
		"Update": func() error {
			_, err := manager.Update(ctx, "x", cron.ScheduleSpec{})

			return err
		},
		"Delete":     func() error { return manager.Delete(ctx, "x") },
		"Pause":      func() error { return manager.Pause(ctx, "x") },
		"Resume":     func() error { return manager.Resume(ctx, "x") },
		"TriggerNow": func() error { return manager.TriggerNow(ctx, "x") },
		"Get": func() error {
			_, err := manager.Get(ctx, "x")

			return err
		},
		"List": func() error {
			_, err := manager.List(ctx, cron.ScheduleFilter{})

			return err
		},
		"ListRuns": func() error {
			_, err := manager.ListRuns(ctx, cron.RunFilter{})

			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, call(), cron.ErrStoreDisabled,
				"every method of the disabled manager must report the store off")
		})
	}
}
