package store

import "time"

func unixTime(unixMs int64) time.Time {
	return time.UnixMilli(unixMs).UTC()
}

func unixMillisPtr(value time.Time) *int64 {
	unixMs := value.UnixMilli()

	return &unixMs
}

// ceilUnixMilli maps an instant onto the first representable millisecond at
// or after it. Range filters use it because persisted instants are discrete:
// both an inclusive lower bound and an exclusive upper bound need ceiling.
func ceilUnixMilli(value time.Time) int64 {
	unixMs := value.UnixMilli()
	if value.Equal(unixTime(unixMs)) {
		return unixMs
	}

	return unixMs + 1
}
