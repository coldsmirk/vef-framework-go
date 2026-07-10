package binding

import "errors"

// ErrBindingMisconfigured signals that a business-bound Flow carries an
// incomplete or unsafe BusinessBindingConfig.
var (
	ErrBindingMisconfigured     = errors.New("approval: business binding misconfigured")
	ErrInvalidBusinessRef       = errors.New("approval: invalid business reference")
	ErrBindingTargetMissing     = errors.New("approval: business binding target not found")
	ErrBindingTargetNotUnique   = errors.New("approval: business binding target is not unique")
	ErrBindingOwnershipConflict = errors.New("approval: business binding owner changed")
	ErrProjectionStateInvalid   = errors.New("approval: business projection state is invalid")
)
