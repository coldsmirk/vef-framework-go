package sequence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestEvaluateReserve(t *testing.T) {
	now := timex.DateTime(time.Date(2024, 6, 15, 10, 0, 0, 0, time.Local))

	t.Run("InvalidCount", func(t *testing.T) {
		rule := &Rule{SeqStep: 1, MaxValue: 0, ResetCycle: ResetNone}

		t.Run("Zero", func(t *testing.T) {
			_, err := evaluateReserve(rule, 0, now)
			assert.ErrorIs(t, err, ErrInvalidCount, "Should reject count=0")
		})

		t.Run("Negative", func(t *testing.T) {
			_, err := evaluateReserve(rule, -1, now)
			assert.ErrorIs(t, err, ErrInvalidCount, "Should reject negative count")
		})
	})

	t.Run("NoOverflow", func(t *testing.T) {
		t.Run("UnlimitedMaxValue", func(t *testing.T) {
			rule := &Rule{SeqStep: 1, CurrentValue: 99999, MaxValue: 0, ResetCycle: ResetNone}

			resetNeeded, err := evaluateReserve(rule, 1, now)

			assert.NoError(t, err, "Should not overflow when MaxValue=0")
			assert.False(t, resetNeeded, "Should not need reset")
		})

		t.Run("ExactlyAtMaxValue", func(t *testing.T) {
			rule := &Rule{SeqStep: 1, CurrentValue: 99, MaxValue: 100, ResetCycle: ResetNone}

			resetNeeded, err := evaluateReserve(rule, 1, now)

			assert.NoError(t, err, "Should not overflow when next value equals MaxValue")
			assert.False(t, resetNeeded, "Should not need reset")
		})

		t.Run("BelowMaxValue", func(t *testing.T) {
			rule := &Rule{SeqStep: 1, CurrentValue: 0, MaxValue: 9999, ResetCycle: ResetNone}

			resetNeeded, err := evaluateReserve(rule, 1, now)

			assert.NoError(t, err, "Should not overflow when well below max")
			assert.False(t, resetNeeded, "Should not need reset")
		})
	})

	t.Run("Overflow", func(t *testing.T) {
		t.Run("ErrorStrategy", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 1, CurrentValue: 100, MaxValue: 100,
				OverflowStrategy: OverflowError, ResetCycle: ResetNone,
			}

			_, err := evaluateReserve(rule, 1, now)

			assert.ErrorIs(t, err, ErrSequenceOverflow, "Should return overflow error")
		})

		t.Run("ResetStrategy", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 1, CurrentValue: 100, MaxValue: 100,
				OverflowStrategy: OverflowReset, ResetCycle: ResetNone,
			}

			resetNeeded, err := evaluateReserve(rule, 1, now)

			assert.NoError(t, err, "Overflow reset should not error")
			assert.True(t, resetNeeded, "Overflow reset should trigger reset")
		})

		t.Run("ExtendStrategy", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 1, CurrentValue: 100, MaxValue: 100,
				OverflowStrategy: OverflowExtend, ResetCycle: ResetNone,
			}

			resetNeeded, err := evaluateReserve(rule, 1, now)

			assert.NoError(t, err, "Overflow extend should not error")
			assert.False(t, resetNeeded, "Overflow extend should not trigger reset")
		})

		t.Run("UnknownStrategyDefaultsToError", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 1, CurrentValue: 100, MaxValue: 100,
				OverflowStrategy: "unknown", ResetCycle: ResetNone,
			}

			_, err := evaluateReserve(rule, 1, now)

			assert.ErrorIs(t, err, ErrSequenceOverflow, "Unknown strategy should default to overflow error")
		})

		t.Run("BatchExceedsMaxValue", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 1, CurrentValue: 98, MaxValue: 100,
				OverflowStrategy: OverflowError, ResetCycle: ResetNone,
			}

			_, err := evaluateReserve(rule, 3, now)

			assert.ErrorIs(t, err, ErrSequenceOverflow, "Batch crossing max should overflow")
		})

		t.Run("StepExceedsMaxValue", func(t *testing.T) {
			rule := &Rule{
				SeqStep: 5, CurrentValue: 98, MaxValue: 100,
				OverflowStrategy: OverflowError, ResetCycle: ResetNone,
			}

			_, err := evaluateReserve(rule, 1, now)

			assert.ErrorIs(t, err, ErrSequenceOverflow, "Large step crossing max should overflow")
		})
	})

	t.Run("CycleResetTakesPriorityOverOverflow", func(t *testing.T) {
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &Rule{
			SeqStep: 1, CurrentValue: 100, MaxValue: 100,
			OverflowStrategy: OverflowError, ResetCycle: ResetDaily,
			LastResetAt: &yesterday,
		}

		resetNeeded, err := evaluateReserve(rule, 1, now)

		assert.NoError(t, err, "Cycle reset should prevent overflow error")
		assert.True(t, resetNeeded, "Cycle reset should take priority over overflow")
	})
}

func TestNeedsResetByCycle(t *testing.T) {
	// 2024-06-15 is a Saturday
	now := timex.DateTime(time.Date(2024, 6, 15, 10, 0, 0, 0, time.Local))

	t.Run("ResetNone", func(t *testing.T) {
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &Rule{ResetCycle: ResetNone, LastResetAt: &yesterday}

		assert.False(t, needsResetByCycle(rule, now), "ResetNone should never reset")
	})

	t.Run("EmptyCycleTreatedAsNone", func(t *testing.T) {
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &Rule{ResetCycle: "", LastResetAt: &yesterday}

		assert.False(t, needsResetByCycle(rule, now), "Empty cycle should be treated as ResetNone")
	})

	t.Run("NilLastResetAtAlwaysResets", func(t *testing.T) {
		rule := &Rule{ResetCycle: ResetDaily, LastResetAt: nil}

		assert.True(t, needsResetByCycle(rule, now), "Nil LastResetAt should trigger reset for first use")
	})

	t.Run("Daily", func(t *testing.T) {
		t.Run("SameDay", func(t *testing.T) {
			earlier := timex.DateTime(time.Date(2024, 6, 15, 1, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetDaily, LastResetAt: &earlier}

			assert.False(t, needsResetByCycle(rule, now), "Same day should not reset")
		})

		t.Run("DifferentDay", func(t *testing.T) {
			yesterday := timex.DateTime(time.Date(2024, 6, 14, 23, 59, 59, 0, time.Local))
			rule := &Rule{ResetCycle: ResetDaily, LastResetAt: &yesterday}

			assert.True(t, needsResetByCycle(rule, now), "Different day should reset")
		})
	})

	t.Run("Weekly", func(t *testing.T) {
		t.Run("SameWeek", func(t *testing.T) {
			// 2024-06-10 is Monday, same week as 2024-06-15 (Saturday)
			monday := timex.DateTime(time.Date(2024, 6, 10, 8, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetWeekly, LastResetAt: &monday}

			assert.False(t, needsResetByCycle(rule, now), "Same week should not reset")
		})

		t.Run("DifferentWeek", func(t *testing.T) {
			// 2024-06-08 is Saturday of the previous week
			lastSaturday := timex.DateTime(time.Date(2024, 6, 8, 23, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetWeekly, LastResetAt: &lastSaturday}

			assert.True(t, needsResetByCycle(rule, now), "Different week should reset")
		})
	})

	t.Run("Monthly", func(t *testing.T) {
		t.Run("SameMonth", func(t *testing.T) {
			firstOfMonth := timex.DateTime(time.Date(2024, 6, 1, 0, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetMonthly, LastResetAt: &firstOfMonth}

			assert.False(t, needsResetByCycle(rule, now), "Same month should not reset")
		})

		t.Run("DifferentMonth", func(t *testing.T) {
			lastMonth := timex.DateTime(time.Date(2024, 5, 31, 23, 59, 59, 0, time.Local))
			rule := &Rule{ResetCycle: ResetMonthly, LastResetAt: &lastMonth}

			assert.True(t, needsResetByCycle(rule, now), "Different month should reset")
		})
	})

	t.Run("Quarterly", func(t *testing.T) {
		t.Run("SameQuarter", func(t *testing.T) {
			// Q2 2024: April-June. June 15 and April 1 are same quarter.
			aprilFirst := timex.DateTime(time.Date(2024, 4, 1, 0, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetQuarterly, LastResetAt: &aprilFirst}

			assert.False(t, needsResetByCycle(rule, now), "Same quarter should not reset")
		})

		t.Run("DifferentQuarter", func(t *testing.T) {
			// Q1 2024: Jan-March
			marchEnd := timex.DateTime(time.Date(2024, 3, 31, 23, 59, 59, 0, time.Local))
			rule := &Rule{ResetCycle: ResetQuarterly, LastResetAt: &marchEnd}

			assert.True(t, needsResetByCycle(rule, now), "Different quarter should reset")
		})
	})

	t.Run("Yearly", func(t *testing.T) {
		t.Run("SameYear", func(t *testing.T) {
			janFirst := timex.DateTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local))
			rule := &Rule{ResetCycle: ResetYearly, LastResetAt: &janFirst}

			assert.False(t, needsResetByCycle(rule, now), "Same year should not reset")
		})

		t.Run("DifferentYear", func(t *testing.T) {
			lastYear := timex.DateTime(time.Date(2023, 12, 31, 23, 59, 59, 0, time.Local))
			rule := &Rule{ResetCycle: ResetYearly, LastResetAt: &lastYear}

			assert.True(t, needsResetByCycle(rule, now), "Different year should reset")
		})
	})

	t.Run("UnknownCycleDoesNotReset", func(t *testing.T) {
		yesterday := timex.DateTime(time.Time(now).AddDate(0, 0, -1))
		rule := &Rule{ResetCycle: "X", LastResetAt: &yesterday}

		assert.False(t, needsResetByCycle(rule, now), "Unknown cycle should not trigger reset")
	})
}
