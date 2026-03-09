package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/result"
)

// ChallengeTypePasswordChange is the challenge type identifier for forced password change.
const ChallengeTypePasswordChange = "password_change"

// Predefined password change reason constants for use in PasswordChangeChecker implementations.
const (
	PasswordChangeReasonFirstLogin = "first_login"
	PasswordChangeReasonExpired    = "expired"
)

// PasswordChangeChallengeData describes the metadata for a forced password change challenge.
type PasswordChangeChallengeData struct {
	Reason string         `json:"reason"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// PasswordChangeChecker determines whether a user must change their password.
type PasswordChangeChecker interface {
	// Check returns challenge data (including reason) if a password change is required,
	// or nil if no change is needed.
	Check(ctx context.Context, principal *Principal) (*PasswordChangeChallengeData, error)
}

// PasswordChanger validates and persists a new password.
type PasswordChanger interface {
	// ChangePassword validates password strength and persists the new password.
	ChangePassword(ctx context.Context, principal *Principal, newPassword string) error
}

// PasswordChangeChallengeProvider orchestrates forced password change evaluation and resolution.
// It implements the ChallengeProvider interface.
type PasswordChangeChallengeProvider struct {
	checker PasswordChangeChecker
	changer PasswordChanger
}

// NewPasswordChangeChallengeProvider creates a forced password change challenge provider.
// Default type "password_change", order 400.
// Panics if checker or changer is nil.
func NewPasswordChangeChallengeProvider(checker PasswordChangeChecker, changer PasswordChanger) *PasswordChangeChallengeProvider {
	if checker == nil {
		panic("security: PasswordChangeChecker is required")
	}

	if changer == nil {
		panic("security: PasswordChanger is required")
	}

	return &PasswordChangeChallengeProvider{checker: checker, changer: changer}
}

func (*PasswordChangeChallengeProvider) Type() string { return ChallengeTypePasswordChange }
func (*PasswordChangeChallengeProvider) Order() int   { return 400 }

func (p *PasswordChangeChallengeProvider) Evaluate(ctx context.Context, principal *Principal) (*LoginChallenge, error) {
	data, err := p.checker.Check(ctx, principal)
	if err != nil || data == nil {
		return nil, err
	}

	return &LoginChallenge{
		Type:     ChallengeTypePasswordChange,
		Data:     data,
		Required: true,
	}, nil
}

func (p *PasswordChangeChallengeProvider) Resolve(ctx context.Context, principal *Principal, response any) (*Principal, error) {
	newPassword, ok := response.(string)
	if !ok || newPassword == "" {
		return nil, result.ErrNewPasswordRequired
	}

	if err := p.changer.ChangePassword(ctx, principal, newPassword); err != nil {
		return nil, err
	}

	return principal, nil
}
