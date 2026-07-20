package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const (
	// maxNameLength mirrors the crn_schedule.name column width.
	maxNameLength = 128
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
	schedule, err := m.materialize(spec, m.now())
	if err != nil {
		return nil, err
	}

	m.refreshNextFire(schedule, m.now())

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
		// the audit safe on write anyway. LastFireAt is carried over
		// explicitly: reshaping a schedule must not erase what already ran.
		updated.ID = current.ID
		updated.CreatedAt = current.CreatedAt
		updated.CreatedBy = current.CreatedBy
		updated.LastFireAt = current.LastFireAt
		updated.UpdatedAt = timex.DateTime(m.now())

		// Reshaping recomputes the fire from now: anchored trigger math
		// keeps unchanged timing stable, changed timing takes effect
		// immediately.
		m.refreshNextFire(updated, m.now())

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
	deleted, err := m.db.NewDelete().
		Model((*cron.Schedule)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("name", name) }).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete schedule %q: %w", name, err)
	}

	if affected, _ := deleted.RowsAffected(); affected == 0 {
		return cron.ErrScheduleNotFound
	}

	return nil
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
		if schedule.NextFireAt == nil {
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

		// The cursor is pulled to now unconditionally. Leaving an already-due
		// one in place would hand the manual request to the misfire policy,
		// and MisfireSkip journals an overdue fire as missed without running
		// anything — a trigger-now that quietly does nothing.
		due := timex.DateTime(now)
		schedule.NextFireAt = &due
		schedule.UpdatedAt = due

		return persistScheduleState(ctx, tx, schedule)
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
				cb.GreaterThanOrEqual("scheduled_at", timex.DateTime(*filter.Since))
			}

			if filter.Until != nil {
				cb.LessThan("scheduled_at", timex.DateTime(*filter.Until))
			}
		}).
		OrderByDesc("created_at").
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
	case len(name) > maxNameLength:
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

	if spec.StartsAt != nil && spec.EndsAt != nil && !spec.EndsAt.After(*spec.StartsAt) {
		return nil, cron.ErrScheduleInvalid(ErrScheduleWindowInverted.Error())
	}

	params, err := marshalParams(spec.Params)
	if err != nil {
		return nil, err
	}

	schedule := &cron.Schedule{
		Name:              name,
		JobName:           spec.JobName,
		Kind:              spec.Trigger.Kind,
		Expr:              spec.Trigger.Expr,
		Timezone:          spec.Trigger.Timezone,
		EveryMs:           spec.Trigger.EveryMs,
		Params:            params,
		MisfirePolicy:     misfire,
		ConcurrencyPolicy: concurrency,
		Recover:           spec.Recover,
		TimeoutMs:         spec.Timeout.Milliseconds(),
		IsEnabled:         spec.Enabled == nil || *spec.Enabled,
	}

	// Creation stamps double as the interval anchor, so set them here
	// rather than leaving them to the insert hook.
	schedule.CreatedAt = timex.DateTime(now)
	schedule.UpdatedAt = timex.DateTime(now)

	if spec.Trigger.At != nil {
		at := localDateTime(*spec.Trigger.At)
		schedule.FireAt = &at
	}

	if spec.StartsAt != nil {
		starts := localDateTime(*spec.StartsAt)
		schedule.StartsAt = &starts
	}

	if spec.EndsAt != nil {
		ends := localDateTime(*spec.EndsAt)
		schedule.EndsAt = &ends
	}

	return schedule, nil
}

// localDateTime converts a caller-supplied instant into the store's naive
// wall-clock form. Spec times arrive from Go code in whatever zone the caller
// built them in (cron.Once(nyTime), a StartsAt parsed with an offset);
// persisting that zone's wall clock would make the column denote a different
// instant once it is read back and reinterpreted as local.
func localDateTime(t time.Time) timex.DateTime {
	return timex.DateTime(t.In(time.Local))
}

// refreshNextFire recomputes NextFireAt strictly after the given instant;
// disabled schedules carry none.
func (*scheduleManager) refreshNextFire(schedule *cron.Schedule, after time.Time) {
	schedule.NextFireAt = nil

	if !schedule.IsEnabled {
		return
	}

	if next, ok := nextFire(schedule, after); ok {
		due := timex.DateTime(next)
		schedule.NextFireAt = &due
	}
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
// operation (pause, resume, trigger-now).
func persistScheduleState(ctx context.Context, tx orm.DB, schedule *cron.Schedule) error {
	if _, err := tx.NewUpdate().
		Model(schedule).
		Select("is_enabled", "next_fire_at", "updated_at").
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
