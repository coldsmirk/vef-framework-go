package security

import (
	"context"

	"github.com/pquerna/otp/totp"
)

const (
	ChallengeTypeTOTP      = "totp"
	TOTPDefaultDestination = "Authenticator App"
)

// TOTPSecretLoader loads a user's TOTP secret.
type TOTPSecretLoader interface {
	// LoadSecret returns the base32-encoded TOTP secret for the given principal.
	// An empty string means the user has not configured TOTP.
	LoadSecret(ctx context.Context, principal *Principal) (string, error)
}

// TOTPEvaluator checks whether a user needs TOTP verification, implementing OTPEvaluator.
type TOTPEvaluator struct {
	loader      TOTPSecretLoader
	destination string
}

// NewTOTPEvaluator creates a TOTPEvaluator with the given loader and optional configuration.
func NewTOTPEvaluator(loader TOTPSecretLoader, opts ...TOTPOption) *TOTPEvaluator {
	evaluator := &TOTPEvaluator{loader: loader}
	for _, opt := range opts {
		opt(evaluator)
	}

	return evaluator
}

func (e *TOTPEvaluator) Evaluate(ctx context.Context, principal *Principal) (*OTPChallengeData, error) {
	secret, err := e.loader.LoadSecret(ctx, principal)
	if err != nil {
		return nil, err
	}

	if secret == "" {
		return nil, nil
	}

	destination := e.destination
	if destination == "" {
		destination = TOTPDefaultDestination
	}

	return &OTPChallengeData{Destination: destination}, nil
}

// TOTPVerifier validates TOTP codes using standard parameters (SHA1, 6 digits, 30s period, skew=1).
// It implements OTPCodeVerifier.
type TOTPVerifier struct {
	loader TOTPSecretLoader
}

// NewTOTPVerifier creates a TOTPVerifier with the given secret loader.
func NewTOTPVerifier(loader TOTPSecretLoader) *TOTPVerifier {
	return &TOTPVerifier{loader: loader}
}

func (v *TOTPVerifier) Verify(ctx context.Context, principal *Principal, code string) (bool, error) {
	secret, err := v.loader.LoadSecret(ctx, principal)
	if err != nil {
		return false, err
	}

	if secret == "" {
		return false, nil
	}

	return totp.Validate(code, secret), nil
}

// TOTPOption configures optional parameters for TOTPEvaluator.
type TOTPOption func(*TOTPEvaluator)

// WithTOTPDestination sets the destination description shown to the user for the TOTP challenge.
// Defaults to TOTPDefaultDestination ("Authenticator App").
func WithTOTPDestination(destination string) TOTPOption {
	return func(e *TOTPEvaluator) {
		e.destination = destination
	}
}

// NewTOTPChallengeProvider creates a TOTP challenge provider.
// Only TOTPSecretLoader needs to be implemented by the application.
// Default type "totp", order 100, destination "Authenticator App".
func NewTOTPChallengeProvider(loader TOTPSecretLoader, opts ...TOTPOption) *OTPChallengeProvider {
	return NewOTPChallengeProvider(OTPChallengeProviderConfig{
		ChallengeType:  ChallengeTypeTOTP,
		ChallengeOrder: 100,
		Evaluator:      NewTOTPEvaluator(loader, opts...),
		Verifier:       NewTOTPVerifier(loader),
	})
}
