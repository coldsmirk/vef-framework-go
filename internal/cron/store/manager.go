package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const (
	// Persisted schedule and job identifiers share the DDL's character width.
	maxScheduleNameLength = 128
	maxJobNameLength      = 128
	// defaultRunPageSize and maxRunPageSize bound ListRuns.
	defaultRunPageSize = 100
	maxRunPageSize     = 1000
)

// scheduleManager implements cron.ScheduleManager over the store tables.
// Every mutation wakes the engine so a nearer fire never waits out the
// current adaptive sleep.
type scheduleManager struct {
	db       orm.DB
	registry *Registry
	engine   *Engine
	now      func() time.Time
}

// NewScheduleManager builds the management surface. With the store disabled
// it degrades to a stub that fails every call with cron.ErrStoreDisabled —
// the dependency stays injectable, the capability reports itself off.
func NewScheduleManager(db orm.DB, enabled bool, registry *Registry, engine *Engine) cron.ScheduleManager {
	if !enabled {
		return disabledScheduleManager{}
	}

	return &scheduleManager{
		db:       db,
		registry: registry,
		engine:   engine,
		now:      func() time.Time { return timex.Now().Unwrap() },
	}
}

func (m *scheduleManager) Create(ctx context.Context, spec cron.ScheduleSpec) (*cron.Schedule, error) {
	now := m.now()

	schedule, err := m.materialize(spec, now)
	if err != nil {
		return nil, err
	}

	m.refreshNextFire(schedule, now)

	// An enabled schedule whose trigger yields nothing from now (a past
	// one-shot, an already-expired window) would be created dead: no journal
	// row, no error, nothing ever fires. Refuse it here; a spent schedule
	// remains updatable so operators can still edit or rename it.
	if schedule.IsEnabled && schedule.NextFireAtUnixMs == nil {
		return nil, cron.ErrScheduleInvalid(ErrScheduleNeverFires.Error())
	}

	if _, err := m.db.NewInsert().Model(schedule).Exec(ctx); err != nil {
		return nil, translateScheduleWriteError(err)
	}

	m.engine.Wake()

	return schedule, nil
}

func (m *scheduleManager) Update(ctx context.Context, name string, spec cron.ScheduleSpec) (*cron.Schedule, error) {
	if spec.Name == "" {
		spec.Name = name
	}

	updated, err := m.materialize(spec, m.now())
	if err != nil {
		return nil, err
	}

	var schedule *cron.Schedule

	err = m.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		current, err := lockScheduleByName(ctx, tx, name)
		if err != nil {
			return err
		}

		// The spec replaces everything but the row identity, the creation
		// audit and the engine-owned fire history; the skipupdate tags keep
		// the audit safe on write anyway. LastFireAtUnixMs is carried over
		// explicitly: reshaping a schedule must not erase what already ran.
		timingChanged := !sameScheduleTiming(current, updated)
		now := m.now()

		updated.ID = current.ID
		updated.CreatedAt = current.CreatedAt
		updated.CreatedBy = current.CreatedBy
		updated.LastFireAtUnixMs = current.LastFireAtUnixMs
		updated.AnchorAtUnixMs = current.AnchorAtUnixMs
		updated.NextFireAtUnixMs = current.NextFireAtUnixMs
		updated.UpdatedAt = timex.DateTime(now)

		// Only a timing-shape change invalidates the persisted cursor. Other
		// edits keep an already-due occurrence and enablement changes follow the
		// same cursor contract as Pause/Resume. A disabled schedule that never
		// had a cursor is armed only when it becomes enabled.
		if timingChanged || (updated.IsEnabled && !current.IsEnabled && current.NextFireAtUnixMs == nil) {
			m.refreshNextFire(updated, now)

			// This edit recomputed the cursor and the trigger yielded nothing:
			// persisting it would leave the schedule enabled but dead, exactly
			// what Create refuses. The check is scoped to the recomputation so
			// an edit that never touched the timing still reaches a spent
			// schedule — renaming or re-pointing one stays possible.
			if updated.IsEnabled && updated.NextFireAtUnixMs == nil {
				return cron.ErrScheduleInvalid(ErrScheduleNeverFires.Error())
			}
		}

		if _, err := tx.NewUpdate().Model(updated).WherePK().Exec(ctx); err != nil {
			return translateScheduleWriteError(err)
		}

		schedule = updated

		return nil
	})
	if err != nil {
		return nil, err
	}

	m.engine.Wake()

	return schedule, nil
}

func (m *scheduleManager) Delete(ctx context.Context, name string) error {
	return m.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		schedule, err := lockScheduleByName(ctx, tx, name)
		if err != nil {
			return err
		}

		if _, err := tx.NewDelete().
			Model((*fireRequest)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("schedule_id", schedule.ID) }).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete fire requests for schedule %q: %w", name, err)
		}

		if _, err := tx.NewDelete().Model(schedule).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("delete schedule %q: %w", name, err)
		}

		return nil
	})
}

func (m *scheduleManager) Pause(ctx context.Context, name string) error {
	return m.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		schedule, err := lockScheduleByName(ctx, tx, name)
		if err != nil {
			return err
		}

		// The fire cursor is deliberately preserved: claiming already filters
		// on is_enabled, so a paused schedule cannot fire, and keeping the
		// cursor is what lets Resume hand the paused gap to the regular
		// misfire decision instead of silently swallowing it.
		schedule.IsEnabled = false
		schedule.UpdatedAt = timex.DateTime(m.now())

		return persistScheduleState(ctx, tx, schedule)
	})
}

func (m *scheduleManager) Resume(ctx context.Context, name string) error {
	err := m.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		schedule, err := lockScheduleByName(ctx, tx, name)
		if err != nil {
			return err
		}

		if schedule.IsEnabled {
			return nil
		}

		schedule.IsEnabled = true

		// A preserved cursor is left exactly where Pause found it, so the
		// next claim applies the schedule's misfire policy to the paused gap
		// — catching up and journaling it the same way downtime is handled.
		// Only a schedule that has no cursor at all (created disabled, or
		// spent) is re-armed from now.
		if schedule.NextFireAtUnixMs == nil {
			m.refreshNextFire(schedule, m.now())
		}

		schedule.UpdatedAt = timex.DateTime(m.now())

		return persistScheduleState(ctx, tx, schedule)
	})
	if err != nil {
		return err
	}

	m.engine.Wake()

	return nil
}

func (m *scheduleManager) TriggerNow(ctx context.Context, name string) error {
	err := m.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		schedule, err := lockScheduleByName(ctx, tx, name)
		if err != nil {
			return err
		}

		if !schedule.IsEnabled {
			return cron.ErrScheduleDisabled
		}

		now := m.now()
		request := &fireRequest{
			ScheduleID:        schedule.ID,
			Kind:              fireRequestManual,
			ScheduledAtUnixMs: now.UnixMilli(),
		}

		if _, err := tx.NewInsert().Model(request).Exec(ctx); err != nil {
			return fmt.Errorf("queue manual fire for schedule %q: %w", name, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	m.engine.Wake()

	return nil
}

func (m *scheduleManager) Get(ctx context.Context, name string) (*cron.Schedule, error) {
	schedule := new(cron.Schedule)

	err := m.db.NewSelect().
		Model(schedule).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("name", name) }).
		Scan(ctx)
	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil, cron.ErrScheduleNotFound
		}

		return nil, fmt.Errorf("load schedule %q: %w", name, err)
	}

	return schedule, nil
}

func (m *scheduleManager) List(ctx context.Context, filter cron.ScheduleFilter) ([]cron.Schedule, error) {
	var schedules []cron.Schedule

	err := m.db.NewSelect().
		Model(&schedules).
		Where(func(cb orm.ConditionBuilder) {
			if filter.JobName != "" {
				cb.Equals("job_name", filter.JobName)
			}

			if filter.Enabled != nil {
				cb.Equals("is_enabled", *filter.Enabled)
			}
		}).
		OrderBy("name").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	return schedules, nil
}

func (m *scheduleManager) ListRuns(ctx context.Context, filter cron.RunFilter) ([]cron.Run, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultRunPageSize
	}

	limit = min(limit, maxRunPageSize)

	var runs []cron.Run

	err := m.db.NewSelect().
		Model(&runs).
		Where(func(cb orm.ConditionBuilder) {
			if filter.ScheduleName != "" {
				cb.Equals("schedule_name", filter.ScheduleName)
			}

			if filter.JobName != "" {
				cb.Equals("job_name", filter.JobName)
			}

			if len(filter.Statuses) > 0 {
				cb.In("status", filter.Statuses)
			}

			if filter.Since != nil {
				cb.GreaterThanOrEqual("scheduled_at_unix_ms", ceilUnixMs(*filter.Since))
			}

			if filter.Until != nil {
				cb.LessThan("scheduled_at_unix_ms", ceilUnixMs(*filter.Until))
			}
		}).
		OrderByDesc("claimed_at_unix_ms").
		OrderByDesc("id").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	return runs, nil
}

// materialize validates the spec and shapes it into a schedule row.
func (m *scheduleManager) materialize(spec cron.ScheduleSpec, now time.Time) (*cron.Schedule, error) {
	name := strings.TrimSpace(spec.Name)

	switch {
	case name == "":
		return nil, cron.ErrScheduleInvalid(ErrScheduleNameRequired.Error())
	case utf8.RuneCountInString(name) > maxScheduleNameLength:
		return nil, cron.ErrScheduleInvalid(ErrScheduleNameTooLong.Error())
	}

	if _, registered := m.registry.Lookup(spec.JobName); !registered {
		return nil, cron.ErrJobNotRegistered
	}

	if err := spec.Trigger.Validate(); err != nil {
		return nil, cron.ErrTriggerInvalid(err.Error())
	}

	misfire, err := normalizeMisfirePolicy(spec.MisfirePolicy)
	if err != nil {
		return nil, err
	}

	concurrency, err := normalizeConcurrencyPolicy(spec.ConcurrencyPolicy)
	if err != nil {
		return nil, err
	}

	if spec.Timeout < 0 {
		return nil, cron.ErrScheduleInvalid(ErrScheduleTimeoutNegative.Error())
	}

	if spec.Timeout%time.Millisecond != 0 {
		return nil, cron.ErrScheduleInvalid(ErrScheduleTimeoutPrecision.Error())
	}

	var startsAtUnixMs, endsAtUnixMs *int64
	if spec.StartsAt != nil {
		startsAtUnixMs = unixMsPtr(*spec.StartsAt)
	}

	if spec.EndsAt != nil {
		endsAtUnixMs = unixMsPtr(*spec.EndsAt)
	}

	if startsAtUnixMs != nil && endsAtUnixMs != nil && *endsAtUnixMs <= *startsAtUnixMs {
		return nil, cron.ErrScheduleInvalid(ErrScheduleWindowInverted.Error())
	}

	params, err := marshalParams(spec.Params)
	if err != nil {
		return nil, err
	}

	timezone := spec.Trigger.Timezone
	if spec.Trigger.Kind == cron.TriggerCron && timezone == "" {
		timezone = cron.DefaultTimezone
	}

	schedule := &cron.Schedule{
		Name:              name,
		JobName:           spec.JobName,
		Kind:              spec.Trigger.Kind,
		Expr:              spec.Trigger.Expr,
		Timezone:          timezone,
		EveryMs:           spec.Trigger.EveryMs,
		Params:            params,
		MisfirePolicy:     misfire,
		ConcurrencyPolicy: concurrency,
		Recover:           spec.Recover,
		TimeoutMs:         spec.Timeout.Milliseconds(),
		IsEnabled:         spec.Enabled == nil || *spec.Enabled,
		AnchorAtUnixMs:    now.UnixMilli(),
	}

	// Stamp the audit fields and absolute interval anchor here rather than
	// leaving creation time to the insert hook.
	schedule.CreatedAt = timex.DateTime(now)
	schedule.UpdatedAt = timex.DateTime(now)

	if spec.Trigger.At != nil {
		schedule.FireAtUnixMs = unixMsPtr(*spec.Trigger.At)
	}

	schedule.StartsAtUnixMs = startsAtUnixMs
	schedule.EndsAtUnixMs = endsAtUnixMs

	return schedule, nil
}

// refreshNextFire recomputes the exact fire cursor after the given instant;
// disabled schedules carry none.
func (*scheduleManager) refreshNextFire(schedule *cron.Schedule, after time.Time) {
	schedule.NextFireAtUnixMs = nil

	if !schedule.IsEnabled {
		return
	}

	if next, ok := nextFire(schedule, after); ok {
		schedule.NextFireAtUnixMs = unixMsPtr(next)
	}
}

func sameScheduleTiming(left, right *cron.Schedule) bool {
	return left.Kind == right.Kind &&
		left.Expr == right.Expr &&
		left.Timezone == right.Timezone &&
		left.EveryMs == right.EveryMs &&
		sameUnixMs(left.FireAtUnixMs, right.FireAtUnixMs) &&
		sameUnixMs(left.StartsAtUnixMs, right.StartsAtUnixMs) &&
		sameUnixMs(left.EndsAtUnixMs, right.EndsAtUnixMs)
}

func sameUnixMs(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}

// lockScheduleByName loads a schedule under a row lock, mapping absence to
// the outward not-found error.
func lockScheduleByName(ctx context.Context, tx orm.DB, name string) (*cron.Schedule, error) {
	schedule := new(cron.Schedule)

	err := tx.NewSelect().
		Model(schedule).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("name", name) }).
		ForUpdate().
		Scan(ctx)
	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil, cron.ErrScheduleNotFound
		}

		return nil, fmt.Errorf("lock schedule %q: %w", name, err)
	}

	return schedule, nil
}

// persistScheduleState writes the scheduling-state columns of a control
// operation (pause, resume).
func persistScheduleState(ctx context.Context, tx orm.DB, schedule *cron.Schedule) error {
	if _, err := tx.NewUpdate().
		Model(schedule).
		Select("is_enabled", "next_fire_at_unix_ms", "updated_at").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("persist schedule %q state: %w", schedule.Name, err)
	}

	return nil
}

// translateScheduleWriteError maps the unique-name violation to the outward
// conflict error.
func translateScheduleWriteError(err error) error {
	if errors.Is(err, result.ErrRecordAlreadyExists) {
		return cron.ErrScheduleExists
	}

	return fmt.Errorf("write schedule: %w", err)
}

// marshalParams normalizes the spec's params into the stored JSON form.
func marshalParams(params any) (json.RawMessage, error) {
	switch value := params.(type) {
	case nil:
		return nil, nil

	case json.RawMessage:
		if len(value) == 0 {
			return nil, nil
		}

		if !json.Valid(value) {
			return nil, cron.ErrScheduleInvalid("params is not valid JSON")
		}

		return value, nil

	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, cron.ErrScheduleInvalid(fmt.Sprintf("params not JSON-encodable: %v", err))
		}

		return encoded, nil
	}
}

// normalizeMisfirePolicy resolves the default and rejects unknown values.
func normalizeMisfirePolicy(policy cron.MisfirePolicy) (cron.MisfirePolicy, error) {
	switch policy {
	case "":
		return cron.MisfireFireNow, nil
	case cron.MisfireFireNow, cron.MisfireSkip:
		return policy, nil
	default:
		return "", cron.ErrScheduleInvalid(fmt.Sprintf("unknown misfire policy %q", policy))
	}
}

// normalizeConcurrencyPolicy resolves the default and rejects unknown values.
func normalizeConcurrencyPolicy(policy cron.ConcurrencyPolicy) (cron.ConcurrencyPolicy, error) {
	switch policy {
	case "":
		return cron.ConcurrencyForbid, nil
	case cron.ConcurrencyForbid, cron.ConcurrencyAllow:
		return policy, nil
	default:
		return "", cron.ErrScheduleInvalid(fmt.Sprintf("unknown concurrency policy %q", policy))
	}
}

// disabledScheduleManager fails every call: the store is off by
// configuration, and saying so beats a missing dependency.
type disabledScheduleManager struct{}

func (disabledScheduleManager) Create(context.Context, cron.ScheduleSpec) (*cron.Schedule, error) {
	return nil, cron.ErrStoreDisabled
}

func (disabledScheduleManager) Update(context.Context, string, cron.ScheduleSpec) (*cron.Schedule, error) {
	return nil, cron.ErrStoreDisabled
}

func (disabledScheduleManager) Delete(context.Context, string) error {
	return cron.ErrStoreDisabled
}

func (disabledScheduleManager) Pause(context.Context, string) error {
	return cron.ErrStoreDisabled
}

func (disabledScheduleManager) Resume(context.Context, string) error {
	return cron.ErrStoreDisabled
}

func (disabledScheduleManager) TriggerNow(context.Context, string) error {
	return cron.ErrStoreDisabled
}

func (disabledScheduleManager) Get(context.Context, string) (*cron.Schedule, error) {
	return nil, cron.ErrStoreDisabled
}

func (disabledScheduleManager) List(context.Context, cron.ScheduleFilter) ([]cron.Schedule, error) {
	return nil, cron.ErrStoreDisabled
}

func (disabledScheduleManager) ListRuns(context.Context, cron.RunFilter) ([]cron.Run, error) {
	return nil, cron.ErrStoreDisabled
}
