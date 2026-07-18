package cron

import (
	"encoding/json"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// MisfirePolicy decides what happens to fire times that were missed for
// longer than the configured misfire threshold (downtime, paused schedule,
// no free executor). Whichever policy applies, occurrences that will never
// run are journaled as a single missed run covering the whole gap.
type MisfirePolicy string

const (
	// MisfireFireNow runs one catch-up fire immediately and resumes the
	// regular sequence from now. The default.
	MisfireFireNow MisfirePolicy = "fire_now"
	// MisfireSkip advances to the next future fire without running.
	MisfireSkip MisfirePolicy = "skip"
)

// ConcurrencyPolicy decides whether a fire may start while a previous run of
// the same schedule is still executing.
type ConcurrencyPolicy string

const (
	// ConcurrencyForbid suppresses the fire and journals it as skipped. The
	// default.
	ConcurrencyForbid ConcurrencyPolicy = "forbid"
	// ConcurrencyAllow lets runs of the same schedule overlap.
	ConcurrencyAllow ConcurrencyPolicy = "allow"
)

// Schedule is one persisted trigger: when to fire which job, with which
// params, under which policies. The trigger columns mirror TriggerSpec;
// NextFireAt is the scheduling state the store engine claims and advances.
// A nil NextFireAt on an enabled schedule means the trigger yields no
// further occurrence (a completed one-shot, an expired window).
type Schedule struct {
	orm.BaseModel `bun:"table:crn_schedule,alias:cs"`
	orm.FullAuditedModel

	// Name uniquely identifies the schedule and is the management key.
	Name string `json:"name" bun:"name"`
	// JobName references the JobHandler that executes the fires.
	JobName string `json:"jobName" bun:"job_name"`

	Kind     TriggerKind     `json:"kind" bun:"kind"`
	Expr     string          `json:"expr" bun:"expr"`
	Timezone string          `json:"timezone" bun:"timezone"`
	EveryMs  int64           `json:"everyMs" bun:"every_ms"`
	FireAt   *timex.DateTime `json:"fireAt,omitempty" bun:"fire_at,nullzero"`

	// StartsAt and EndsAt bound the fire window; either may be nil. StartsAt
	// also anchors the fixed-rate phase of interval triggers.
	StartsAt *timex.DateTime `json:"startsAt,omitempty" bun:"starts_at,nullzero"`
	EndsAt   *timex.DateTime `json:"endsAt,omitempty" bun:"ends_at,nullzero"`

	// Params is delivered verbatim to the handler on every run.
	Params json.RawMessage `json:"params,omitempty" bun:"params,type:jsonb,nullzero"`

	MisfirePolicy     MisfirePolicy     `json:"misfirePolicy" bun:"misfire_policy"`
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy" bun:"concurrency_policy"`

	// Recover re-fires a run that was abandoned mid-execution (its node
	// stopped heartbeating) as soon as possible. Recovery makes delivery
	// at-least-once; the handler must be idempotent.
	Recover bool `json:"recover" bun:"recover"`

	// TimeoutMs bounds one run; zero inherits vef.cron.store.run_timeout.
	TimeoutMs int64 `json:"timeoutMs" bun:"timeout_ms"`

	// IsEnabled is operator-owned: Pause clears it, Resume restores it. The
	// engine only claims enabled schedules.
	IsEnabled bool `json:"isEnabled" bun:"is_enabled"`

	// NextFireAt is the next due fire the engine will claim; nil when the
	// schedule is paused or yields no further occurrence.
	NextFireAt *timex.DateTime `json:"nextFireAt,omitempty" bun:"next_fire_at,nullzero"`
	// LastFireAt records the most recent claimed fire's logical time.
	LastFireAt *timex.DateTime `json:"lastFireAt,omitempty" bun:"last_fire_at,nullzero"`
}

// Trigger reconstructs the spec form of the schedule's trigger columns. The
// one-shot fire time is reinterpreted as local wall clock (timestamps are
// stored timezone-naive), so the spec computes correctly against Now
// regardless of how a database driver labeled the scanned value.
func (s *Schedule) Trigger() TriggerSpec {
	spec := TriggerSpec{
		Kind:     s.Kind,
		Expr:     s.Expr,
		Timezone: s.Timezone,
		EveryMs:  s.EveryMs,
	}

	if s.FireAt != nil {
		at := s.FireAt.AsLocal()
		spec.At = &at
	}

	return spec
}

// Timeout returns the per-run timeout, zero when unset.
func (s *Schedule) Timeout() time.Duration {
	return time.Duration(s.TimeoutMs) * time.Millisecond
}

// ScheduleSpec declares a schedule to create, or the desired state of an
// update. Zero-valued policies resolve to their defaults (MisfireFireNow,
// ConcurrencyForbid); a nil Enabled resolves to true.
type ScheduleSpec struct {
	// Name uniquely identifies the schedule; on a seeded default schedule it
	// falls back to the job name.
	Name string `json:"name"`
	// JobName references the registered JobHandler to execute.
	JobName string `json:"jobName"`
	// Trigger declares when the schedule fires.
	Trigger TriggerSpec `json:"trigger"`
	// Params is JSON-marshaled and delivered to the handler on every run;
	// json.RawMessage passes through verbatim.
	Params any `json:"params,omitempty"`
	// StartsAt and EndsAt bound the fire window; either may be nil.
	StartsAt *time.Time `json:"startsAt,omitempty"`
	EndsAt   *time.Time `json:"endsAt,omitempty"`

	MisfirePolicy     MisfirePolicy     `json:"misfirePolicy,omitempty"`
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// Recover re-fires abandoned runs; see Schedule.Recover.
	Recover bool `json:"recover,omitempty"`
	// Timeout bounds one run; zero inherits vef.cron.store.run_timeout.
	Timeout time.Duration `json:"timeout,omitempty"`
	// Enabled sets the initial (or updated) enablement; nil means true.
	Enabled *bool `json:"enabled,omitempty"`
}
