package binding

import "errors"

var (
	// ErrBindingMisconfigured signals that a business-bound Flow carries an
	// incomplete or unsafe BusinessBindingConfig.
	ErrBindingMisconfigured = errors.New("approval: business binding misconfigured")

	// ErrInvalidBusinessRef signals that an instance's BusinessRef could not be
	// resolved, encoded, or decoded into the configured key columns.
	ErrInvalidBusinessRef = errors.New("approval: invalid business reference")

	// ErrBindingTargetMissing signals that no business row matches the
	// projection's record key.
	ErrBindingTargetMissing = errors.New("approval: business binding target not found")

	// ErrBindingTargetNotUnique signals that the record key matches more than
	// one business row — runtime drift from the save-time-validated unique key.
	ErrBindingTargetNotUnique = errors.New("approval: business binding target is not unique")

	// ErrBindingOwnershipConflict signals that the business row's instance-ID
	// column no longer holds the applied owner, so the compare-and-set fence
	// refuses the write.
	ErrBindingOwnershipConflict = errors.New("approval: business binding owner changed")

	// ErrProjectionStateInvalid signals a projection whose stored state is
	// internally inconsistent (missing row, mismatched target data, or an
	// impossible status/consistency/finish-time combination).
	ErrProjectionStateInvalid = errors.New("approval: business projection state is invalid")
)
