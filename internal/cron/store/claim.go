package store

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

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

// claimBatch reports both executable fires and durable progress. Journaled
// missed/skipped outcomes and consumed requests are progress even though they
// do not occupy an executor slot.
type claimBatch struct {
	fires      []claimedFire
	abandoned  []cron.Run
	progressed bool
}

// fireWork is one regular cursor occurrence or explicit durable request,
// ordered by its logical fire time before materialization.
type fireWork struct {
	schedule    *cron.Schedule
	request     *fireRequest
	scheduledAt time.Time
}

// claimer claims regular schedule occurrences and explicit fire requests.
// The transaction locks schedule rows first, then request rows, advances any
// regular cursors, consumes only requests whose outcome is journaled, and
// inserts every journal row atomically. Recovery requests blocked by
// ConcurrencyForbid stay pending instead of being skipped and lost.
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
	batch, err := c.claimDueBatch(ctx, limit)

	return batch.fires, err
}

func (c *claimer) claimDueBatch(ctx context.Context, limit int) (claimBatch, error) {
	if limit <= 0 || c.registry.IsEmpty() {
		return claimBatch{}, nil
	}

	var batch claimBatch

	err := c.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		now := c.now()

		requestSchedules, err := c.lockRequestedSchedules(ctx, tx, limit)
		if err != nil {
			return err
		}

		due, err := c.lockDueSchedules(ctx, tx, now, limit)
		if err != nil {
			return err
		}

		candidates, scheduleIDs := mergeCandidateSchedules(requestSchedules, due)
		if len(candidates) == 0 {
			return nil
		}

		takeover, err := takeOverRunning(
			ctx,
			tx,
			candidates,
			now,
			c.config.EffectiveAbandonedAfter(),
		)
		if err != nil {
			return err
		}

		batch.abandoned = takeover.abandoned

		requests, err := c.lockFireRequests(ctx, tx, scheduleIDs, limit)
		if err != nil {
			return err
		}

		work := mergeFireWork(candidates, requests, due)
		if len(work) == 0 {
			batch.progressed = len(batch.abandoned) > 0

			return nil
		}

		running := takeover.active

		var (
			journal            []*cron.Run
			consumedRequestIDs []string
		)

		for i := range work {
			if len(batch.fires) >= limit {
				break
			}

			item := &work[i]
			schedule := item.schedule

			handler, ok := c.registry.Lookup(schedule.JobName)
			if !ok {
				continue
			}

			if item.request != nil {
				row, consume := c.journalRequest(schedule, item.request, running.Contains(schedule.ID), now)
				if !consume {
					continue
				}

				journal = append(journal, row)
				consumedRequestIDs = append(consumedRequestIDs, item.request.ID)

				if row.Status == cron.RunRunning {
					running.Add(schedule.ID)
					batch.fires = append(batch.fires, claimedFire{
						run:      row,
						schedule: *schedule,
						handler:  handler,
						timeout:  c.runTimeout(schedule),
					})
				}

				if err := c.recordRequestedFire(ctx, tx, schedule, item.scheduledAt, now); err != nil {
					return err
				}

				continue
			}

			decision := decide(schedule, now, c.config.EffectiveMisfireThreshold())
			rows := c.journalRows(schedule, decision, running.Contains(schedule.ID), now)
			journal = append(journal, rows...)

			for _, row := range rows {
				if row.Status == cron.RunRunning {
					running.Add(schedule.ID)
					batch.fires = append(batch.fires, claimedFire{
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

		if len(journal) > 0 {
			if _, err := tx.NewInsert().Model(&journal).Exec(ctx); err != nil {
				return fmt.Errorf("insert journal rows: %w", err)
			}
		}

		if len(consumedRequestIDs) > 0 {
			if _, err := tx.NewDelete().
				Model((*fireRequest)(nil)).
				Where(func(cb orm.ConditionBuilder) { cb.PKIn(consumedRequestIDs) }).
				Exec(ctx); err != nil {
				return fmt.Errorf("consume fire requests: %w", err)
			}
		}

		batch.progressed = len(journal) > 0 || len(consumedRequestIDs) > 0 || len(batch.abandoned) > 0

		return nil
	})
	if err != nil {
		// The transaction rolled back whole: nothing fired, nothing
		// advanced; the next tick retries.
		if isLockContention(err) {
			logger.Warnf("Claim lost a write race, retrying next tick: %v", err)

			return claimBatch{}, nil
		}

		return claimBatch{}, err
	}

	return batch, nil
}

// lockRequestedSchedules finds the schedules owning the oldest explicit requests,
// locking and limiting that ordered scan in one statement so SKIP LOCKED can
// refill the batch with another schedule instead of being trapped inside a
// preselected ID window.
func (c *claimer) lockRequestedSchedules(
	ctx context.Context,
	tx orm.DB,
	limit int,
) ([]cron.Schedule, error) {
	var schedules []cron.Schedule
	if err := tx.NewSelect().
		Model(&schedules).
		Where(func(cb orm.ConditionBuilder) {
			cb.IsTrue("is_enabled").
				In("job_name", c.registry.Names()).
				InSubQuery("id", func(sq orm.SelectQuery) {
					sq.Model((*fireRequest)(nil)).
						Select("schedule_id").
						Where(runnableFireRequests).
						GroupBy("schedule_id")
				})
		}).
		OrderByExpr(func(eb orm.ExprBuilder) any {
			return eb.SubQuery(func(sq orm.SelectQuery) {
				sq.Model((*fireRequest)(nil)).
					SelectExpr(func(eb orm.ExprBuilder) any {
						return eb.MinColumn("scheduled_at_unix_ms")
					}).
					Where(func(cb orm.ConditionBuilder) {
						cb.EqualsColumn("cfr.schedule_id", "cs.id")
						runnableFireRequests(cb)
					})
			})
		}).
		OrderBy("id").
		Limit(limit).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("lock requested schedules: %w", err)
	}

	return schedules, nil
}

// lockFireRequests locks runnable request rows only after their schedule and
// running rows are locked, preserving the module-wide lock order.
func (*claimer) lockFireRequests(
	ctx context.Context,
	tx orm.DB,
	scheduleIDs []string,
	limit int,
) ([]fireRequest, error) {
	if len(scheduleIDs) == 0 {
		return nil, nil
	}

	var requests []fireRequest
	if err := tx.NewSelect().
		Model(&requests).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("schedule_id", scheduleIDs)
			runnableFireRequests(cb)
		}).
		OrderBy("scheduled_at_unix_ms", "id").
		Limit(limit).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("lock fire requests: %w", err)
	}

	return requests, nil
}

func runnableFireRequests(cb orm.ConditionBuilder) {
	cb.Group(func(cb orm.ConditionBuilder) {
		cb.NotEquals("kind", fireRequestRecovery).
			OrNotInSubQuery("schedule_id", func(sq orm.SelectQuery) {
				sq.Model((*cron.Schedule)(nil)).
					Select("id").
					Where(func(cb orm.ConditionBuilder) {
						cb.NotEquals("concurrency_policy", cron.ConcurrencyAllow).
							InSubQuery("id", func(sq orm.SelectQuery) {
								sq.Model((*cron.Run)(nil)).
									Select("schedule_id").
									Where(func(cb orm.ConditionBuilder) {
										cb.Equals("status", cron.RunRunning)
									})
							})
					})
			})
	})
}

func (c *claimer) lockDueSchedules(
	ctx context.Context,
	tx orm.DB,
	now time.Time,
	limit int,
) ([]cron.Schedule, error) {
	var due []cron.Schedule

	if err := tx.NewSelect().
		Model(&due).
		Where(func(cb orm.ConditionBuilder) {
			cb.IsTrue("is_enabled").
				IsNotNull("next_fire_at_unix_ms").
				LessThanOrEqual("next_fire_at_unix_ms", now.UnixMilli()).
				In("job_name", c.registry.Names())
		}).
		OrderBy("next_fire_at_unix_ms", "id").
		Limit(limit).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("select due schedules: %w", err)
	}

	return due, nil
}

func mergeCandidateSchedules(groups ...[]cron.Schedule) ([]cron.Schedule, []string) {
	byID := make(map[string]cron.Schedule)
	for _, schedules := range groups {
		for i := range schedules {
			if _, exists := byID[schedules[i].ID]; !exists {
				byID[schedules[i].ID] = schedules[i]
			}
		}
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	schedules := make([]cron.Schedule, 0, len(ids))
	for _, id := range ids {
		schedules = append(schedules, byID[id])
	}

	return schedules, ids
}

// mergeFireWork deduplicates schedule snapshots and orders regular cursor
// occurrences together with explicit requests by logical fire time. A regular
// occurrence wins an exact tie so TriggerNow never displaces that occurrence.
func mergeFireWork(
	requestSchedules []cron.Schedule,
	requests []fireRequest,
	due []cron.Schedule,
) []fireWork {
	schedules := make(map[string]*cron.Schedule, len(requestSchedules)+len(due))
	for i := range requestSchedules {
		schedule := &requestSchedules[i]
		schedules[schedule.ID] = schedule
	}

	for i := range due {
		schedule := &due[i]
		if _, exists := schedules[schedule.ID]; !exists {
			schedules[schedule.ID] = schedule
		}
	}

	work := make([]fireWork, 0, len(requests)+len(due))
	for i := range requests {
		request := &requests[i]
		if schedule, ok := schedules[request.ScheduleID]; ok {
			work = append(work, fireWork{
				schedule:    schedule,
				request:     request,
				scheduledAt: request.scheduledAt(),
			})
		}
	}

	for i := range due {
		schedule := schedules[due[i].ID]
		work = append(work, fireWork{
			schedule:    schedule,
			scheduledAt: unixTime(*schedule.NextFireAtUnixMs),
		})
	}

	slices.SortFunc(work, func(left, right fireWork) int {
		if order := left.scheduledAt.Compare(right.scheduledAt); order != 0 {
			return order
		}

		if (left.request == nil) != (right.request == nil) {
			if left.request == nil {
				return -1
			}

			return 1
		}

		if left.request != nil {
			return cmp.Compare(left.request.ID, right.request.ID)
		}

		return cmp.Compare(left.schedule.ID, right.schedule.ID)
	})

	return work
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

// journalRequest materializes one explicit request. Manual requests follow
// the regular concurrency policy and may be skipped; recovery requests stay
// pending while ConcurrencyForbid sees an active run.
func (c *claimer) journalRequest(
	schedule *cron.Schedule,
	request *fireRequest,
	overlapping bool,
	now time.Time,
) (*cron.Run, bool) {
	forbidden := overlapping && schedule.ConcurrencyPolicy != cron.ConcurrencyAllow
	if forbidden && request.Kind == fireRequestRecovery {
		return nil, false
	}

	row := &cron.Run{
		ScheduleID:        schedule.ID,
		ScheduleName:      schedule.Name,
		JobName:           schedule.JobName,
		ScheduledAtUnixMs: request.ScheduledAtUnixMs,
		ClaimedAtUnixMs:   now.UnixMilli(),
	}

	if forbidden {
		row.Status = cron.RunSkipped
		row.FinishedAtUnixMs = unixMillisPtr(now)
	} else {
		row.Status = cron.RunRunning
		row.NodeID = c.nodeID
		row.StartedAtUnixMs = unixMillisPtr(now)
		row.HeartbeatAtUnixMs = unixMillisPtr(now)
	}

	return row, true
}

// recordRequestedFire updates the public fire history without moving the
// regular cursor. An old recovery occurrence must not move LastFireAtUnixMs
// backwards past a newer logical fire.
func (*claimer) recordRequestedFire(
	ctx context.Context,
	tx orm.DB,
	schedule *cron.Schedule,
	scheduledAt time.Time,
	now time.Time,
) error {
	scheduledAtUnixMs := scheduledAt.UnixMilli()
	if schedule.LastFireAtUnixMs != nil && scheduledAtUnixMs <= *schedule.LastFireAtUnixMs {
		return nil
	}

	schedule.LastFireAtUnixMs = &scheduledAtUnixMs
	schedule.UpdatedAt = timex.DateTime(now)

	if _, err := tx.NewUpdate().
		Model(schedule).
		Select("last_fire_at_unix_ms", "updated_at").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("record requested fire for schedule %q: %w", schedule.Name, err)
	}

	return nil
}

// journalRows materializes a decision into journal rows: at most one
// running (or skipped, when ConcurrencyForbid suppresses an overlapping
// fire) row plus at most one missed row covering the misfire gap.
func (c *claimer) journalRows(schedule *cron.Schedule, decision fireDecision, overlapping bool, now time.Time) []*cron.Run {
	var rows []*cron.Run

	if decision.fire {
		row := &cron.Run{
			ScheduleID:        schedule.ID,
			ScheduleName:      schedule.Name,
			JobName:           schedule.JobName,
			ScheduledAtUnixMs: decision.scheduledAt.UnixMilli(),
			ClaimedAtUnixMs:   now.UnixMilli(),
		}

		if overlapping && schedule.ConcurrencyPolicy != cron.ConcurrencyAllow {
			row.Status = cron.RunSkipped
			row.FinishedAtUnixMs = unixMillisPtr(now)
		} else {
			row.Status = cron.RunRunning
			row.NodeID = c.nodeID
			row.StartedAtUnixMs = unixMillisPtr(now)
			row.HeartbeatAtUnixMs = unixMillisPtr(now)
		}

		rows = append(rows, row)
	}

	if decision.missed > 0 {
		row := &cron.Run{
			ScheduleID:        schedule.ID,
			ScheduleName:      schedule.Name,
			JobName:           schedule.JobName,
			ScheduledAtUnixMs: decision.missedFrom.UnixMilli(),
			Status:            cron.RunMissed,
			MissedCount:       decision.missed,
			ClaimedAtUnixMs:   now.UnixMilli(),
		}
		row.FinishedAtUnixMs = unixMillisPtr(now)
		rows = append(rows, row)
	}

	return rows
}

// advance moves the schedule's scheduling state past the claimed occurrence.
func (*claimer) advance(ctx context.Context, tx orm.DB, schedule *cron.Schedule, decision fireDecision, now time.Time) error {
	schedule.NextFireAtUnixMs = nil

	if decision.next != nil {
		schedule.NextFireAtUnixMs = unixMillisPtr(*decision.next)
	}

	columns := []string{"next_fire_at_unix_ms", "updated_at"}
	schedule.UpdatedAt = timex.DateTime(now)

	if decision.fire {
		schedule.LastFireAtUnixMs = unixMillisPtr(decision.scheduledAt)

		columns = append(columns, "last_fire_at_unix_ms")
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
