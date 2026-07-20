package store

import (
	"context"
	"fmt"
	"time"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// sweepBatchSize bounds one recovery pass; a backlog larger than this is
// drained across consecutive ticks.
const sweepBatchSize = 100

// takeoverResult is the concurrency state observed while holding every
// running row of a set of already-locked schedules. Stale rows are finalized
// and queued here, so only genuinely live rows remain active.
type takeoverResult struct {
	active         collections.Set[string]
	abandoned      []cron.Run
	queuedRecovery bool
}

// sweepAbandoned finds schedule gates with stale runs, then takes the same
// schedule -> run -> fire-request lock order as claiming. A separate orphan
// lane finalizes runs whose schedule has already been deleted.
func (e *Engine) sweepAbandoned(ctx context.Context) {
	now := e.now()
	staleBefore := now.Add(-e.config.EffectiveAbandonedAfter()).UnixMilli()

	var (
		orphans        []cron.Run
		queuedRecovery bool
	)

	err := e.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		schedules, err := lockSchedulesWithStaleRuns(ctx, tx, staleBefore)
		if err != nil {
			return err
		}

		takeover, err := takeOverRunning(
			ctx,
			tx,
			schedules,
			now,
			e.config.EffectiveAbandonedAfter(),
		)
		if err != nil {
			return err
		}

		orphans = append(orphans, takeover.abandoned...)
		queuedRecovery = takeover.queuedRecovery

		remaining := max(sweepBatchSize-len(orphans), 0)
		if remaining == 0 {
			return nil
		}

		scheduleless, err := lockSchedulelessStaleRuns(ctx, tx, staleBefore, remaining)
		if err != nil {
			return err
		}

		if err := markAbandoned(ctx, tx, scheduleless, now); err != nil {
			return err
		}

		orphans = append(orphans, scheduleless...)

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

	e.reportAbandoned(orphans)

	if queuedRecovery {
		e.Wake()
	}
}

func lockSchedulesWithStaleRuns(ctx context.Context, tx orm.DB, staleBefore int64) ([]cron.Schedule, error) {
	var schedules []cron.Schedule
	if err := tx.NewSelect().
		Model(&schedules).
		Where(func(cb orm.ConditionBuilder) {
			cb.InSubQuery("id", func(sq orm.SelectQuery) {
				sq.Model((*cron.Run)(nil)).
					Select("schedule_id").
					Where(func(cb orm.ConditionBuilder) { staleRunningCondition(cb, staleBefore) }).
					GroupBy("schedule_id")
			})
		}).
		OrderBy("id").
		Limit(sweepBatchSize).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("lock schedules with abandoned runs: %w", err)
	}

	return schedules, nil
}

func lockSchedulelessStaleRuns(
	ctx context.Context,
	tx orm.DB,
	staleBefore int64,
	limit int,
) ([]cron.Run, error) {
	var runs []cron.Run
	if err := tx.NewSelect().
		Model(&runs).
		Where(func(cb orm.ConditionBuilder) {
			staleRunningCondition(cb, staleBefore)
			cb.NotInSubQuery("schedule_id", func(sq orm.SelectQuery) {
				sq.Model((*cron.Schedule)(nil)).Select("id")
			})
		}).
		OrderBy("id").
		Limit(limit).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("lock scheduleless abandoned runs: %w", err)
	}

	return runs, nil
}

func staleRunningCondition(cb orm.ConditionBuilder, staleBefore int64) {
	cb.Equals("status", cron.RunRunning).
		Group(func(cb orm.ConditionBuilder) {
			cb.IsNull("heartbeat_at_unix_ms").OrLessThan("heartbeat_at_unix_ms", staleBefore)
		})
}

// takeOverRunning locks every running row of already-locked schedules, waits
// for in-flight heartbeat/completion writes, and rechecks liveness on the
// committed row version before applying concurrency policy.
func takeOverRunning(
	ctx context.Context,
	tx orm.DB,
	schedules []cron.Schedule,
	now time.Time,
	abandonedAfter time.Duration,
) (takeoverResult, error) {
	result := takeoverResult{active: collections.NewHashSetFrom[string]()}
	if len(schedules) == 0 {
		return result, nil
	}

	scheduleIDs := make([]string, len(schedules))

	recoverable := collections.NewHashSetFrom[string]()
	for i := range schedules {
		scheduleIDs[i] = schedules[i].ID
		if schedules[i].Recover {
			recoverable.Add(schedules[i].ID)
		}
	}

	var running []cron.Run
	if err := tx.NewSelect().
		Model(&running).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("schedule_id", scheduleIDs).Equals("status", cron.RunRunning)
		}).
		OrderBy("id").
		ForUpdate().
		Scan(ctx); err != nil {
		return takeoverResult{}, fmt.Errorf("lock running schedules: %w", err)
	}

	staleBefore := now.Add(-abandonedAfter).UnixMilli()
	for i := range running {
		run := &running[i]
		if run.HeartbeatAtUnixMs != nil && *run.HeartbeatAtUnixMs >= staleBefore {
			result.active.Add(run.ScheduleID)

			continue
		}

		result.abandoned = append(result.abandoned, *run)
	}

	if err := markAbandoned(ctx, tx, result.abandoned, now); err != nil {
		return takeoverResult{}, err
	}

	queued, err := enqueueRecoverable(ctx, tx, result.abandoned, recoverable)
	if err != nil {
		return takeoverResult{}, err
	}

	result.queuedRecovery = queued

	return result, nil
}

// markAbandoned finalizes locked orphan rows.
func markAbandoned(ctx context.Context, tx orm.DB, orphans []cron.Run, now time.Time) error {
	if len(orphans) == 0 {
		return nil
	}

	updated, err := tx.NewUpdate().
		Model((*cron.Run)(nil)).
		Set("status", cron.RunAbandoned).
		Set("finished_at_unix_ms", now.UnixMilli()).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKIn(runIDs(orphans)).Equals("status", cron.RunRunning)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark abandoned: %w", err)
	}

	affected, err := updated.RowsAffected()
	if err != nil {
		return fmt.Errorf("count abandoned runs: %w", err)
	}

	if affected != int64(len(orphans)) {
		return fmt.Errorf("%w: updated %d of %d", ErrAbandonTakeoverIncomplete, affected, len(orphans))
	}

	for i := range orphans {
		orphans[i].Status = cron.RunAbandoned
		orphans[i].FinishedAtUnixMs = unixMillisPtr(now)
	}

	return nil
}

// enqueueRecoverable persists one explicit fire request per orphan whose
// locked schedule opted into recovery. The orphan's run ID is a durable
// idempotency fence, and its original logical time stays attached to the retry.
func enqueueRecoverable(
	ctx context.Context,
	tx orm.DB,
	orphans []cron.Run,
	recoverable collections.Set[string],
) (bool, error) {
	requests := make([]*fireRequest, 0, len(orphans))

	for i := range orphans {
		orphan := &orphans[i]
		if !recoverable.Contains(orphan.ScheduleID) {
			continue
		}

		requests = append(requests, &fireRequest{
			ScheduleID:        orphan.ScheduleID,
			Kind:              fireRequestRecovery,
			ScheduledAtUnixMs: orphan.ScheduledAtUnixMs,
			SourceRunID:       orphan.ID,
		})
	}

	if len(requests) == 0 {
		return false, nil
	}

	if _, err := tx.NewInsert().Model(&requests).Exec(ctx); err != nil {
		return false, fmt.Errorf("queue recovery fires: %w", err)
	}

	return true, nil
}

func (e *Engine) reportAbandoned(orphans []cron.Run) {
	for i := range orphans {
		orphan := &orphans[i]
		logger.Warnf("Run %s of schedule %q abandoned by node %s", orphan.ID, orphan.ScheduleName, orphan.NodeID)
		e.publisher.RunAbandoned(orphan)
	}
}

// runIDs projects the runs' primary keys.
func runIDs(runs []cron.Run) []string {
	ids := make([]string, len(runs))
	for i := range runs {
		ids[i] = runs[i].ID
	}

	return ids
}
