package sequence

import "github.com/coldsmirk/vef-framework-go/timex"

// evaluateReserve checks whether the reservation is valid and whether a counter reset is needed.
// Returns (resetNeeded, error).
func evaluateReserve(rule *Rule, count int, now timex.DateTime) (bool, error) {
	if count < 1 {
		return false, ErrInvalidCount
	}

	// Every store funnels through here, so a misconfigured step is rejected
	// on the first reservation instead of silently issuing duplicate (step 0)
	// or regressing (negative step) serial numbers.
	if rule.SeqStep < 1 {
		return false, ErrInvalidStep
	}

	resetNeeded := needsResetByCycle(rule, now)

	// A cycle reset rebases the counter to StartValue before incrementing;
	// the overflow ceiling must then be evaluated against that post-reset base
	// rather than the stale CurrentValue, otherwise OverflowError could be
	// silently violated on the reset boundary.
	base := rule.CurrentValue
	if resetNeeded {
		base = rule.StartValue
	}

	if rule.MaxValue > 0 && base+rule.SeqStep*count > rule.MaxValue {
		switch rule.OverflowStrategy {
		case OverflowReset:
			// Resetting again cannot help once the post-reset batch already
			// exceeds MaxValue, so this is a genuine misconfiguration.
			if resetNeeded {
				return false, ErrSequenceOverflow
			}

			return true, nil

		case OverflowExtend:
			// Keep growing past MaxValue without erroring.
		default:
			return false, ErrSequenceOverflow
		}
	}

	return resetNeeded, nil
}

func needsResetByCycle(rule *Rule, now timex.DateTime) bool {
	if rule.ResetCycle == "" || rule.ResetCycle == ResetNone {
		return false
	}

	if rule.LastResetAt == nil {
		return true
	}

	// Both sides are read as local wall-clock time and their period starts
	// compared by value. DateTime is timezone-naive: last_reset_at is stored
	// as a bare local wall-clock string, and drivers hand it back labeled UTC
	// (pgdriver and MySQL for timestamp columns, the SQLite drivers) while now
	// is local. Compared as they came — and with !=, which also compares the
	// Location, so even a server running in UTC saw two different midnights —
	// every reservation looked like a new period, reset the counter to
	// StartValue, and reissued the same serial numbers.
	last := timex.DateTime(rule.LastResetAt.AsLocal())
	current := timex.DateTime(now.AsLocal())

	switch rule.ResetCycle {
	case ResetDaily:
		return !last.BeginOfDay().Equal(current.BeginOfDay())
	case ResetWeekly:
		return !last.BeginOfWeek().Equal(current.BeginOfWeek())
	case ResetMonthly:
		return !last.BeginOfMonth().Equal(current.BeginOfMonth())
	case ResetQuarterly:
		return !last.BeginOfQuarter().Equal(current.BeginOfQuarter())
	case ResetYearly:
		return last.Year() != current.Year()
	default:
		return false
	}
}
