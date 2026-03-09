package sequence

import "github.com/coldsmirk/vef-framework-go/timex"

// ResetCycle defines when the sequence counter should be reset.
type ResetCycle string

const (
	ResetNone      ResetCycle = "N"
	ResetDaily     ResetCycle = "D"
	ResetWeekly    ResetCycle = "W"
	ResetMonthly   ResetCycle = "M"
	ResetQuarterly ResetCycle = "Q"
	ResetYearly    ResetCycle = "Y"
)

// OverflowStrategy defines the behavior when the sequence value exceeds MaxValue.
type OverflowStrategy string

const (
	// OverflowError returns ErrSequenceOverflow when MaxValue is exceeded. This is the default strategy.
	OverflowError OverflowStrategy = "error"
	// OverflowReset automatically resets the counter to StartValue when MaxValue is exceeded.
	OverflowReset OverflowStrategy = "reset"
	// OverflowExtend allows the sequence number to exceed SeqLength (more digits than configured).
	OverflowExtend OverflowStrategy = "extend"
)

// Rule defines the configuration for serial number generation.
type Rule struct {
	Key              string
	Name             string
	Prefix           string           // empty = no prefix
	Suffix           string           // empty = no suffix
	DateFormat       string           // e.g. "yyyyMMdd", empty = no date part
	SeqLength        int              // zero-padded width
	SeqStep          int              // increment per generation
	StartValue       int              // value after reset (first generated = StartValue + Step)
	MaxValue         int              // upper limit (0 = unlimited)
	OverflowStrategy OverflowStrategy // behavior when MaxValue is reached
	ResetCycle       ResetCycle       // N/D/W/M/Q/Y
	CurrentValue     int              // current counter value
	LastResetAt      *timex.DateTime  // nil = never reset
	IsActive         bool             // whether the rule is active
}

// Clone returns a deep copy of the Rule.
func (r *Rule) Clone() *Rule {
	copied := *r
	if r.LastResetAt != nil {
		resetAt := *r.LastResetAt
		copied.LastResetAt = &resetAt
	}

	return &copied
}
