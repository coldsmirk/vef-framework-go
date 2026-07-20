package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ManagerTestSuite exercises the schedule manager against a migrated store.
type ManagerTestSuite struct {
	suite.Suite

	db      orm.DB
	manager *scheduleManager
	now     time.Time
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}

func (s *ManagerTestSuite) SetupTest() {
	s.setupStore()
}

func (s *ManagerTestSuite) SetupSubTest() {
	s.setupStore()
}

func (s *ManagerTestSuite) setupStore() {
	s.db = newStoreDB(s.T())
	s.now = time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)

	registry := mustRegistry(s.T(), noopHandler("orders.sync"), noopHandler("report.daily"))
	s.manager = newTestManager(s.db, registry, func() time.Time { return s.now })
}

func (*ManagerTestSuite) ctx() context.Context {
	return context.Background()
}

func (*ManagerTestSuite) validSpec(name string) cron.ScheduleSpec {
	return cron.ScheduleSpec{
		Name:    name,
		JobName: "orders.sync",
		Trigger: cron.Every(time.Minute),
	}
}

func (s *ManagerTestSuite) TestCreate() {
	s.Run("PersistsAndArms", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("sync"))
		s.Require().NoError(err, "Creating a valid spec should succeed")

		s.Equal(cron.MisfireFireNow, schedule.MisfirePolicy, "The misfire policy must default")
		s.Equal(cron.ConcurrencyForbid, schedule.ConcurrencyPolicy, "The concurrency policy must default")
		s.True(schedule.IsEnabled, "Enablement must default to true")
		s.Require().NotNil(schedule.NextFireAtUnixMs, "An enabled schedule must be armed")
		s.Equal(s.now.Add(time.Minute).UnixMilli(), *schedule.NextFireAtUnixMs,
			"The first fire lands one interval after creation")
	})

	s.Run("DuplicateName", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("dup"))
		s.Require().NoError(err, "The first create should succeed")

		_, err = s.manager.Create(s.ctx(), s.validSpec("dup"))
		s.Require().ErrorIs(err, cron.ErrScheduleExists, "A taken name must be rejected")
	})

	s.Run("ValidationFailures", func() {
		cases := []struct {
			name    string
			mutate  func(*cron.ScheduleSpec)
			wantErr error
		}{
			{"BlankName", func(spec *cron.ScheduleSpec) { spec.Name = "  " }, cron.ErrScheduleInvalid("")},
			{
				"NameBeyondPersistedWidth",
				func(spec *cron.ScheduleSpec) { spec.Name = strings.Repeat("n", maxScheduleNameLength+1) },
				cron.ErrScheduleInvalid(""),
			},
			{"UnregisteredJob", func(spec *cron.ScheduleSpec) { spec.JobName = "ghost" }, cron.ErrJobNotRegistered},
			{"BadTrigger", func(spec *cron.ScheduleSpec) { spec.Trigger = cron.Expr("nope", "") }, cron.ErrTriggerInvalid("")},
			{"BadMisfirePolicy", func(spec *cron.ScheduleSpec) { spec.MisfirePolicy = "later" }, cron.ErrScheduleInvalid("")},
			{"BadConcurrencyPolicy", func(spec *cron.ScheduleSpec) { spec.ConcurrencyPolicy = "queue" }, cron.ErrScheduleInvalid("")},
			{"NegativeTimeout", func(spec *cron.ScheduleSpec) { spec.Timeout = -time.Second }, cron.ErrScheduleInvalid("")},
			{
				"SubMillisecondTimeout",
				func(spec *cron.ScheduleSpec) { spec.Timeout = 500 * time.Microsecond },
				cron.ErrScheduleInvalid(""),
			},
			{
				"FractionalMillisecondTimeout",
				func(spec *cron.ScheduleSpec) { spec.Timeout = 1500 * time.Microsecond },
				cron.ErrScheduleInvalid(""),
			},
			{
				"InvertedWindow",
				func(spec *cron.ScheduleSpec) {
					starts := s.now.Add(time.Hour)
					ends := s.now
					spec.StartsAt, spec.EndsAt = &starts, &ends
				},
				cron.ErrScheduleInvalid(""),
			},
			{
				"WindowCollapsesWithinOneMillisecond",
				func(spec *cron.ScheduleSpec) {
					starts := s.now.Add(100 * time.Microsecond)
					ends := s.now.Add(900 * time.Microsecond)
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
				s.Require().ErrorIs(err, tc.wantErr, "The spec fault must map to its outward error")
			})
		}
	})

	s.Run("DisabledSpecStaysUnarmed", func() {
		disabled := false
		spec := s.validSpec("dormant")
		spec.Enabled = &disabled

		schedule, err := s.manager.Create(s.ctx(), spec)
		s.Require().NoError(err, "Creating a disabled schedule should succeed")
		s.False(schedule.IsEnabled, "The schedule must be created paused")
		s.Nil(schedule.NextFireAtUnixMs, "A paused schedule carries no next fire")
	})

	s.Run("RejectsEnabledScheduleThatNeverFires", func() {
		spent := s.validSpec("spent-once")
		spent.Trigger = cron.Once(s.now.Add(-time.Hour))

		_, err := s.manager.Create(s.ctx(), spent)
		s.Require().ErrorIs(err, cron.ErrScheduleInvalid(""),
			"A past one-shot would be created dead and must be refused")

		expired := s.validSpec("expired-window")
		ends := s.now.Add(-time.Minute)
		expired.EndsAt = &ends

		_, err = s.manager.Create(s.ctx(), expired)
		s.Require().ErrorIs(err, cron.ErrScheduleInvalid(""),
			"An already-expired window would be created dead and must be refused")

		disabled := false
		dormant := s.validSpec("dormant-once")
		dormant.Trigger = cron.Once(s.now.Add(-time.Hour))
		dormant.Enabled = &disabled

		_, err = s.manager.Create(s.ctx(), dormant)
		s.Require().NoError(err,
			"A disabled schedule carries no cursor by design and stays creatable")
	})

	s.Run("NameWidthCountsCharacters", func() {
		spec := s.validSpec(strings.Repeat("名", maxScheduleNameLength))

		_, err := s.manager.Create(s.ctx(), spec)
		s.Require().NoError(err, "A multibyte name at the character limit should persist")
	})

	s.Run("RejectsInvalidRawParams", func() {
		spec := s.validSpec("raw")
		spec.Params = json.RawMessage(`{broken`)

		_, err := s.manager.Create(s.ctx(), spec)
		s.Require().ErrorIs(err, cron.ErrScheduleInvalid(""), "Malformed raw params must be rejected")
	})
}

func (s *ManagerTestSuite) TestUpdate() {
	s.Run("ReshapesAndRearms", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("reshape"))
		s.Require().NoError(err, "The fixture create should succeed")

		spec := s.validSpec("reshape")
		spec.Trigger = cron.Expr("0 2 * * *", "Asia/Shanghai")
		spec.JobName = "report.daily"

		updated, err := s.manager.Update(s.ctx(), "reshape", spec)
		s.Require().NoError(err, "Updating should succeed")
		s.Equal(cron.TriggerCron, updated.Kind, "The trigger kind must change")
		s.Equal("report.daily", updated.JobName, "The job must change")
		s.Require().NotNil(updated.NextFireAtUnixMs, "The schedule must re-arm from now")
	})

	s.Run("Rename", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("old-name"))
		s.Require().NoError(err, "The fixture create should succeed")

		spec := s.validSpec("new-name")

		_, err = s.manager.Update(s.ctx(), "old-name", spec)
		s.Require().NoError(err, "Renaming should succeed")

		_, err = s.manager.Get(s.ctx(), "old-name")
		s.Require().ErrorIs(err, cron.ErrScheduleNotFound, "The old name must be gone")

		renamed, err := s.manager.Get(s.ctx(), "new-name")
		s.Require().NoError(err, "The new name must resolve")
		s.Equal("new-name", renamed.Name, "The row must carry the new name")
	})

	s.Run("RenameOntoTakenName", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("a"))
		s.Require().NoError(err, "Fixture a should be created")
		_, err = s.manager.Create(s.ctx(), s.validSpec("b"))
		s.Require().NoError(err, "Fixture b should be created")

		_, err = s.manager.Update(s.ctx(), "a", s.validSpec("b"))
		s.Require().ErrorIs(err, cron.ErrScheduleExists, "Renaming onto a taken name must conflict")
	})

	s.Run("UnknownSchedule", func() {
		_, err := s.manager.Update(s.ctx(), "ghost", s.validSpec("ghost"))
		s.Require().ErrorIs(err, cron.ErrScheduleNotFound, "Updating a missing schedule must fail")
	})

	s.Run("KeepsTheFireHistory", func() {
		created, err := s.manager.Create(s.ctx(), s.validSpec("historic"))
		s.Require().NoError(err, "The fixture create should succeed")

		// The engine owns LastFireAtUnixMs; simulate a fire it already claimed.
		created.LastFireAtUnixMs = unixMillisPtr(s.now.Add(-time.Hour))
		_, err = s.db.NewUpdate().Model(created).
			Select("last_fire_at_unix_ms").
			WherePK().
			Exec(s.ctx())
		s.Require().NoError(err, "Stamping the fire history should succeed")

		spec := s.validSpec("historic")
		spec.Params = map[string]any{"tweaked": true}

		_, err = s.manager.Update(s.ctx(), "historic", spec)
		s.Require().NoError(err, "Reshaping the schedule should succeed")

		updated, err := s.manager.Get(s.ctx(), "historic")
		s.Require().NoError(err, "The updated schedule must load")
		s.Require().NotNil(updated.LastFireAtUnixMs, "Editing a schedule must not erase what already ran")
		s.Equal(s.now.Add(-time.Hour).UnixMilli(), *updated.LastFireAtUnixMs,
			"The recorded fire must survive verbatim")
	})

	s.Run("NonTimingEditKeepsAnOverdueCursor", func() {
		created, err := s.manager.Create(s.ctx(), s.validSpec("overdue-edit"))
		s.Require().NoError(err, "The fixture create should succeed")

		overdue := s.now.Add(-time.Hour).UnixMilli()
		created.NextFireAtUnixMs = &overdue
		_, err = s.db.NewUpdate().Model(created).
			Select("next_fire_at_unix_ms").
			WherePK().
			Exec(s.ctx())
		s.Require().NoError(err, "Stamping the overdue cursor should succeed")

		spec := s.validSpec("overdue-edit")
		spec.Params = map[string]any{"region": "east"}
		updated, err := s.manager.Update(s.ctx(), created.Name, spec)
		s.Require().NoError(err, "A params-only edit should succeed")
		s.Require().NotNil(updated.NextFireAtUnixMs, "The pending occurrence must remain armed")
		s.Equal(overdue, *updated.NextFireAtUnixMs, "A non-timing edit must not move an overdue cursor")
	})

	s.Run("DisableThroughUpdatePreservesThePauseGap", func() {
		created, err := s.manager.Create(s.ctx(), s.validSpec("update-pause"))
		s.Require().NoError(err, "The fixture create should succeed")
		s.Require().NotNil(created.NextFireAtUnixMs, "The fixture should have a pending cursor")
		cursor := *created.NextFireAtUnixMs

		disabled := false
		spec := s.validSpec("update-pause")
		spec.Enabled = &disabled
		s.now = s.now.Add(10 * time.Minute)

		updated, err := s.manager.Update(s.ctx(), created.Name, spec)
		s.Require().NoError(err, "Disabling through update should succeed")
		s.False(updated.IsEnabled, "The update should disable the schedule")
		s.Require().NotNil(updated.NextFireAtUnixMs, "Disabling must preserve the pending cursor")
		s.Equal(cursor, *updated.NextFireAtUnixMs, "Disabling must preserve the same pause gap as Pause")

		s.Require().NoError(s.manager.Resume(s.ctx(), created.Name), "Resuming the updated schedule should succeed")
		resumed, err := s.manager.Get(s.ctx(), created.Name)
		s.Require().NoError(err, "The resumed schedule should load")
		s.Require().NotNil(resumed.NextFireAtUnixMs, "Resume should retain the overdue cursor for misfire handling")
		s.Equal(cursor, *resumed.NextFireAtUnixMs, "Resume must not erase the preserved gap")
	})
}

func (s *ManagerTestSuite) TestMaterializePreservesForeignZoneInstants() {
	newYork, err := time.LoadLocation("America/New_York")
	s.Require().NoError(err, "The New York zone must load")

	// Go callers can use any location; persistence keeps the named instant.
	at := s.now.Add(24 * time.Hour).In(newYork)
	starts := s.now.Add(time.Hour).In(newYork)
	ends := s.now.Add(48 * time.Hour).In(newYork)

	schedule, err := s.manager.Create(s.ctx(), cron.ScheduleSpec{
		Name:     "foreign",
		JobName:  "orders.sync",
		Trigger:  cron.Once(at),
		StartsAt: &starts,
		EndsAt:   &ends,
	})
	s.Require().NoError(err, "Creating a schedule with foreign-zone times should succeed")

	s.Require().NotNil(schedule.FireAtUnixMs, "The one-shot time must be stored")
	s.Equal(at.UnixMilli(), *schedule.FireAtUnixMs, "The stored fire time must preserve the caller's instant")
	s.Require().NotNil(schedule.StartsAtUnixMs, "The window start must be stored")
	s.Equal(starts.UnixMilli(), *schedule.StartsAtUnixMs, "The window start must preserve the caller's instant")
	s.Require().NotNil(schedule.EndsAtUnixMs, "The window end must be stored")
	s.Equal(ends.UnixMilli(), *schedule.EndsAtUnixMs, "The window end must preserve the caller's instant")

	s.Require().NotNil(schedule.NextFireAtUnixMs, "The one-shot must be armed")
	s.Equal(at.UnixMilli(), *schedule.NextFireAtUnixMs, "The armed fire must preserve the caller's instant")
}

func (s *ManagerTestSuite) TestMaterializeCanonicalizesTheDefaultTimezone() {
	schedule, err := s.manager.Create(s.ctx(), cron.ScheduleSpec{
		Name:    "utc-default",
		JobName: "orders.sync",
		Trigger: cron.TriggerSpec{Kind: cron.TriggerCron, Expr: "0 2 * * *"},
	})
	s.Require().NoError(err, "Creating a cron trigger without a timezone should succeed")
	s.Equal(cron.DefaultTimezone, schedule.Timezone, "The persisted schedule should name its UTC default explicitly")
}

func (s *ManagerTestSuite) TestPauseResume() {
	s.Run("PausePreservesTheCursor", func() {
		// The interval trigger arms one period after creation.
		armed := s.now.Add(time.Minute)

		_, err := s.manager.Create(s.ctx(), s.validSpec("pausable"))
		s.Require().NoError(err, "The fixture create should succeed")

		s.Require().NoError(s.manager.Pause(s.ctx(), "pausable"), "Pausing should succeed")

		paused, err := s.manager.Get(s.ctx(), "pausable")
		s.Require().NoError(err, "The paused schedule must load")
		s.False(paused.IsEnabled, "Pause must disable")
		s.Require().NotNil(paused.NextFireAtUnixMs, "Pause must keep the cursor so the gap stays accountable")
		s.Equal(armed.UnixMilli(), *paused.NextFireAtUnixMs, "Pause must not move the cursor")
	})

	s.Run("ResumeHandsThePausedGapToTheMisfirePolicy", func() {
		armed := s.now.Add(time.Minute)

		_, err := s.manager.Create(s.ctx(), s.validSpec("catchup"))
		s.Require().NoError(err, "The fixture create should succeed")
		s.Require().NoError(s.manager.Pause(s.ctx(), "catchup"), "Pausing should succeed")

		// Resume far past the paused occurrence: the cursor stays put so the
		// claim applies the misfire policy to the gap, exactly as it does for
		// downtime — resume itself decides nothing.
		s.now = s.now.Add(2 * time.Hour)
		defer func() { s.now = s.now.Add(-2 * time.Hour) }()

		s.Require().NoError(s.manager.Resume(s.ctx(), "catchup"), "Resuming should succeed")

		resumed, err := s.manager.Get(s.ctx(), "catchup")
		s.Require().NoError(err, "The resumed schedule must load")
		s.True(resumed.IsEnabled, "Resume must re-enable")
		s.Require().NotNil(resumed.NextFireAtUnixMs, "The schedule must stay armed")
		s.Equal(armed.UnixMilli(), *resumed.NextFireAtUnixMs,
			"Resume must leave the paused cursor untouched for the misfire decision")
	})

	s.Run("ResumeRearmsAScheduleThatHasNoCursor", func() {
		spec := s.validSpec("dormant")
		spec.Enabled = new(bool)

		_, err := s.manager.Create(s.ctx(), spec)
		s.Require().NoError(err, "The fixture create should succeed")

		created, err := s.manager.Get(s.ctx(), "dormant")
		s.Require().NoError(err, "The disabled schedule must load")
		s.Require().Nil(created.NextFireAtUnixMs, "A schedule created disabled carries no cursor")

		s.now = s.now.Add(2 * time.Hour)
		defer func() { s.now = s.now.Add(-2 * time.Hour) }()

		s.Require().NoError(s.manager.Resume(s.ctx(), "dormant"), "Resuming should succeed")

		resumed, err := s.manager.Get(s.ctx(), "dormant")
		s.Require().NoError(err, "The resumed schedule must load")
		s.Require().NotNil(resumed.NextFireAtUnixMs, "A cursorless schedule must be armed from now")
		s.Greater(*resumed.NextFireAtUnixMs, s.now.UnixMilli(), "The fresh cursor lands in the future")
	})

	s.Run("ResumeEnabledIsIdempotent", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("already"))
		s.Require().NoError(err, "The fixture create should succeed")

		before, err := s.manager.Get(s.ctx(), "already")
		s.Require().NoError(err, "The schedule must load")

		s.Require().NoError(s.manager.Resume(s.ctx(), "already"), "Resuming an enabled schedule is a no-op")

		after, err := s.manager.Get(s.ctx(), "already")
		s.Require().NoError(err, "The schedule must reload")
		s.Equal(before.NextFireAtUnixMs, after.NextFireAtUnixMs, "A no-op resume must not move the fire")
	})
}

func (s *ManagerTestSuite) TestTriggerNow() {
	s.Run("QueuesWithoutMovingTheRegularCursor", func() {
		created, err := s.manager.Create(s.ctx(), s.validSpec("manual"))
		s.Require().NoError(err, "The fixture create should succeed")
		s.Require().NotNil(created.NextFireAtUnixMs, "The regular epoch cursor should be armed")
		regular := *created.NextFireAtUnixMs

		s.Require().NoError(s.manager.TriggerNow(s.ctx(), "manual"), "Triggering should succeed")

		triggered, err := s.manager.Get(s.ctx(), "manual")
		s.Require().NoError(err, "The schedule must load")
		s.Require().NotNil(triggered.NextFireAtUnixMs, "The regular cursor should remain persisted")
		s.Equal(regular, *triggered.NextFireAtUnixMs, "Manual fire should not move the regular cursor")

		requests := loadFireRequests(s.T(), s.db, created.ID)
		s.Require().Len(requests, 1, "TriggerNow should create one durable request")
		s.Equal(fireRequestManual, requests[0].Kind, "The request kind should be manual")
		s.Equal(s.now.UnixMilli(), requests[0].ScheduledAtUnixMs, "The request should carry the trigger instant")
	})

	s.Run("PausedScheduleRefuses", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("held"))
		s.Require().NoError(err, "The fixture create should succeed")
		s.Require().NoError(s.manager.Pause(s.ctx(), "held"), "Pausing should succeed")

		err = s.manager.TriggerNow(s.ctx(), "held")
		s.Require().ErrorIs(err, cron.ErrScheduleDisabled, "A paused schedule must refuse manual fires")
	})

	s.Run("AlreadyDueKeepsItsIndependentCursor", func() {
		fixture := scheduleFixture("due", "orders.sync", s.now.Add(-time.Hour))
		fixture.MisfirePolicy = cron.MisfireSkip
		schedule := insertSchedule(s.T(), s.db, fixture)

		s.Require().NoError(s.manager.TriggerNow(s.ctx(), "due"), "Triggering should succeed")

		after := reloadSchedule(s.T(), s.db, schedule.ID)
		s.Require().NotNil(after.NextFireAtUnixMs, "The overdue regular cursor should remain armed")
		s.Equal(s.now.Add(-time.Hour).UnixMilli(), *after.NextFireAtUnixMs,
			"The overdue regular occurrence should remain intact")
		requests := loadFireRequests(s.T(), s.db, schedule.ID)
		s.Require().Len(requests, 1, "The manual request should be represented independently")
		s.Equal(s.now.UnixMilli(), requests[0].ScheduledAtUnixMs, "The manual request should still be immediate")
	})
}

func (s *ManagerTestSuite) TestDeleteAndQueries() {
	s.Run("DeleteKeepsRuns", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("doomed"))
		s.Require().NoError(err, "The fixture create should succeed")

		run := &cron.Run{
			ScheduleID:      schedule.ID,
			ScheduleName:    schedule.Name,
			JobName:         schedule.JobName,
			Status:          cron.RunSucceeded,
			ClaimedAtUnixMs: s.now.UnixMilli(),
		}
		run.ScheduledAtUnixMs = s.now.UnixMilli()
		_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
		s.Require().NoError(err, "The journal fixture insert should succeed")
		s.Require().NoError(s.manager.TriggerNow(s.ctx(), "doomed"), "Queuing a pending fire should succeed")
		s.Require().Len(loadFireRequests(s.T(), s.db, schedule.ID), 1,
			"The delete fixture should have one pending request")

		s.Require().NoError(s.manager.Delete(s.ctx(), "doomed"), "Deleting should succeed")
		s.Require().ErrorIs(s.manager.Delete(s.ctx(), "doomed"), cron.ErrScheduleNotFound,
			"A second delete must report absence")

		runs, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{ScheduleName: "doomed"})
		s.Require().NoError(err, "Listing runs should succeed")
		s.Len(runs, 1, "Journal rows must survive schedule deletion")
		s.Empty(loadFireRequests(s.T(), s.db, schedule.ID), "Pending requests should be deleted with the schedule")
	})

	s.Run("ListFilters", func() {
		_, err := s.manager.Create(s.ctx(), s.validSpec("list-a"))
		s.Require().NoError(err, "Fixture list-a should be created")

		reportSpec := s.validSpec("list-b")
		reportSpec.JobName = "report.daily"
		_, err = s.manager.Create(s.ctx(), reportSpec)
		s.Require().NoError(err, "Fixture list-b should be created")
		s.Require().NoError(s.manager.Pause(s.ctx(), "list-b"), "Pausing list-b should succeed")

		byJob, err := s.manager.List(s.ctx(), cron.ScheduleFilter{JobName: "report.daily"})
		s.Require().NoError(err, "Listing by job should succeed")
		s.Require().Len(byJob, 1, "Only the matching job's schedule must return")
		s.Equal("list-b", byJob[0].Name, "The filter must match by job name")

		enabled := true
		byEnabled, err := s.manager.List(s.ctx(), cron.ScheduleFilter{Enabled: &enabled})
		s.Require().NoError(err, "Listing by enablement should succeed")
		s.Require().Len(byEnabled, 1, "Only the enabled schedule must return")
		s.Equal("list-a", byEnabled[0].Name, "The filter must match by enablement")
	})

	s.Run("ListRunsFiltersAndBounds", func() {
		schedule, err := s.manager.Create(s.ctx(), s.validSpec("journal"))
		s.Require().NoError(err, "The fixture create should succeed")

		statuses := []cron.RunStatus{cron.RunSucceeded, cron.RunFailed, cron.RunMissed}
		for i, status := range statuses {
			scheduledAt := s.now.Add(time.Duration(i) * time.Minute)
			run := &cron.Run{
				ScheduleID:      schedule.ID,
				ScheduleName:    schedule.Name,
				JobName:         schedule.JobName,
				Status:          status,
				ClaimedAtUnixMs: scheduledAt.UnixMilli(),
			}
			run.ScheduledAtUnixMs = scheduledAt.UnixMilli()
			_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
			s.Require().NoError(err, "The journal fixture insert should succeed")
		}

		failed, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Statuses: []cron.RunStatus{cron.RunFailed}})
		s.Require().NoError(err, "Filtering by status should succeed")
		s.Require().Len(failed, 1, "Only the failed run must return")
		s.Equal(cron.RunFailed, failed[0].Status, "The status filter must hold")

		since := s.now.Add(30 * time.Second)
		until := s.now.Add(90 * time.Second)
		windowed, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Since: &since, Until: &until})
		s.Require().NoError(err, "Filtering by window should succeed")
		s.Require().Len(windowed, 1, "Only the in-window run must return")
		s.Equal(cron.RunFailed, windowed[0].Status, "The window must select the middle run")

		bounded, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Limit: 2})
		s.Require().NoError(err, "Limiting should succeed")
		s.Len(bounded, 2, "The limit must bound the page")
	})
}

func (s *ManagerTestSuite) TestListRunsUsesForeignZoneInstants() {
	newYork, err := time.LoadLocation("America/New_York")
	s.Require().NoError(err, "The New York zone must load")

	first := time.Date(2012, time.November, 4, 1, 30, 0, 0, newYork)
	second := first.Add(time.Hour)
	s.Require().Equal(first.Format(time.DateTime), second.In(newYork).Format(time.DateTime),
		"The fixtures must share one fallback wall-clock label")

	schedule, err := s.manager.Create(s.ctx(), s.validSpec("foreign-window"))
	s.Require().NoError(err, "The fixture schedule should be created")

	for _, scheduledAt := range []time.Time{first, second} {
		run := &cron.Run{
			ScheduleID:      schedule.ID,
			ScheduleName:    schedule.Name,
			JobName:         schedule.JobName,
			Status:          cron.RunSucceeded,
			ClaimedAtUnixMs: scheduledAt.UnixMilli(),
		}
		run.ScheduledAtUnixMs = scheduledAt.UnixMilli()
		_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
		s.Require().NoError(err, "The fallback run fixture should be inserted")
	}

	until := second.Add(time.Minute)
	runs, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Since: &second, Until: &until})
	s.Require().NoError(err, "Foreign-zone bounds should query the absolute timeline")
	s.Require().Len(runs, 1, "Only the second copy of the fallback wall time should match")
	s.Equal(second.UnixMilli(), runs[0].ScheduledAtUnixMs,
		"The range should return the second fallback instant")
}

func (s *ManagerTestSuite) TestListRunsRoundsSubMillisecondBoundsUp() {
	schedule, err := s.manager.Create(s.ctx(), s.validSpec("sub-millisecond-window"))
	s.Require().NoError(err, "The fixture schedule should be created")

	run := &cron.Run{
		ScheduleID:        schedule.ID,
		ScheduleName:      schedule.Name,
		JobName:           schedule.JobName,
		ScheduledAtUnixMs: s.now.UnixMilli(),
		ClaimedAtUnixMs:   s.now.UnixMilli(),
		Status:            cron.RunSucceeded,
	}
	_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
	s.Require().NoError(err, "The journal fixture should be inserted")

	betweenMilliseconds := s.now.Add(500 * time.Microsecond)
	afterLowerBound, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Since: &betweenMilliseconds})
	s.Require().NoError(err, "The sub-millisecond lower bound should query successfully")
	s.Empty(afterLowerBound, "An inclusive lower bound after the stored millisecond must exclude the run")

	beforeUpperBound, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{Until: &betweenMilliseconds})
	s.Require().NoError(err, "The sub-millisecond upper bound should query successfully")
	s.Require().Len(beforeUpperBound, 1, "An exclusive upper bound after the stored millisecond must include the run")
	s.Equal(run.ID, beforeUpperBound[0].ID, "The exact journal row should remain inside the upper bound")
}

func (s *ManagerTestSuite) TestListRunsOrdersByClaimInstantAcrossFallback() {
	newYork, err := time.LoadLocation("America/New_York")
	s.Require().NoError(err, "The New York zone must load")

	earlier := time.Date(2012, time.November, 4, 5, 45, 0, 0, time.UTC)
	later := time.Date(2012, time.November, 4, 6, 15, 0, 0, time.UTC)

	s.Equal("01:45 EDT", earlier.In(newYork).Format("15:04 MST"),
		"The earlier claim should have the later wall label")
	s.Equal("01:15 EST", later.In(newYork).Format("15:04 MST"),
		"The later claim should have the earlier wall label")

	schedule, err := s.manager.Create(s.ctx(), s.validSpec("fallback-order"))
	s.Require().NoError(err, "The fixture schedule should be created")

	for _, claimedAt := range []time.Time{earlier, later} {
		run := &cron.Run{
			ScheduleID:      schedule.ID,
			ScheduleName:    schedule.Name,
			JobName:         schedule.JobName,
			Status:          cron.RunSucceeded,
			ClaimedAtUnixMs: claimedAt.UnixMilli(),
		}
		run.ScheduledAtUnixMs = claimedAt.UnixMilli()
		_, err = s.db.NewInsert().Model(run).Exec(s.ctx())
		s.Require().NoError(err, "The fallback run fixture should be inserted")
	}

	runs, err := s.manager.ListRuns(s.ctx(), cron.RunFilter{})
	s.Require().NoError(err, "Listing fallback runs should succeed")
	s.Require().Len(runs, 2, "Both fallback runs should be returned")
	s.Equal(
		[]int64{later.UnixMilli(), earlier.UnixMilli()},
		[]int64{runs[0].ClaimedAtUnixMs, runs[1].ClaimedAtUnixMs},
		"Newest-first must follow the absolute claim timeline",
	)
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
				"Every method of the disabled manager must report the store off")
		})
	}
}
