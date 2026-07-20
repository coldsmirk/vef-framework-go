package cron

import (
	"fmt"
	"math/big"
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
	// maxDurationMilliseconds is the largest whole-millisecond duration Go can
	// represent without integer wraparound.
	maxDurationMilliseconds = int64((1<<63 - 1) / time.Millisecond)
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
func Every(every time.Duration) TriggerSpec {
	return TriggerSpec{Kind: TriggerInterval, EveryMs: every.Milliseconds()}
}

// Once returns a single-fire trigger.
func Once(at time.Time) TriggerSpec {
	return TriggerSpec{Kind: TriggerOnce, At: &at}
}

// every returns the interval as a duration.
func (t TriggerSpec) every() time.Duration {
	return time.Duration(t.EveryMs) * time.Millisecond
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
		if t.EveryMs <= 0 {
			return fmt.Errorf("%w: %dms is below %s", ErrTriggerIntervalTooShort, t.EveryMs, MinInterval)
		}

		if t.EveryMs > maxDurationMilliseconds {
			return fmt.Errorf("%w: %dms", ErrTriggerIntervalTooLong, t.EveryMs)
		}

		every := t.every()
		if every < MinInterval {
			return fmt.Errorf("%w: %s is below %s", ErrTriggerIntervalTooShort, every, MinInterval)
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
	if t.hasFieldConflict() {
		return time.Time{}, false
	}

	switch t.Kind {
	case TriggerCron:
		schedule, err := exprParser.Parse(t.Expr)
		if err != nil {
			return time.Time{}, false
		}

		location, err := t.location()
		if err != nil {
			return time.Time{}, false
		}

		next := schedule.Next(after.In(location))
		if next.IsZero() {
			return time.Time{}, false
		}

		return next, true

	case TriggerInterval:
		if t.EveryMs <= 0 || t.EveryMs > maxDurationMilliseconds {
			return time.Time{}, false
		}

		every := t.every()
		if every < MinInterval {
			return time.Time{}, false
		}

		if anchor.IsZero() {
			anchor = after
		}

		if after.Before(anchor) {
			return anchor, true
		}

		return after.Add(intervalDelay(after, anchor, every)), true

	case TriggerOnce:
		if t.At == nil || !t.At.After(after) {
			return time.Time{}, false
		}

		return *t.At, true

	default:
		return time.Time{}, false
	}
}

func intervalDelay(after, anchor time.Time, every time.Duration) time.Duration {
	elapsed := after.Sub(anchor)
	if anchor.Add(elapsed).Equal(after) {
		return every - elapsed%every
	}

	var afterSeconds, anchorSeconds, elapsedNanos big.Int
	afterSeconds.SetInt64(after.Unix())
	anchorSeconds.SetInt64(anchor.Unix())
	elapsedNanos.Sub(&afterSeconds, &anchorSeconds)
	elapsedNanos.Mul(&elapsedNanos, big.NewInt(int64(time.Second)))
	elapsedNanos.Add(&elapsedNanos, big.NewInt(int64(after.Nanosecond()-anchor.Nanosecond())))

	var remainder big.Int
	remainder.Mod(&elapsedNanos, big.NewInt(int64(every)))

	return every - time.Duration(remainder.Int64())
}

// Occurrences counts the fire times in the half-open interval (from, to],
// capped at limit so pathological gaps (a per-second expression after days
// of downtime) stay cheap to account for.
func (t TriggerSpec) Occurrences(from, to, anchor time.Time, limit int) int {
	if limit <= 0 || !to.After(from) {
		return 0
	}

	count := 0
	cursor := from

	for count < limit {
		next, ok := t.Next(cursor, anchor)
		if !ok || next.After(to) {
			break
		}

		count++
		cursor = next
	}

	return count
}
