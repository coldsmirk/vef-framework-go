package security

import (
	"context"
	"time"
)

// PasswordMetadataLoader exposes password metadata the framework needs but does
// not own, since the user store belongs to the application. It is the extension
// point behind password-expiry enforcement.
type PasswordMetadataLoader interface {
	// PasswordChangedAt returns when the principal's password was last set. A
	// zero time means the timestamp is unknown, which the expiry checker treats
	// as not-yet-expired rather than forcing a change on incomplete data.
	PasswordChangedAt(ctx context.Context, principal *Principal) (time.Time, error)
}

// ExpiryPasswordChangeChecker forces a password change once the password's age
// exceeds maxAge. It implements PasswordChangeChecker, so it plugs into the
// forced-change challenge flow (compose it with other checkers via
// NewCompositePasswordChangeChecker).
type ExpiryPasswordChangeChecker struct {
	loader PasswordMetadataLoader
	maxAge time.Duration
}

// NewExpiryPasswordChangeChecker creates an expiry checker. A non-positive
// maxAge disables expiry so Check always returns nil. Panics if loader is nil.
func NewExpiryPasswordChangeChecker(loader PasswordMetadataLoader, maxAge time.Duration) *ExpiryPasswordChangeChecker {
	if loader == nil {
		panic("security: PasswordMetadataLoader is required")
	}

	return &ExpiryPasswordChangeChecker{loader: loader, maxAge: maxAge}
}

// Check returns the expired challenge when the password's age exceeds maxAge.
func (c *ExpiryPasswordChangeChecker) Check(ctx context.Context, principal *Principal) (*PasswordChangeChallengeData, error) {
	if c.maxAge <= 0 {
		return nil, nil
	}

	changedAt, err := c.loader.PasswordChangedAt(ctx, principal)
	if err != nil {
		return nil, err
	}

	if changedAt.IsZero() || time.Since(changedAt) < c.maxAge {
		return nil, nil
	}

	return &PasswordChangeChallengeData{
		Reason: PasswordChangeReasonExpired,
		Meta:   map[string]any{"expiredAt": changedAt.Add(c.maxAge)},
	}, nil
}
