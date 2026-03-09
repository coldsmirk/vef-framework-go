package shared

import "github.com/coldsmirk/vef-framework-go/timex"

// ComputeTaskDeadline calculates a task deadline from timeout hours.
// Returns nil when timeout is disabled.
func ComputeTaskDeadline(timeoutHours int) *timex.DateTime {
	if timeoutHours <= 0 {
		return nil
	}

	return new(timex.Now().AddHours(timeoutHours))
}
