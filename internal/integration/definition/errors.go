package definition

import "errors"

var (
	// ErrSecretKeyMissing indicates an encrypted auth parameter was loaded
	// but vef.integration.secret_key is not configured, so it cannot be
	// decrypted.
	ErrSecretKeyMissing = errors.New("integration: encrypted auth parameter found but vef.integration.secret_key is not configured")

	// ErrMaskedSecretWithoutPrior indicates a save submitted the masked
	// placeholder for a sensitive parameter that has no stored value to keep.
	ErrMaskedSecretWithoutPrior = errors.New("integration: masked auth parameter has no stored value")

	// ErrEmptyCodeValue rejects an empty-string value in a code map entry.
	ErrEmptyCodeValue = errors.New("code value must not be empty")

	// ErrNonScalarCodeValue rejects a code map value outside the JSON scalar
	// types lookups can normalize.
	ErrNonScalarCodeValue = errors.New("code value must be a JSON string, number, or boolean")

	// ErrDuplicateCodeValue rejects a code map side reaching the same lookup
	// value through two entries — lookups would turn order-dependent.
	ErrDuplicateCodeValue = errors.New("duplicate code value")
)
