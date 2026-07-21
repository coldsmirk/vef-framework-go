package store

import "time"

func unixTime(unixMs int64) time.Time {
	return time.UnixMilli(unixMs).UTC()
}

func unixMsPtr(value time.Time) *int64 {
	unixMs := value.UnixMilli()

	return &unixMs
}

func unixTimePtr(unixMs *int64) *time.Time {
	if unixMs == nil {
		return nil
	}

	value := unixTime(*unixMs)

	return &value
}

// ceilUnixMs maps an instant onto the first representable millisecond at
// or after it. Range filters use it because persisted instants are discrete:
// both an inclusive lower bound and an exclusive upper bound need ceiling.
func ceilUnixMs(value time.Time) int64 {
	unixMs := value.UnixMilli()
	if value.Equal(unixTime(unixMs)) {
		return unixMs
	}

	return unixMs + 1
}
