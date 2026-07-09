package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

// ─── Mock implementations ───

type MockPasswordChangeChecker struct {
	CheckFn func(ctx context.Context, principal *Principal) (*PasswordChangeChallengeData, error)
}

func (m *MockPasswordChangeChecker) Check(ctx context.Context, principal *Principal) (*PasswordChangeChallengeData, error) {
	return m.CheckFn(ctx, principal)
}

type MockPasswordChanger struct {
	ChangePasswordFn func(ctx context.Context, principal *Principal, newPassword string) error
}

func (m *MockPasswordChanger) ChangePassword(ctx context.Context, principal *Principal, newPassword string) error {
	return m.ChangePasswordFn(ctx, principal, newPassword)
}

// newTestProvider builds a provider without strength validation, the common case
// for the checker/changer/resolve tests below.
func newTestProvider(checker PasswordChangeChecker, changer PasswordChanger) *PasswordChangeChallengeProvider {
	return NewPasswordChangeChallengeProvider(checker, changer, nil)
}

// ─── Constructor validation ───

func TestNewPasswordChangeChallengeProvider(t *testing.T) {
	validChecker := &MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }}
	validChanger := &MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }}

	t.Run("MissingChecker", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: PasswordChangeChecker is required", func() {
			NewPasswordChangeChallengeProvider(nil, validChanger, nil)
		}, "Should panic when checker is nil")
	})

	t.Run("MissingChanger", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: PasswordChanger is required", func() {
			NewPasswordChangeChallengeProvider(validChecker, nil, nil)
		}, "Should panic when changer is nil")
	})

	t.Run("ValidConfig", func(t *testing.T) {
		assert.NotPanics(t, func() {
			NewPasswordChangeChallengeProvider(validChecker, validChanger, nil)
		}, "Should not panic with valid config")
	})
}

// ─── Type and Order ───

func TestPasswordChangeChallengeProviderTypeAndOrder(t *testing.T) {
	provider := newTestProvider(
		&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }},
		&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
	)

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, "password_change", provider.Type(), "Should return password_change type")
	})

	t.Run("Order", func(t *testing.T) {
		assert.Equal(t, 400, provider.Order(), "Should return default order 400")
	})
}

// ─── Evaluate ───

func TestPasswordChangeChallengeProviderEvaluate(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("ChangeNotRequired", func(t *testing.T) {
		changerCalled := false
		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }},
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error {
				changerCalled = true

				return nil
			}},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when change is not required")
		assert.Nil(t, challenge, "Should return nil challenge when checker returns nil")
		assert.False(t, changerCalled, "Changer should not be called during evaluate")
	})

	t.Run("ChangeRequired", func(t *testing.T) {
		data := &PasswordChangeChallengeData{Reason: PasswordChangeReasonFirstLogin}
		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return data, nil }},
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when change is required")
		require.NotNil(t, challenge, "Should return challenge when change is required")
		assert.Equal(t, ChallengeTypePasswordChange, challenge.Type, "Challenge type should be password_change")
		assert.Equal(t, data, challenge.Data, "Challenge data should match checker output")
		assert.True(t, challenge.Required, "Challenge should be marked as required")
	})

	t.Run("CheckerError", func(t *testing.T) {
		checkErr := errors.New("check failed")
		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, checkErr }},
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, checkErr, "Should propagate checker error")
		assert.Nil(t, challenge, "Should return nil challenge on checker error")
	})

	t.Run("CheckerReturnsDataAndError", func(t *testing.T) {
		checkErr := errors.New("partial failure")
		data := &PasswordChangeChallengeData{Reason: PasswordChangeReasonExpired}
		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return data, checkErr }},
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, checkErr, "Should propagate error even when data is non-nil")
		assert.Nil(t, challenge, "Should discard data when error is present")
	})

	t.Run("ChallengeDataWithMeta", func(t *testing.T) {
		meta := map[string]any{"daysUntilExpiry": 0, "policy": "90-day"}
		data := &PasswordChangeChallengeData{Reason: PasswordChangeReasonExpired, Meta: meta}
		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return data, nil }},
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Challenge data with meta should evaluate without error")
		require.NotNil(t, challenge, "Should return challenge")
		assert.Same(t, data, challenge.Data, "Should pass through data with Meta intact")
	})
}

// ─── Resolve ───

func TestPasswordChangeChallengeProviderResolve(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	noopChecker := &MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }}

	t.Run("ValidPassword", func(t *testing.T) {
		var receivedPassword string

		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(_ context.Context, _ *Principal, pw string) error {
				receivedPassword = pw

				return nil
			}},
		)

		resolved, err := provider.Resolve(ctx, principal, "newPass123")

		require.NoError(t, err, "Should not return error for valid password")
		assert.Same(t, principal, resolved, "Should return the same principal on success")
		assert.Equal(t, "newPass123", receivedPassword, "Should pass password to changer")
	})

	t.Run("ResponseNotString", func(t *testing.T) {
		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		_, err := provider.Resolve(ctx, principal, 12345)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for non-string response")
		assert.Equal(t, ErrCodeNewPasswordRequired, resErr.Code, "Should return new password required error")
	})

	t.Run("ResponseNil", func(t *testing.T) {
		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		_, err := provider.Resolve(ctx, principal, nil)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for nil response")
		assert.Equal(t, ErrCodeNewPasswordRequired, resErr.Code, "Should return new password required error")
	})

	t.Run("ResponseEmpty", func(t *testing.T) {
		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return nil }},
		)

		_, err := provider.Resolve(ctx, principal, "")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for empty response")
		assert.Equal(t, ErrCodeNewPasswordRequired, resErr.Code, "Should return new password required error")
	})

	t.Run("WhitespacePassword", func(t *testing.T) {
		var receivedPassword string

		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(_ context.Context, _ *Principal, pw string) error {
				receivedPassword = pw

				return nil
			}},
		)

		resolved, err := provider.Resolve(ctx, principal, "  ")

		require.NoError(t, err, "Should not return error for whitespace-only password")
		assert.Same(t, principal, resolved, "Should return principal when changer succeeds")
		assert.Equal(t, "  ", receivedPassword, "Should pass whitespace password as-is to changer")
	})

	t.Run("ChangerError", func(t *testing.T) {
		changeErr := errors.New("password too weak")
		provider := newTestProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error { return changeErr }},
		)

		_, err := provider.Resolve(ctx, principal, "weak")

		require.ErrorIs(t, err, changeErr, "Should propagate changer error")
	})

	t.Run("PrincipalPassthrough", func(t *testing.T) {
		var checkerPrincipal, changerPrincipal *Principal

		provider := newTestProvider(
			&MockPasswordChangeChecker{CheckFn: func(_ context.Context, p *Principal) (*PasswordChangeChallengeData, error) {
				checkerPrincipal = p

				return &PasswordChangeChallengeData{Reason: PasswordChangeReasonExpired}, nil
			}},
			&MockPasswordChanger{ChangePasswordFn: func(_ context.Context, p *Principal, _ string) error {
				changerPrincipal = p

				return nil
			}},
		)

		_, _ = provider.Evaluate(ctx, principal)
		_, _ = provider.Resolve(ctx, principal, "newPass")

		assert.Same(t, principal, checkerPrincipal, "Should pass the same principal to checker")
		assert.Same(t, principal, changerPrincipal, "Should pass the same principal to changer")
	})
}

// ─── Composite checker ───

func TestCompositePasswordChangeChecker(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	firstLogin := &PasswordChangeChallengeData{Reason: PasswordChangeReasonFirstLogin}
	expired := &PasswordChangeChallengeData{Reason: PasswordChangeReasonExpired}

	noChange := func() PasswordChangeChecker {
		return &MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }}
	}
	requires := func(data *PasswordChangeChallengeData) PasswordChangeChecker {
		return &MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return data, nil }}
	}

	t.Run("ReturnsFirstRequiringChecker", func(t *testing.T) {
		checker := NewCompositePasswordChangeChecker(noChange(), requires(firstLogin), requires(expired))

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "composite should not error")
		assert.Same(t, firstLogin, data, "the first requiring checker should win")
	})

	t.Run("SkipsNilCheckers", func(t *testing.T) {
		checker := NewCompositePasswordChangeChecker(nil, requires(expired))

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "nil checkers should be skipped without error")
		assert.Same(t, expired, data, "the first non-nil requiring checker should win")
	})

	t.Run("PropagatesError", func(t *testing.T) {
		checkErr := errors.New("check failed")
		checker := NewCompositePasswordChangeChecker(
			&MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, checkErr }},
			requires(expired),
		)

		_, err := checker.Check(ctx, principal)

		require.ErrorIs(t, err, checkErr, "a checker error should short-circuit and propagate")
	})

	t.Run("NoneRequireChange", func(t *testing.T) {
		checker := NewCompositePasswordChangeChecker(noChange(), noChange())

		data, err := checker.Check(ctx, principal)

		require.NoError(t, err, "composite should not error when no checker requires a change")
		assert.Nil(t, data, "no change is required when every checker passes")
	})
}

// ─── Strength validation ───

func TestPasswordChangeChallengeProviderValidatesStrength(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	noopChecker := &MockPasswordChangeChecker{CheckFn: func(context.Context, *Principal) (*PasswordChangeChallengeData, error) { return nil, nil }}
	validator := NewRuleBasedValidator(NewMinLengthRule(8))

	t.Run("RejectsWeakPasswordBeforeChanger", func(t *testing.T) {
		changerCalled := false
		provider := NewPasswordChangeChallengeProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(context.Context, *Principal, string) error {
				changerCalled = true

				return nil
			}},
			validator,
		)

		_, err := provider.Resolve(ctx, principal, "short")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for a weak password")
		assert.Equal(t, ErrCodePasswordPolicyViolation, resErr.Code, "Should return the password-policy violation code")
		assert.False(t, changerCalled, "Changer must not run when validation fails")
	})

	t.Run("AcceptsCompliantPassword", func(t *testing.T) {
		var received string

		provider := NewPasswordChangeChallengeProvider(
			noopChecker,
			&MockPasswordChanger{ChangePasswordFn: func(_ context.Context, _ *Principal, pw string) error {
				received = pw

				return nil
			}},
			validator,
		)

		resolved, err := provider.Resolve(ctx, principal, "longenough")

		require.NoError(t, err, "Should accept a compliant password")
		assert.Same(t, principal, resolved, "Should return the principal on success")
		assert.Equal(t, "longenough", received, "Should pass the validated password to the changer")
	})
}

// ─── Interface compliance ───

func TestPasswordChangeInterfaceCompliance(t *testing.T) {
	t.Run("ImplementsChallengeProvider", func(*testing.T) {
		var _ ChallengeProvider = (*PasswordChangeChallengeProvider)(nil)
	})
}
