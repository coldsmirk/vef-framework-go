package service

import "errors"

var (
	// ErrSecretKeyMissing indicates an encrypted auth parameter was loaded
	// but vef.integration.secret_key is not configured, so it cannot be
	// decrypted.
	ErrSecretKeyMissing = errors.New("integration: encrypted auth parameter found but vef.integration.secret_key is not configured")

	// ErrMaskedSecretWithoutPrior indicates a save submitted the masked
	// placeholder for a sensitive parameter that has no stored value to keep.
	ErrMaskedSecretWithoutPrior = errors.New("integration: masked auth parameter has no stored value")
)
