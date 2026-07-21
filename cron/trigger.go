package cron

import (
	"fmt"
	"time"

	// Schedules carry IANA timezones; the embedded tzdata keeps LoadLocation
	// working on zoneinfo-less deployments (CGO_ENABLED=0 scratch images).
	// Hosts with system zoneinfo are unaffected — Go consults it first.
	_ "time/tzdata"

	cronv3 "github.com/robfig/cron/v3"
)

// TriggerKind identifies how a schedule derives its fire times.
type TriggerKind string

const (
	// TriggerCron derives fire times from a cron expression evaluated in an
	// IANA timezone.
	TriggerCron TriggerKind = "cron"
	// TriggerInterval fires at a fixed rate anchored to the schedule's start,
	// so the fire phase stays stable regardless of catch-ups or manual fires.
	TriggerInterval TriggerKind = "interval"
	// TriggerOnce fires a single time.
	TriggerOnce TriggerKind = "once"
)

const (
	// MinInterval is the smallest fixed rate an interval trigger accepts.
	MinInterval = time.Second
	// DefaultTimezone is the deterministic zone used when a cron trigger does
	// not name one explicitly. Durable schedules must never depend on a node's
	// process-local timezone.
	DefaultTimezone = "UTC"
	// MaxDurationMilliseconds is the largest whole-millisecond count a
	// time.Duration can represent; interval and timeout inputs beyond it
	// cannot round-trip through Go durations without wraparound.
	MaxDurationMilliseconds = int64((1<<63 - 1) / time.Millisecond)
)

// exprParser accepts standard 5-field expressions, 6-field expressions with a
// leading seconds field, and @-descriptors (@daily, @every 90m, ...).
var exprParser = cronv3.NewParser(
	cronv3.SecondOptional | cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow | cronv3.Descriptor,
)

// TriggerSpec is the declarative trigger of a schedule: exactly one kind with
// its kind-specific fields. Build specs with Expr, Every, or Once.
type TriggerSpec struct {
	// Kind selects the trigger semantics.
	Kind TriggerKind `json:"kind"`
	// Expr is the cron expression of a TriggerCron trigger; 5 or 6 fields
	// (leading seconds optional) and @-descriptors are accepted.
	Expr string `json:"expr,omitempty"`
	// Timezone is the IANA zone a TriggerCron expression is evaluated in;
	// empty resolves to DefaultTimezone.
	Timezone string `json:"timezone,omitempty"`
	// EveryMs is the fixed rate of a TriggerInterval trigger, in milliseconds.
	EveryMs int64 `json:"everyMs,omitempty"`
	// At is the single fire time of a TriggerOnce trigger.
	At *time.Time `json:"at,omitempty"`
}

// Expr returns a cron-expression trigger evaluated in the given IANA
// timezone; an empty timezone resolves to DefaultTimezone.
func Expr(expr, timezone string) TriggerSpec {
	if timezone == "" {
		timezone = DefaultTimezone
	}

	return TriggerSpec{Kind: TriggerCron, Expr: expr, Timezone: timezone}
}

// Every returns a fixed-rate trigger. The rate is anchored to the schedule's
// start (StartsAt, else its creation time), keeping the fire phase stable.
// Schedules persist the rate in whole milliseconds, so a finer-grained
// duration truncates toward zero here; rates below MinInterval are refused by
// Validate either way.
func Every(every time.Duration) TriggerSpec {
	return TriggerSpec{Kind: TriggerInterval, EveryMs: every.Milliseconds()}
}

// Once returns a single-fire trigger.
func Once(at time.Time) TriggerSpec {
	return TriggerSpec{Kind: TriggerOnce, At: &at}
}

// location resolves the trigger's evaluation zone.
func (t TriggerSpec) location() (*time.Location, error) {
	if t.Timezone == "" {
		return time.UTC, nil
	}

	if t.Timezone == "Local" {
		return nil, ErrTriggerTimezoneInvalid
	}

	return time.LoadLocation(t.Timezone)
}

func (t TriggerSpec) hasFieldConflict() bool {
	switch t.Kind {
	case TriggerCron:
		return t.EveryMs != 0 || t.At != nil
	case TriggerInterval:
		return t.Expr != "" || t.Timezone != "" || t.At != nil
	case TriggerOnce:
		return t.Expr != "" || t.Timezone != "" || t.EveryMs != 0
	default:
		return false
	}
}

// Validate checks the spec for structural soundness: a known kind, a parsable
// expression and loadable timezone for cron triggers, a rate of at least
// MinInterval for interval triggers, and a fire time for one-shot triggers.
func (t TriggerSpec) Validate() error {
	if t.hasFieldConflict() {
		return fmt.Errorf("%w: %s", ErrTriggerFieldsConflict, t.Kind)
	}

	switch t.Kind {
	case TriggerCron:
		if t.Expr == "" {
			return ErrTriggerExprRequired
		}

		if _, err := exprParser.Parse(t.Expr); err != nil {
			return fmt.Errorf("%w %q: %w", ErrTriggerExprInvalid, t.Expr, err)
		}

		if _, err := t.location(); err != nil {
			return fmt.Errorf("%w %q", ErrTriggerTimezoneInvalid, t.Timezone)
		}

		return nil

	case TriggerInterval:
		if t.EveryMs > MaxDurationMilliseconds {
			return fmt.Errorf("%w: %dms", ErrTriggerIntervalTooLong, t.EveryMs)
		}

		if t.EveryMs < MinInterval.Milliseconds() {
			return fmt.Errorf("%w: %dms is below %s", ErrTriggerIntervalTooShort, t.EveryMs, MinInterval)
		}

		return nil

	case TriggerOnce:
		if t.At == nil || t.At.IsZero() {
			return ErrTriggerFireTimeRequired
		}

		return nil

	default:
		return fmt.Errorf("%w %q", ErrTriggerKindUnknown, t.Kind)
	}
}

// Next returns the first fire time strictly after the given instant, or
// ok=false when the trigger yields no further occurrence. The anchor fixes
// the fixed-rate phase of interval triggers and is ignored by other kinds.
// The spec must have passed Validate; an invalid spec yields no occurrence.
func (t TriggerSpec) Next(after, anchor time.Time) (time.Time, bool) {
	compiled, ok := t.compile()
	if !ok {
		return time.Time{}, false
	}

	return compiled.next(after, anchor)
}

// Occurrences counts the fire times in the half-open interval (from, to],
// capped at limit so pathological gaps (a per-second expression after days
// of downtime) stay cheap to account for.
func (t TriggerSpec) Occurrences(from, to, anchor time.Time, limit int) int {
	if limit <= 0 || !to.After(from) {
		return 0
	}

	compiled, ok := t.compile()
	if !ok {
		return 0
	}

	count := 0
	cursor := from

	for count < limit {
		next, ok := compiled.next(cursor, anchor)
		if !ok || next.After(to) {
			break
		}

		count++
		cursor = next
	}

	return count
}

// compiledTrigger carries the parse-once artifacts of a structurally valid
// spec, so occurrence loops (misfire accounting over a long gap) do not
// re-parse the expression or reload the timezone on every step.
type compiledTrigger struct {
	spec     TriggerSpec
	schedule cronv3.Schedule
	location *time.Location
}

// compile checks the spec exactly as Next used to and captures the cron
// parse artifacts; ok=false mirrors "an invalid spec yields no occurrence".
func (t TriggerSpec) compile() (compiledTrigger, bool) {
	if t.hasFieldConflict() {
		return compiledTrigger{}, false
	}

	switch t.Kind {
	case TriggerCron:
		schedule, err := exprParser.Parse(t.Expr)
		if err != nil {
			return compiledTrigger{}, false
		}

		location, err := t.location()
		if err != nil {
			return compiledTrigger{}, false
		}

		return compiledTrigger{spec: t, schedule: schedule, location: location}, true

	case TriggerInterval:
		if t.EveryMs < MinInterval.Milliseconds() || t.EveryMs > MaxDurationMilliseconds {
			return compiledTrigger{}, false
		}

		return compiledTrigger{spec: t}, true

	case TriggerOnce:
		if t.At == nil {
			return compiledTrigger{}, false
		}

		return compiledTrigger{spec: t}, true

	default:
		return compiledTrigger{}, false
	}
}

func (c compiledTrigger) next(after, anchor time.Time) (time.Time, bool) {
	switch c.spec.Kind {
	case TriggerCron:
		next := c.schedule.Next(after.In(c.location))
		if next.IsZero() {
			return time.Time{}, false
		}

		return next, true

	case TriggerInterval:
		every := time.Duration(c.spec.EveryMs) * time.Millisecond

		if anchor.IsZero() {
			anchor = after
		}

		if after.Before(anchor) {
			return anchor, true
		}

		// Sub saturates once the gap exceeds ~292 years, so the phase of such
		// a degenerate anchor drifts, but the delay stays within (0, every]
		// and the fire remains strictly after the query instant.
		return after.Add(every - after.Sub(anchor)%every), true

	case TriggerOnce:
		if !c.spec.At.After(after) {
			return time.Time{}, false
		}

		return *c.spec.At, true

	default:
		return time.Time{}, false
	}
}
