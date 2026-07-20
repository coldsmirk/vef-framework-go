package store

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/cron"
)

// missedCap bounds per-claim misfire accounting so pathological gaps (a
// per-second expression after days of downtime) stay cheap; a capped count
// still journals, it just stops counting.
const missedCap = 10_000

// anchorOf returns the fixed-rate phase anchor of interval triggers: the
// window start when set, else the schedule's creation.
func anchorOf(schedule *cron.Schedule) time.Time {
	if schedule.StartsAtUnixMs != nil {
		return unixTime(*schedule.StartsAtUnixMs)
	}

	return unixTime(schedule.AnchorAtUnixMs)
}

// nextFire computes the schedule's next fire strictly after the given
// instant, honoring the StartsAt/EndsAt window; ok=false when the trigger
// yields no further occurrence inside it.
func nextFire(schedule *cron.Schedule, after time.Time) (time.Time, bool) {
	if schedule.StartsAtUnixMs != nil {
		// The window start itself is a valid fire time; Next is
		// strictly-after, so probe from just before it.
		floor := unixTime(*schedule.StartsAtUnixMs).Add(-time.Nanosecond)
		if after.Before(floor) {
			after = floor
		}
	}

	next, ok := schedule.Trigger().Next(after, anchorOf(schedule))
	if !ok {
		return time.Time{}, false
	}

	if schedule.EndsAtUnixMs != nil && next.After(unixTime(*schedule.EndsAtUnixMs)) {
		return time.Time{}, false
	}

	return next, true
}

// fireDecision resolves one due schedule at claim time: what executes, what
// is accounted as missed, and where NextFireAtUnixMs advances to.
type fireDecision struct {
	// fire is whether an executable occurrence was claimed; scheduledAt is
	// its logical time.
	fire        bool
	scheduledAt time.Time
	// missed counts occurrences that will never run; missedFrom is the
	// earliest of them. Zero missed means no misfire accounting.
	missed     int
	missedFrom time.Time
	// next is the schedule's new exact fire cursor; nil when the trigger is spent.
	next *time.Time
}

// decide resolves one due schedule. An on-time fire (lateness within the
// misfire threshold) executes at its logical time and advances normally. A
// misfired schedule follows its policy: MisfireFireNow executes one catch-up
// at the oldest due occurrence and accounts the rest as missed;
// MisfireSkip accounts them all. Either way NextFireAtUnixMs advances strictly
// past now, so one decision consumes the whole gap.
func decide(schedule *cron.Schedule, now time.Time, misfireThreshold time.Duration) fireDecision {
	due := unixTime(*schedule.NextFireAtUnixMs)

	if now.Sub(due) <= misfireThreshold {
		decision := fireDecision{fire: true, scheduledAt: due}
		if next, ok := nextFire(schedule, due); ok {
			decision.next = &next
		}

		return decision
	}

	// Misfired: account every occurrence in (due, min(now, EndsAt)] on top
	// of the due one, then advance past now.
	horizon := now
	if schedule.EndsAtUnixMs != nil {
		ends := unixTime(*schedule.EndsAtUnixMs)
		if ends.Before(now) {
			horizon = ends
		}
	}

	trigger := schedule.Trigger()
	anchor := anchorOf(schedule)
	overdue := trigger.Occurrences(due, horizon, anchor, missedCap)

	var next *time.Time
	if n, ok := nextFire(schedule, now); ok {
		next = &n
	}

	if schedule.MisfirePolicy == cron.MisfireSkip {
		return fireDecision{missed: 1 + overdue, missedFrom: due, next: next}
	}

	// MisfireFireNow (and, defensively, any unknown policy): one catch-up
	// run at the oldest due occurrence; the rest of the gap is missed.
	decision := fireDecision{fire: true, scheduledAt: due, missed: overdue, next: next}
	if overdue > 0 {
		if first, ok := trigger.Next(due, anchor); ok {
			decision.missedFrom = first
		}
	}

	return decision
}
