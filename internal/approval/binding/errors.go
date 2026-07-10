package binding

import "errors"

// ErrBindingMisconfigured signals that a business-bound Flow carries an
// incomplete or unsafe BusinessBindingConfig. The Listener acknowledges
// instead of retrying because configuration errors do not heal by retry.
var (
	ErrBindingMisconfigured   = errors.New("approval: business binding misconfigured")
	ErrInvalidBusinessRef     = errors.New("approval: invalid business reference")
	ErrBindingTargetMissing   = errors.New("approval: business binding target not found")
	ErrBindingTargetNotUnique = errors.New("approval: business binding target is not unique")
)

func isPermanentError(err error) bool {
	return errors.Is(err, ErrBindingMisconfigured) ||
		errors.Is(err, ErrInvalidBusinessRef) ||
		errors.Is(err, ErrBindingTargetMissing) ||
		errors.Is(err, ErrBindingTargetNotUnique)
}
