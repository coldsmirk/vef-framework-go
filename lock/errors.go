package lock

import "errors"

var (
	// ErrNotAcquired indicates the lock is held by someone else and the
	// acquisition gave up (immediately, or after the WithWait window).
	ErrNotAcquired = errors.New("lock not acquired")
	// ErrNotHeld indicates a release or refresh on a lease that is no longer
	// owned — it expired, was released already, or was taken over.
	ErrNotHeld = errors.New("lock not held")
)
