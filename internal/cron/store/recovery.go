package store

import (
	"context"
	"fmt"
	"time"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// sweepBatchSize bounds one recovery pass; a backlog larger than this is
// drained across consecutive ticks.
const sweepBatchSize = 100

// sweepAbandoned takes over runs whose heartbeat went stale — their node died
// or lost the database. The whole takeover is one transaction: the stale rows
// are locked so a heartbeat cannot land between the staleness check and the
// write, they are finalized as abandoned, and the schedules that opted into
// recovery are re-armed alongside them. Doing it in one transaction is what
// makes the downstream effects honest — the abandoned notifications and the
// recovery re-fire describe rows this sweep actually took over, and a failure
// anywhere rolls the whole takeover back for the next tick to retry rather
// than leaving runs abandoned with their recovery lost.
//
// Own runs are safe by construction: this node renews their heartbeats on a
// cadence the staleness window must exceed at least twofold (config-validated).
func (e *Engine) sweepAbandoned(ctx context.Context) {
	now := e.now()

	var (
		orphans []cron.Run
		refired bool
	)

	err := e.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		stale := timex.DateTime(now.Add(-e.config.EffectiveAbandonedAfter()))

		var found []cron.Run

		// SKIP LOCKED partitions the orphans across concurrent sweeps, and
		// the row lock blocks the executing node's heartbeat update — so a
		// run that proves liveness mid-sweep is never taken over.
		if err := tx.NewSelect().
			Model(&found).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("status", cron.RunRunning).LessThan("heartbeat_at", stale)
			}).
			OrderBy("heartbeat_at").
			Limit(sweepBatchSize).
			ForUpdateSkipLocked().
			Scan(ctx); err != nil {
			return fmt.Errorf("select abandoned runs: %w", err)
		}

		if len(found) == 0 {
			return nil
		}

		if err := markAbandoned(ctx, tx, found, now); err != nil {
			return err
		}

		moved, err := refireRecoverable(ctx, tx, found, now)
		if err != nil {
			return err
		}

		orphans, refired = found, moved

		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}

		if isLockContention(err) {
			logger.Warnf("Recovery sweep lost a write race, retrying next tick: %v", err)

			return
		}

		logger.Errorf("Sweep abandoned runs: %v", err)

		return
	}

	for i := range orphans {
		orphan := &orphans[i]
		logger.Warnf("Run %s of schedule %q abandoned by node %s", orphan.ID, orphan.ScheduleName, orphan.NodeID)
		e.publisher.RunAbandoned(ctx, orphan)
	}

	if refired {
		e.Wake()
	}
}

// markAbandoned finalizes the locked orphan rows.
func markAbandoned(ctx context.Context, tx orm.DB, orphans []cron.Run, now time.Time) error {
	finished := timex.DateTime(now)

	if _, err := tx.NewUpdate().
		Model((*cron.Run)(nil)).
		Set("status", cron.RunAbandoned).
		Set("finished_at", finished).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKIn(runIDs(orphans)).Equals("status", cron.RunRunning)
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("mark abandoned: %w", err)
	}

	return nil
}

// refireRecoverable pulls NextFireAt to now for the orphans' schedules that
// opted into recovery, reporting whether any fire moved. A pending fire that
// is already due is normally left alone — the imminent claim covers the
// recovery — except under MisfireSkip, which would journal that overdue fire
// as missed and run nothing, swallowing the recovery with it.
func refireRecoverable(ctx context.Context, tx orm.DB, orphans []cron.Run, now time.Time) (bool, error) {
	ids := collections.NewHashSetFrom[string]()
	for i := range orphans {
		ids.Add(orphans[i].ScheduleID)
	}

	due := timex.DateTime(now)

	updated, err := tx.NewUpdate().
		Model((*cron.Schedule)(nil)).
		Set("next_fire_at", due).
		Set("updated_at", due).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKIn(ids.ToSlice()).
				IsTrue("recover").
				Group(func(cb orm.ConditionBuilder) {
					cb.IsNull("next_fire_at").
						OrGreaterThan("next_fire_at", due).
						OrEquals("misfire_policy", cron.MisfireSkip)
				})
		}).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("re-fire schedules: %w", err)
	}

	affected, _ := updated.RowsAffected()

	return affected > 0, nil
}

// runIDs projects the runs' primary keys.
func runIDs(runs []cron.Run) []string {
	ids := make([]string, len(runs))
	for i := range runs {
		ids[i] = runs[i].ID
	}

	return ids
}
