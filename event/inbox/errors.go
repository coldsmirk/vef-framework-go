package inbox

import "errors"

// ErrInProgress indicates a duplicate delivery arrived while the
// original delivery still has a valid processing lease.
var ErrInProgress = errors.New("event inbox: delivery already in progress")

// ErrLockLost indicates the caller no longer owns the processing
// lease for a delivery it previously acquired.
var ErrLockLost = errors.New("event inbox: processing lock lost")

// ErrUnknownAcquireResult indicates a repository returned an acquire
// result the middleware does not understand.
var ErrUnknownAcquireResult = errors.New("event inbox: unknown acquire result")

// ErrMissingLockID indicates a repository reported an acquired
// delivery without returning its processing lock id.
var ErrMissingLockID = errors.New("event inbox: missing lock id")
