package cron

import (
	"encoding/json"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
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
	// ConcurrencyForbid suppresses regular and manual fires and journals them
	// as skipped. Recovery requests remain pending until the active run ends.
	// The default.
	ConcurrencyForbid ConcurrencyPolicy = "forbid"
	// ConcurrencyAllow lets runs of the same schedule overlap.
	ConcurrencyAllow ConcurrencyPolicy = "allow"
)

// Schedule is one persisted trigger: when to fire which job, with which
// params, under which policies. The trigger columns mirror TriggerSpec;
// NextFireAtUnixMs is the scheduling state the store engine claims and advances.
// A nil NextFireAtUnixMs on an enabled schedule means the trigger yields no
// further occurrence (a completed one-shot, an expired window).
type Schedule struct {
	orm.BaseModel `bun:"table:crn_schedule,alias:cs"`
	orm.FullAuditedModel

	// Name uniquely identifies the schedule and is the management key.
	Name string `json:"name" bun:"name"`
	// JobName references the JobHandler that executes the fires.
	JobName string `json:"jobName" bun:"job_name"`

	Kind         TriggerKind `json:"kind" bun:"kind"`
	Expr         string      `json:"expr" bun:"expr"`
	Timezone     string      `json:"timezone" bun:"timezone"`
	EveryMs      int64       `json:"everyMs" bun:"every_ms"`
	FireAtUnixMs *int64      `json:"fireAtUnixMs,omitempty" bun:"fire_at_unix_ms"`

	// StartsAtUnixMs and EndsAtUnixMs bound the fire window; either may be nil.
	// StartsAtUnixMs also anchors the fixed-rate phase of interval triggers.
	StartsAtUnixMs *int64 `json:"startsAtUnixMs,omitempty" bun:"starts_at_unix_ms"`
	EndsAtUnixMs   *int64 `json:"endsAtUnixMs,omitempty" bun:"ends_at_unix_ms"`
	AnchorAtUnixMs int64  `json:"anchorAtUnixMs" bun:"anchor_at_unix_ms"`

	// Params is delivered verbatim to the handler on every run.
	Params json.RawMessage `json:"params,omitempty" bun:"params,type:jsonb,nullzero"`

	MisfirePolicy     MisfirePolicy     `json:"misfirePolicy" bun:"misfire_policy"`
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy" bun:"concurrency_policy"`

	// Recover re-fires a run that did not complete: abandoned mid-execution
	// (its node stopped heartbeating) or canceled by graceful shutdown.
	// Recovery makes delivery at-least-once; the handler must be idempotent.
	Recover bool `json:"recover" bun:"recover"`

	// TimeoutMs bounds one run; zero inherits vef.cron.store.run_timeout.
	TimeoutMs int64 `json:"timeoutMs" bun:"timeout_ms"`

	// IsEnabled is operator-owned: Pause clears it, Resume restores it. The
	// engine only claims enabled schedules.
	IsEnabled bool `json:"isEnabled" bun:"is_enabled"`

	// NextFireAtUnixMs is the next due fire the engine will claim; nil when the
	// trigger yields no further occurrence. Pausing preserves the cursor
	// rather than clearing it — claiming filters on IsEnabled anyway, and
	// keeping it is what lets Resume hand the paused gap to the misfire
	// policy instead of silently dropping it.
	NextFireAtUnixMs *int64 `json:"nextFireAtUnixMs,omitempty" bun:"next_fire_at_unix_ms"`
	// LastFireAtUnixMs records the most recent claimed fire's logical time.
	LastFireAtUnixMs *int64 `json:"lastFireAtUnixMs,omitempty" bun:"last_fire_at_unix_ms"`
}

// Trigger reconstructs the spec form of the schedule's trigger columns. The
// epoch preserves the one-shot instant across timezone folds.
func (s *Schedule) Trigger() TriggerSpec {
	spec := TriggerSpec{
		Kind:     s.Kind,
		Expr:     s.Expr,
		Timezone: s.Timezone,
		EveryMs:  s.EveryMs,
	}

	if s.FireAtUnixMs != nil {
		at := time.UnixMilli(*s.FireAtUnixMs).UTC()
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
	// Timeout bounds one run and must be an exact whole-millisecond duration;
	// zero inherits vef.cron.store.run_timeout.
	Timeout time.Duration `json:"timeout,omitempty"`
	// Enabled sets the initial (or updated) enablement; nil means true.
	Enabled *bool `json:"enabled,omitempty"`
}
