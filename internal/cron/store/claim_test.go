package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/internal/cron/store/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func newTestClaimer(db orm.DB, registry *Registry, nodeID string, now func() time.Time) *claimer {
	return &claimer{
		db:       db,
		config:   fastStoreConfig(),
		registry: registry,
		nodeID:   nodeID,
		now:      now,
	}
}

func TestClaimDue(t *testing.T) {
	base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	registry := mustRegistry(t, noopHandler("orders.sync"))

	t.Run("ClaimsAndAdvances", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s1", "orders.sync", base))

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "claiming should succeed")
		require.Len(t, claimed, 1, "one due schedule must yield one fire")

		fire := claimed[0]
		assert.Equal(t, "node-a", fire.run.NodeID, "the run must carry the claiming node")
		assert.Equal(t, cron.RunRunning, fire.run.Status, "the claimed fire is running")
		assert.True(t, fire.run.ScheduledAt.Unwrap().Equal(base), "the run carries the logical fire time")
		assert.NotEmpty(t, fire.run.ID, "the journal row must have been inserted with an id")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "the schedule must advance")
		assert.True(t, after.NextFireAt.AsLocal().Equal(base.Add(time.Minute)),
			"the next fire advances one interval")
		require.NotNil(t, after.LastFireAt, "the executed fire is recorded")
		assert.True(t, after.LastFireAt.AsLocal().Equal(base), "LastFireAt carries the logical fire time")

		again, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(2*time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "re-claiming should succeed")
		assert.Empty(t, again, "an advanced schedule is no longer due")
	})

	t.Run("MisfireSkipJournalsOneMissedRow", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s2", "orders.sync", base)
		schedule.MisfirePolicy = cron.MisfireSkip
		insertSchedule(t, db, schedule)

		now := base.Add(5*time.Minute + 30*time.Second)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "claiming should succeed")
		assert.Empty(t, claimed, "skip policy must not fire")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 1, "the whole gap collapses into one journal row")
		assert.Equal(t, cron.RunMissed, runs[0].Status, "the row is a missed record")
		assert.Equal(t, 6, runs[0].MissedCount, "it covers every overdue occurrence")
		require.NotNil(t, runs[0].FinishedAt, "a missed row is terminal")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "the schedule must advance past the gap")
		assert.True(t, after.NextFireAt.AsLocal().After(now), "the next fire is strictly future")
		assert.Nil(t, after.LastFireAt, "nothing executed, so LastFireAt stays unset")
	})

	t.Run("MisfireFireNowJournalsCatchUpAndMissed", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s3", "orders.sync", base))

		now := base.Add(5*time.Minute + 30*time.Second)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(now)).ClaimDue(context.Background(), 10)
		require.NoError(t, err, "claiming should succeed")
		require.Len(t, claimed, 1, "fire_now must run one catch-up")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "the claim journals the catch-up and the missed gap")
		assert.Equal(t, cron.RunRunning, runs[0].Status, "the oldest due occurrence runs")
		assert.Equal(t, cron.RunMissed, runs[1].Status, "the rest of the gap is missed")
		assert.Equal(t, 5, runs[1].MissedCount, "five further occurrences were overdue")
	})

	t.Run("ConcurrencyForbidSkips", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s4", "orders.sync", base))

		clock := fixedNow(base.Add(time.Second))
		c := newTestClaimer(db, registry, "node-a", clock)

		first, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "first claim should succeed")
		require.Len(t, first, 1, "the first occurrence fires")

		// The first run is still running when the next occurrence comes due.
		c.now = fixedNow(base.Add(time.Minute + time.Second))

		second, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "second claim should succeed")
		assert.Empty(t, second, "forbid must suppress the overlapping fire")

		runs := loadRuns(t, db, schedule.ID)
		require.Len(t, runs, 2, "the suppression is journaled")
		assert.Equal(t, cron.RunSkipped, runs[1].Status, "the overlapping occurrence is skipped")
		assert.Empty(t, runs[1].NodeID, "a skipped row never executed anywhere")
	})

	t.Run("ConcurrencyAllowOverlaps", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s5", "orders.sync", base)
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, db, schedule)

		c := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second)))

		first, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "first claim should succeed")
		require.Len(t, first, 1, "the first occurrence fires")

		c.now = fixedNow(base.Add(time.Minute + time.Second))

		second, err := c.ClaimDue(context.Background(), 10)
		require.NoError(t, err, "second claim should succeed")
		assert.Len(t, second, 1, "allow lets the fires overlap")
	})

	t.Run("ForeignJobsAreLeftAlone", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := insertSchedule(t, db, scheduleFixture("s6", "job.not.here", base))

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "claiming should succeed")
		assert.Empty(t, claimed, "a node without the handler must not claim the schedule")

		after := reloadSchedule(t, db, schedule.ID)
		require.NotNil(t, after.NextFireAt, "the schedule must stay due for capable nodes")
		assert.True(t, after.NextFireAt.AsLocal().Equal(base), "the fire must not advance")
	})

	t.Run("OneShotDisarmsAfterFiring", func(t *testing.T) {
		db := newStoreDB(t)
		schedule := scheduleFixture("s7", "orders.sync", base)
		schedule.Kind = cron.TriggerOnce
		schedule.EveryMs = 0
		fireAt := timex.DateTime(base)
		schedule.FireAt = &fireAt
		insertSchedule(t, db, schedule)

		claimed, err := newTestClaimer(db, registry, "node-a", fixedNow(base.Add(time.Second))).
			ClaimDue(context.Background(), 10)
		require.NoError(t, err, "claiming should succeed")
		require.Len(t, claimed, 1, "the one-shot fires once")

		after := reloadSchedule(t, db, schedule.ID)
		assert.Nil(t, after.NextFireAt, "a spent one-shot has no next fire")
		assert.True(t, after.IsEnabled, "enablement stays operator-owned")
	})
}

// TestClaimContention proves the exactly-once claim across every supported
// dialect: two nodes race for the same occurrences round after round, and
// each occurrence must be claimed by exactly one of them.
func TestClaimContention(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		require.NoError(t, migration.Migrate(env.Ctx, env.DB, env.DS.Kind),
			"cron store migration should provision the tables")

		base := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
		registry := mustRegistry(t, noopHandler("orders.sync"))

		schedule := scheduleFixture("contended", "orders.sync", base)
		schedule.EveryMs = time.Second.Milliseconds()
		schedule.ConcurrencyPolicy = cron.ConcurrencyAllow
		insertSchedule(t, env.DB, schedule)

		const rounds = 20

		var (
			mu      sync.Mutex
			claimed []cron.Run
		)

		for round := range rounds {
			now := base.Add(time.Duration(round)*time.Second + 100*time.Millisecond)

			var wg sync.WaitGroup

			for _, node := range []string{"node-a", "node-b"} {
				wg.Go(func() {
					fires, err := newTestClaimer(env.DB, registry, node, fixedNow(now)).
						ClaimDue(env.Ctx, 10)
					assert.NoError(t, err, "contended claiming must not error")

					mu.Lock()
					defer mu.Unlock()

					for _, fire := range fires {
						claimed = append(claimed, *fire.run)
					}
				})
			}

			wg.Wait()
		}

		require.Len(t, claimed, rounds, "every occurrence must be claimed exactly once")

		seen := make(map[string]string, rounds)
		for _, run := range claimed {
			key := run.ScheduledAt.String()
			previous, duplicated := seen[key]
			require.False(t, duplicated,
				"occurrence %s was claimed by both %s and %s", key, previous, run.NodeID)
			seen[key] = run.NodeID
		}

		runs := loadRuns(t, env.DB, schedule.ID)
		assert.Len(t, runs, rounds, "the journal must hold exactly one row per occurrence")
	})
}
