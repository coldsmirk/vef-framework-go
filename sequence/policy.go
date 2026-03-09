package sequence

import "github.com/coldsmirk/vef-framework-go/timex"

// evaluateReserve checks whether the reservation is valid and whether a counter reset is needed.
// Returns (resetNeeded, error).
func evaluateReserve(rule *Rule, count int, now timex.DateTime) (bool, error) {
	if count < 1 {
		return false, ErrInvalidCount
	}

	resetNeeded := needsResetByCycle(rule, now)
	if !resetNeeded && rule.MaxValue > 0 && rule.CurrentValue+rule.SeqStep*count > rule.MaxValue {
		switch rule.OverflowStrategy {
		case OverflowReset:
			resetNeeded = true
		case OverflowExtend:
			// Keep growing after max value.
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

	last := *rule.LastResetAt
	switch rule.ResetCycle {
	case ResetDaily:
		return last.BeginOfDay() != now.BeginOfDay()
	case ResetWeekly:
		return last.BeginOfWeek() != now.BeginOfWeek()
	case ResetMonthly:
		return last.BeginOfMonth() != now.BeginOfMonth()
	case ResetQuarterly:
		return last.BeginOfQuarter() != now.BeginOfQuarter()
	case ResetYearly:
		return last.Year() != now.Year()
	default:
		return false
	}
}
