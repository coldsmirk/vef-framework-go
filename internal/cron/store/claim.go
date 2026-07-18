package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// claimedFire is one fire this node owns after a successful claim: the
// inserted running journal row, its schedule snapshot, and the resolved
// handler.
type claimedFire struct {
	run      *cron.Run
	schedule cron.Schedule
	handler  cron.JobHandler
	timeout  time.Duration
}

// claimer claims due schedules for execution. Claiming IS the mutual
// exclusion: inside one transaction it locks due rows (FOR UPDATE SKIP
// LOCKED), advances NextFireAt past now, and inserts the journal rows — so
// one occurrence can be owned by at most one node, with no lease column and
// no distributed lock. A unique (schedule_id, scheduled_at) fence was
// deliberately rejected: timestamps carry second precision, so a manual
// fire or a recovery re-fire in the same second as an earlier row would
// livelock the schedule on a constraint that guards nothing the
// claim-advance transaction does not already guarantee.
type claimer struct {
	db       orm.DB
	config   *config.CronStoreConfig
	registry *Registry
	nodeID   string
	now      func() time.Time
}

// ClaimDue claims at most limit due fires this node can execute. Schedules
// whose job has no handler on this node are left untouched — heterogeneous
// deployments claim only what they can run.
func (c *claimer) ClaimDue(ctx context.Context, limit int) ([]claimedFire, error) {
	if limit <= 0 || c.registry.IsEmpty() {
		return nil, nil
	}

	var claimed []claimedFire

	err := c.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		now := c.now()

		var due []cron.Schedule
		if err := tx.NewSelect().
			Model(&due).
			Where(func(cb orm.ConditionBuilder) {
				cb.IsTrue("is_enabled").
					IsNotNull("next_fire_at").
					LessThanOrEqual("next_fire_at", timex.DateTime(now)).
					In("job_name", c.registry.Names())
			}).
			OrderBy("next_fire_at", "id").
			Limit(limit).
			ForUpdateSkipLocked().
			Scan(ctx); err != nil {
			return fmt.Errorf("select due schedules: %w", err)
		}

		if len(due) == 0 {
			return nil
		}

		running, err := c.runningSchedules(ctx, tx, scheduleIDs(due))
		if err != nil {
			return err
		}

		var journal []*cron.Run

		for i := range due {
			schedule := &due[i]

			handler, ok := c.registry.Lookup(schedule.JobName)
			if !ok {
				continue
			}

			decision := decide(schedule, now, c.config.EffectiveMisfireThreshold())
			rows := c.journalRows(schedule, decision, running.Contains(schedule.ID), now)
			journal = append(journal, rows...)

			for _, row := range rows {
				if row.Status == cron.RunRunning {
					claimed = append(claimed, claimedFire{
						run:      row,
						schedule: *schedule,
						handler:  handler,
						timeout:  c.runTimeout(schedule),
					})
				}
			}

			if err := c.advance(ctx, tx, schedule, decision, now); err != nil {
				return err
			}
		}

		if len(journal) == 0 {
			return nil
		}

		if _, err := tx.NewInsert().Model(&journal).Exec(ctx); err != nil {
			return fmt.Errorf("insert journal rows: %w", err)
		}

		return nil
	})
	if err != nil {
		// The transaction rolled back whole: nothing fired, nothing
		// advanced; the next tick retries.
		if isLockContention(err) {
			logger.Warnf("Claim lost a write race, retrying next tick: %v", err)

			return nil, nil
		}

		return nil, err
	}

	return claimed, nil
}

// isLockContention reports a benign claim-race loss: another writer held the
// store while the claim transaction tried to write. SQLite surfaces this as
// SQLITE_BUSY even inside the busy timeout — a snapshot that went stale must
// not wait for the writer. Row-locking dialects never produce it; FOR UPDATE
// SKIP LOCKED partitions the due rows up front.
func isLockContention(err error) bool {
	message := err.Error()

	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "SQLITE_BUSY")
}

// runningSchedules returns which of the given schedules currently have a
// running journal row — the ConcurrencyForbid overlap check.
func (*claimer) runningSchedules(ctx context.Context, tx orm.DB, ids []string) (collections.Set[string], error) {
	var runningIDs []string

	if err := tx.NewSelect().
		Model((*cron.Run)(nil)).
		Select("schedule_id").
		Distinct().
		Where(func(cb orm.ConditionBuilder) {
			cb.In("schedule_id", ids).Equals("status", cron.RunRunning)
		}).
		Scan(ctx, &runningIDs); err != nil {
		return nil, fmt.Errorf("select running schedules: %w", err)
	}

	return collections.NewHashSetFrom(runningIDs...), nil
}

// journalRows materializes a decision into journal rows: at most one
// running (or skipped, when ConcurrencyForbid suppresses an overlapping
// fire) row plus at most one missed row covering the misfire gap.
func (c *claimer) journalRows(schedule *cron.Schedule, decision fireDecision, overlapping bool, now time.Time) []*cron.Run {
	var rows []*cron.Run

	if decision.fire {
		row := &cron.Run{
			ScheduleID:   schedule.ID,
			ScheduleName: schedule.Name,
			JobName:      schedule.JobName,
			ScheduledAt:  timex.DateTime(decision.scheduledAt),
		}

		if overlapping && schedule.ConcurrencyPolicy != cron.ConcurrencyAllow {
			row.Status = cron.RunSkipped
			finished := timex.DateTime(now)
			row.FinishedAt = &finished
		} else {
			row.Status = cron.RunRunning
			row.NodeID = c.nodeID
			started := timex.DateTime(now)
			row.StartedAt = &started
			row.HeartbeatAt = &started
		}

		rows = append(rows, row)
	}

	if decision.missed > 0 {
		finished := timex.DateTime(now)
		rows = append(rows, &cron.Run{
			ScheduleID:   schedule.ID,
			ScheduleName: schedule.Name,
			JobName:      schedule.JobName,
			ScheduledAt:  timex.DateTime(decision.missedFrom),
			Status:       cron.RunMissed,
			FinishedAt:   &finished,
			MissedCount:  decision.missed,
		})
	}

	return rows
}

// advance moves the schedule's scheduling state past the claimed occurrence.
func (*claimer) advance(ctx context.Context, tx orm.DB, schedule *cron.Schedule, decision fireDecision, now time.Time) error {
	schedule.NextFireAt = nil
	if decision.next != nil {
		next := timex.DateTime(*decision.next)
		schedule.NextFireAt = &next
	}

	columns := []string{"next_fire_at", "updated_at"}
	schedule.UpdatedAt = timex.DateTime(now)

	if decision.fire {
		last := timex.DateTime(decision.scheduledAt)
		schedule.LastFireAt = &last

		columns = append(columns, "last_fire_at")
	}

	if _, err := tx.NewUpdate().
		Model(schedule).
		Select(columns...).
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("advance schedule %q: %w", schedule.Name, err)
	}

	return nil
}

// runTimeout resolves the per-run timeout: the schedule's own, else the
// configured default; zero leaves the run unbounded.
func (c *claimer) runTimeout(schedule *cron.Schedule) time.Duration {
	if timeout := schedule.Timeout(); timeout > 0 {
		return timeout
	}

	return c.config.RunTimeout
}

// scheduleIDs projects the schedules' primary keys.
func scheduleIDs(schedules []cron.Schedule) []string {
	ids := make([]string, len(schedules))
	for i := range schedules {
		ids[i] = schedules[i].ID
	}

	return ids
}
