package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/result"
)

// ─── Helpers ───

type MockTOTPSecretLoader struct {
	LoadSecretFn func(ctx context.Context, principal *Principal) (string, error)
}

func (m *MockTOTPSecretLoader) LoadSecret(ctx context.Context, principal *Principal) (string, error) {
	return m.LoadSecretFn(ctx, principal)
}

func generateTOTPKey(t *testing.T) *otp.Key {
	t.Helper()

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "VEF",
		AccountName: "test@example.com",
	})
	require.NoError(t, err, "Should generate TOTP key without error")

	return key
}

func generateValidCode(t *testing.T, secret string) string {
	t.Helper()

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err, "Should generate TOTP code without error")

	return code
}

// ─── TOTPEvaluator ───

func TestTOTPEvaluator(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("SecretExists", func(t *testing.T) {
		key := generateTOTPKey(t)
		evaluator := NewTOTPEvaluator(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil },
		})

		data, err := evaluator.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when secret exists")
		require.NotNil(t, data, "Should return challenge data when secret exists")
		assert.Equal(t, TOTPDefaultDestination, data.Destination, "Should use default destination")
	})

	t.Run("SecretEmpty", func(t *testing.T) {
		evaluator := NewTOTPEvaluator(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", nil },
		})

		data, err := evaluator.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when secret is empty")
		assert.Nil(t, data, "Should return nil when user has no TOTP configured")
	})

	t.Run("CustomDestination", func(t *testing.T) {
		key := generateTOTPKey(t)
		evaluator := NewTOTPEvaluator(
			&MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil }},
			WithTOTPDestination("Google Authenticator"),
		)

		data, err := evaluator.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error with custom destination")
		require.NotNil(t, data, "Should return challenge data with custom destination")
		assert.Equal(t, "Google Authenticator", data.Destination, "Should use custom destination")
	})

	t.Run("LoaderError", func(t *testing.T) {
		loadErr := errors.New("load failed")
		evaluator := NewTOTPEvaluator(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", loadErr },
		})

		data, err := evaluator.Evaluate(ctx, principal)

		require.ErrorIs(t, err, loadErr, "Should propagate loader error")
		assert.Nil(t, data, "Should return nil data on loader error")
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		key := generateTOTPKey(t)
		firstDest := WithTOTPDestination("First")
		secondDest := WithTOTPDestination("Second")
		eval := NewTOTPEvaluator(
			&MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil }},
			firstDest, secondDest,
		)

		data, err := eval.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error with multiple options")
		require.NotNil(t, data, "Should return challenge data with multiple options")
		assert.Equal(t, "Second", data.Destination, "Last option should win when multiple options are applied")
	})
}

// ─── TOTPVerifier ───

func TestTOTPVerifier(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("ValidCode", func(t *testing.T) {
		key := generateTOTPKey(t)
		code := generateValidCode(t, key.Secret())
		verifier := NewTOTPVerifier(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil },
		})

		valid, err := verifier.Verify(ctx, principal, code)

		require.NoError(t, err, "Should not return error for valid code")
		assert.True(t, valid, "Should return true for valid TOTP code")
	})

	t.Run("InvalidCode", func(t *testing.T) {
		key := generateTOTPKey(t)
		verifier := NewTOTPVerifier(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil },
		})

		valid, err := verifier.Verify(ctx, principal, "000000")

		require.NoError(t, err, "Should not return error for invalid code")
		assert.False(t, valid, "Should return false for invalid TOTP code")
	})

	t.Run("EmptySecret", func(t *testing.T) {
		verifier := NewTOTPVerifier(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", nil },
		})

		valid, err := verifier.Verify(ctx, principal, "123456")

		require.NoError(t, err, "Should not return error when secret is empty")
		assert.False(t, valid, "Should return false when user has no TOTP configured")
	})

	t.Run("LoaderError", func(t *testing.T) {
		loadErr := errors.New("load failed")
		verifier := NewTOTPVerifier(&MockTOTPSecretLoader{
			LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", loadErr },
		})

		valid, err := verifier.Verify(ctx, principal, "123456")

		require.ErrorIs(t, err, loadErr, "Should propagate loader error")
		assert.False(t, valid, "Should return false when loader returns error")
	})
}

// ─── NewTOTPChallengeProvider ───

func TestNewTOTPChallengeProvider(t *testing.T) {
	loader := &MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", nil }}

	t.Run("TypeAndOrder", func(t *testing.T) {
		provider := NewTOTPChallengeProvider(loader)

		assert.Equal(t, ChallengeTypeTOTP, provider.Type(), "Should use TOTP challenge type")
		assert.Equal(t, 100, provider.Order(), "Should use default TOTP order 100")
	})

	t.Run("WithDestinationOption", func(t *testing.T) {
		key := generateTOTPKey(t)
		secretLoader := &MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil }}
		provider := NewTOTPChallengeProvider(secretLoader, WithTOTPDestination("1Password"))

		challenge, err := provider.Evaluate(context.Background(), NewUser("u1", "Alice"))

		require.NoError(t, err, "Should not return error with destination option")
		require.NotNil(t, challenge, "Should return challenge when secret exists")
		data, ok := challenge.Data.(*OTPChallengeData)
		require.True(t, ok, "Challenge data should be *OTPChallengeData")
		assert.Equal(t, "1Password", data.Destination, "Should use custom destination from option")
	})

	t.Run("NoSecretSkipsChallenge", func(t *testing.T) {
		secretLoader := &MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return "", nil }}
		provider := NewTOTPChallengeProvider(secretLoader)

		challenge, err := provider.Evaluate(context.Background(), NewUser("u1", "Alice"))

		require.NoError(t, err, "Should not return error when user has no TOTP secret")
		assert.Nil(t, challenge, "Should return nil challenge when user has no TOTP configured")
	})

	t.Run("Integration", func(t *testing.T) {
		key := generateTOTPKey(t)
		secretLoader := &MockTOTPSecretLoader{LoadSecretFn: func(context.Context, *Principal) (string, error) { return key.Secret(), nil }}
		provider := NewTOTPChallengeProvider(secretLoader)
		ctx := context.Background()
		principal := NewUser("u1", "Alice")

		// Evaluate should return a challenge
		challenge, err := provider.Evaluate(ctx, principal)
		require.NoError(t, err, "Should evaluate without error")
		require.NotNil(t, challenge, "Should return a challenge for user with TOTP")
		assert.Equal(t, ChallengeTypeTOTP, challenge.Type, "Challenge type should be TOTP")
		assert.True(t, challenge.Required, "Challenge should be required")

		// Resolve with a valid code should succeed
		code := generateValidCode(t, key.Secret())
		resolved, err := provider.Resolve(ctx, principal, code)
		require.NoError(t, err, "Should resolve with valid code")
		assert.Same(t, principal, resolved, "Should return same principal on success")

		// Resolve with an invalid code should fail
		_, err = provider.Resolve(ctx, principal, "000000")
		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for invalid code")
		assert.Equal(t, result.ErrCodeOTPCodeInvalid, resErr.Code, "Should return OTP code invalid error")
	})
}

// ─── Interface compliance ───

func TestTOTPInterfaceCompliance(t *testing.T) {
	t.Run("TOTPEvaluatorImplementsOTPEvaluator", func(*testing.T) {
		var _ OTPEvaluator = (*TOTPEvaluator)(nil)
	})

	t.Run("TOTPVerifierImplementsOTPCodeVerifier", func(*testing.T) {
		var _ OTPCodeVerifier = (*TOTPVerifier)(nil)
	})
}
