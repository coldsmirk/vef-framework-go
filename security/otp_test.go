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

type MockOTPEvaluator struct {
	EvaluateFn func(ctx context.Context, principal *Principal) (*OTPChallengeData, error)
}

func (m *MockOTPEvaluator) Evaluate(ctx context.Context, principal *Principal) (*OTPChallengeData, error) {
	return m.EvaluateFn(ctx, principal)
}

type MockOTPCodeSender struct {
	SendFn func(ctx context.Context, principal *Principal) error
}

func (m *MockOTPCodeSender) Send(ctx context.Context, principal *Principal) error {
	return m.SendFn(ctx, principal)
}

type MockOTPCodeVerifier struct {
	VerifyFn func(ctx context.Context, principal *Principal, code string) (bool, error)
}

func (m *MockOTPCodeVerifier) Verify(ctx context.Context, principal *Principal, code string) (bool, error) {
	return m.VerifyFn(ctx, principal, code)
}

type MockOTPCodeStore struct {
	GenerateFn func(ctx context.Context, principal *Principal) (string, error)
	VerifyFn   func(ctx context.Context, principal *Principal, code string) (bool, error)
}

func (m *MockOTPCodeStore) Generate(ctx context.Context, principal *Principal) (string, error) {
	return m.GenerateFn(ctx, principal)
}

func (m *MockOTPCodeStore) Verify(ctx context.Context, principal *Principal, code string) (bool, error) {
	return m.VerifyFn(ctx, principal, code)
}

type MockOTPCodeDelivery struct {
	DeliverFn func(ctx context.Context, principal *Principal, code string) error
}

func (m *MockOTPCodeDelivery) Deliver(ctx context.Context, principal *Principal, code string) error {
	return m.DeliverFn(ctx, principal, code)
}

// ─── NewOTPChallengeProvider validation ───

func TestNewOTPChallengeProvider(t *testing.T) {
	validEvaluator := &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }}
	validVerifier := &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }}

	t.Run("MissingType", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: OTPChallengeProviderConfig.ChallengeType is required", func() {
			NewOTPChallengeProvider(OTPChallengeProviderConfig{
				Evaluator: validEvaluator,
				Verifier:  validVerifier,
			})
		}, "Should panic when ChallengeType is empty")
	})

	t.Run("MissingEvaluator", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: OTPChallengeProviderConfig.Evaluator is required", func() {
			NewOTPChallengeProvider(OTPChallengeProviderConfig{
				ChallengeType: "test",
				Verifier:      validVerifier,
			})
		}, "Should panic when Evaluator is nil")
	})

	t.Run("MissingVerifier", func(t *testing.T) {
		assert.PanicsWithValue(t, "security: OTPChallengeProviderConfig.Verifier is required", func() {
			NewOTPChallengeProvider(OTPChallengeProviderConfig{
				ChallengeType: "test",
				Evaluator:     validEvaluator,
			})
		}, "Should panic when Verifier is nil")
	})

	t.Run("ValidConfig", func(t *testing.T) {
		assert.NotPanics(t, func() {
			NewOTPChallengeProvider(OTPChallengeProviderConfig{
				ChallengeType: "test",
				Evaluator:     validEvaluator,
				Verifier:      validVerifier,
			})
		}, "Should not panic with valid config")
	})
}

// ─── Type and Order ───

func TestOTPChallengeProviderTypeAndOrder(t *testing.T) {
	provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
		ChallengeType:  "custom_otp",
		ChallengeOrder: 42,
		Evaluator:      &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }},
		Verifier:       &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
	})

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, "custom_otp", provider.Type(), "Should return configured challenge type")
	})

	t.Run("Order", func(t *testing.T) {
		assert.Equal(t, 42, provider.Order(), "Should return configured challenge order")
	})

	t.Run("DefaultOrder", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }},
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})
		assert.Equal(t, 0, provider.Order(), "Should return zero when ChallengeOrder is not set")
	})
}

// ─── Evaluate ───

func TestOTPChallengeProviderEvaluate(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("ChallengeNotNeeded", func(t *testing.T) {
		senderCalled := false
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }},
			Sender: &MockOTPCodeSender{SendFn: func(context.Context, *Principal) error {
				senderCalled = true

				return nil
			}},
			Verifier: &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when challenge is not needed")
		assert.Nil(t, challenge, "Should return nil challenge when evaluator returns nil")
		assert.False(t, senderCalled, "Sender should not be called when challenge is not needed")
	})

	t.Run("ChallengeNeededNilSender", func(t *testing.T) {
		data := &OTPChallengeData{Destination: "Authenticator App"}
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "totp",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return data, nil }},
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error with nil sender")
		require.NotNil(t, challenge, "Should return challenge when evaluator returns data")
		assert.Equal(t, "totp", challenge.Type, "Challenge type should match config")
		assert.Equal(t, data, challenge.Data, "Challenge data should match evaluator output")
		assert.True(t, challenge.Required, "Challenge should be marked as required")
	})

	t.Run("WithSender", func(t *testing.T) {
		senderCalled := false
		data := &OTPChallengeData{Destination: "****1234"}
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "sms_otp",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return data, nil }},
			Sender: &MockOTPCodeSender{SendFn: func(context.Context, *Principal) error {
				senderCalled = true

				return nil
			}},
			Verifier: &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		challenge, err := provider.Evaluate(ctx, principal)

		require.NoError(t, err, "Should not return error when sender succeeds")
		require.NotNil(t, challenge, "Should return challenge when sender succeeds")
		assert.True(t, senderCalled, "Sender should be called when challenge is needed")
	})

	t.Run("SenderError", func(t *testing.T) {
		sendErr := errors.New("send failed")
		data := &OTPChallengeData{Destination: "****1234"}
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "sms_otp",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return data, nil }},
			Sender:        &MockOTPCodeSender{SendFn: func(context.Context, *Principal) error { return sendErr }},
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, sendErr, "Should propagate sender error")
		assert.Nil(t, challenge, "Should return nil challenge on sender error")
	})

	t.Run("EvaluatorError", func(t *testing.T) {
		evalErr := errors.New("evaluate failed")
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, evalErr }},
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		challenge, err := provider.Evaluate(ctx, principal)

		require.ErrorIs(t, err, evalErr, "Should propagate evaluator error")
		assert.Nil(t, challenge, "Should return nil challenge on evaluator error")
	})
}

// ─── Resolve ───

func TestOTPChallengeProviderResolve(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	noopEvaluator := &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }}

	t.Run("ValidCode", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		resolved, err := provider.Resolve(ctx, principal, "123456")

		require.NoError(t, err, "Should not return error for valid code")
		assert.Same(t, principal, resolved, "Should return the same principal on success")
	})

	t.Run("InvalidCode", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return false, nil }},
		})

		_, err := provider.Resolve(ctx, principal, "wrong")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error")
		assert.Equal(t, result.ErrCodeOTPCodeInvalid, resErr.Code, "Should return OTP code invalid error")
	})

	t.Run("ResponseNotString", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		_, err := provider.Resolve(ctx, principal, 12345)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for non-string response")
		assert.Equal(t, result.ErrCodeOTPCodeRequired, resErr.Code, "Should return OTP code required error")
	})

	t.Run("ResponseNil", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		_, err := provider.Resolve(ctx, principal, nil)

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for nil response")
		assert.Equal(t, result.ErrCodeOTPCodeRequired, resErr.Code, "Should return OTP code required error")
	})

	t.Run("ResponseEmpty", func(t *testing.T) {
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil }},
		})

		_, err := provider.Resolve(ctx, principal, "")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "Should return a result.Error for empty response")
		assert.Equal(t, result.ErrCodeOTPCodeRequired, resErr.Code, "Should return OTP code required error")
	})

	t.Run("VerifierError", func(t *testing.T) {
		verifyErr := errors.New("verify failed")
		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier:      &MockOTPCodeVerifier{VerifyFn: func(context.Context, *Principal, string) (bool, error) { return false, verifyErr }},
		})

		_, err := provider.Resolve(ctx, principal, "123456")

		require.ErrorIs(t, err, verifyErr, "Should propagate verifier error")
	})

	t.Run("WhitespaceOnlyCode", func(t *testing.T) {
		var receivedCode string

		provider := NewOTPChallengeProvider(OTPChallengeProviderConfig{
			ChallengeType: "test",
			Evaluator:     noopEvaluator,
			Verifier: &MockOTPCodeVerifier{VerifyFn: func(_ context.Context, _ *Principal, code string) (bool, error) {
				receivedCode = code

				return true, nil
			}},
		})

		resolved, err := provider.Resolve(ctx, principal, "  ")

		require.NoError(t, err, "Should not return error for whitespace-only code")
		assert.Same(t, principal, resolved, "Should return principal when verifier accepts whitespace code")
		assert.Equal(t, "  ", receivedCode, "Should pass whitespace code as-is to verifier")
	})
}

// ─── DeliveredCodeSender ───

func TestDeliveredCodeSender(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("Success", func(t *testing.T) {
		var deliveredCode string

		store := &MockOTPCodeStore{
			GenerateFn: func(context.Context, *Principal) (string, error) { return "654321", nil },
		}
		delivery := &MockOTPCodeDelivery{
			DeliverFn: func(_ context.Context, _ *Principal, code string) error {
				deliveredCode = code

				return nil
			},
		}
		sender := NewDeliveredCodeSender(store, delivery)

		err := sender.Send(ctx, principal)

		require.NoError(t, err, "Should not return error on successful send")
		assert.Equal(t, "654321", deliveredCode, "Should deliver the generated code")
	})

	t.Run("GenerateError", func(t *testing.T) {
		genErr := errors.New("generate failed")
		deliverCalled := false
		store := &MockOTPCodeStore{
			GenerateFn: func(context.Context, *Principal) (string, error) { return "", genErr },
		}
		delivery := &MockOTPCodeDelivery{
			DeliverFn: func(context.Context, *Principal, string) error {
				deliverCalled = true

				return nil
			},
		}
		sender := NewDeliveredCodeSender(store, delivery)

		err := sender.Send(ctx, principal)

		require.ErrorIs(t, err, genErr, "Should propagate store generate error")
		assert.False(t, deliverCalled, "Deliver should not be called when Generate fails")
	})

	t.Run("DeliverError", func(t *testing.T) {
		deliverErr := errors.New("deliver failed")
		store := &MockOTPCodeStore{
			GenerateFn: func(context.Context, *Principal) (string, error) { return "654321", nil },
		}
		delivery := &MockOTPCodeDelivery{
			DeliverFn: func(context.Context, *Principal, string) error { return deliverErr },
		}
		sender := NewDeliveredCodeSender(store, delivery)

		err := sender.Send(ctx, principal)

		require.ErrorIs(t, err, deliverErr, "Should propagate delivery error")
	})

	t.Run("PrincipalPassthrough", func(t *testing.T) {
		var storePrincipal, deliveryPrincipal *Principal

		store := &MockOTPCodeStore{
			GenerateFn: func(_ context.Context, p *Principal) (string, error) {
				storePrincipal = p

				return "111111", nil
			},
		}
		delivery := &MockOTPCodeDelivery{
			DeliverFn: func(_ context.Context, p *Principal, _ string) error {
				deliveryPrincipal = p

				return nil
			},
		}
		sender := NewDeliveredCodeSender(store, delivery)

		err := sender.Send(ctx, principal)

		require.NoError(t, err, "Should not return error on successful send")
		assert.Same(t, principal, storePrincipal, "Should pass the same principal to store")
		assert.Same(t, principal, deliveryPrincipal, "Should pass the same principal to delivery")
	})
}

// ─── DeliveredCodeVerifier ───

func TestDeliveredCodeVerifier(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("Valid", func(t *testing.T) {
		store := &MockOTPCodeStore{
			VerifyFn: func(context.Context, *Principal, string) (bool, error) { return true, nil },
		}
		verifier := NewDeliveredCodeVerifier(store)

		valid, err := verifier.Verify(ctx, principal, "123456")

		require.NoError(t, err, "Should not return error for valid code")
		assert.True(t, valid, "Should return true for valid code")
	})

	t.Run("Invalid", func(t *testing.T) {
		store := &MockOTPCodeStore{
			VerifyFn: func(context.Context, *Principal, string) (bool, error) { return false, nil },
		}
		verifier := NewDeliveredCodeVerifier(store)

		valid, err := verifier.Verify(ctx, principal, "wrong")

		require.NoError(t, err, "Should not return error for invalid code")
		assert.False(t, valid, "Should return false for invalid code")
	})

	t.Run("StoreError", func(t *testing.T) {
		storeErr := errors.New("store error")
		store := &MockOTPCodeStore{
			VerifyFn: func(context.Context, *Principal, string) (bool, error) { return false, storeErr },
		}
		verifier := NewDeliveredCodeVerifier(store)

		_, err := verifier.Verify(ctx, principal, "123456")

		require.ErrorIs(t, err, storeErr, "Should propagate store error")
	})
}

// ─── Convenience constructors ───

func TestNewSMSChallengeProvider(t *testing.T) {
	evaluator := &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }}
	store := &MockOTPCodeStore{}
	delivery := &MockOTPCodeDelivery{}

	provider := NewSMSChallengeProvider(evaluator, store, delivery)

	assert.Equal(t, ChallengeTypeSMS, provider.Type(), "Should use SMS challenge type")
	assert.Equal(t, 200, provider.Order(), "Should use default SMS order 200")
}

func TestNewEmailChallengeProvider(t *testing.T) {
	evaluator := &MockOTPEvaluator{EvaluateFn: func(context.Context, *Principal) (*OTPChallengeData, error) { return nil, nil }}
	store := &MockOTPCodeStore{}
	delivery := &MockOTPCodeDelivery{}

	provider := NewEmailChallengeProvider(evaluator, store, delivery)

	assert.Equal(t, ChallengeTypeEmail, provider.Type(), "Should use Email challenge type")
	assert.Equal(t, 300, provider.Order(), "Should use default Email order 300")
}

// ─── Interface compliance ───

func TestOTPInterfaceCompliance(t *testing.T) {
	t.Run("OTPChallengeProviderImplementsChallengeProvider", func(*testing.T) {
		var _ ChallengeProvider = (*OTPChallengeProvider)(nil)
	})

	t.Run("DeliveredCodeSenderImplementsOTPCodeSender", func(*testing.T) {
		var _ OTPCodeSender = (*DeliveredCodeSender)(nil)
	})

	t.Run("DeliveredCodeVerifierImplementsOTPCodeVerifier", func(*testing.T) {
		var _ OTPCodeVerifier = (*DeliveredCodeVerifier)(nil)
	})
}
