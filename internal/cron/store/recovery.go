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

// sweepAbandoned finds running rows whose heartbeat went stale — their node
// died or lost the database — marks them abandoned, and re-fires the ones
// whose schedule opted into recovery. Own runs are safe by construction:
// this node renews their heartbeats on a cadence the staleness window must
// exceed at least twofold (config-validated).
func (e *Engine) sweepAbandoned(ctx context.Context) {
	now := e.now()
	stale := timex.DateTime(now.Add(-e.config.EffectiveAbandonedAfter()))

	var orphans []cron.Run

	if err := e.db.NewSelect().
		Model(&orphans).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("status", cron.RunRunning).LessThan("heartbeat_at", stale)
		}).
		OrderBy("heartbeat_at").
		Limit(sweepBatchSize).
		Scan(ctx); err != nil {
		if ctx.Err() == nil {
			logger.Errorf("Select abandoned runs: %v", err)
		}

		return
	}

	if len(orphans) == 0 {
		return
	}

	if err := e.markAbandoned(ctx, orphans, now); err != nil {
		if ctx.Err() == nil {
			logger.Errorf("Mark abandoned runs: %v", err)
		}

		return
	}

	for i := range orphans {
		orphan := &orphans[i]
		logger.Warnf("Run %s of schedule %q abandoned by node %s", orphan.ID, orphan.ScheduleName, orphan.NodeID)
		e.publisher.RunAbandoned(ctx, orphan)
	}

	if refired, err := e.refireRecoverable(ctx, orphans, now); err != nil {
		if ctx.Err() == nil {
			logger.Errorf("Re-fire recoverable schedules: %v", err)
		}
	} else if refired {
		e.Wake()
	}
}

// markAbandoned finalizes the orphaned rows; the status guard tolerates a
// concurrent sweep or a photo-finish completion.
func (e *Engine) markAbandoned(ctx context.Context, orphans []cron.Run, now time.Time) error {
	finished := timex.DateTime(now)

	if _, err := e.db.NewUpdate().
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
// opted into recovery, reporting whether any fire moved.
func (e *Engine) refireRecoverable(ctx context.Context, orphans []cron.Run, now time.Time) (bool, error) {
	ids := collections.NewHashSetFrom[string]()
	for i := range orphans {
		ids.Add(orphans[i].ScheduleID)
	}

	due := timex.DateTime(now)

	updated, err := e.db.NewUpdate().
		Model((*cron.Schedule)(nil)).
		Set("next_fire_at", due).
		Set("updated_at", due).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKIn(ids.ToSlice()).
				IsTrue("recover").
				Group(func(cb orm.ConditionBuilder) {
					cb.IsNull("next_fire_at").OrGreaterThan("next_fire_at", due)
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
