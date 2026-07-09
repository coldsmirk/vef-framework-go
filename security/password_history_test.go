package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/result"
)

type stubHistoryStore struct {
	recent   []string
	err      error
	gotID    string
	gotLimit int
	called   bool
}

func (s *stubHistoryStore) Recent(_ context.Context, principalID string, limit int) ([]string, error) {
	s.called = true
	s.gotID = principalID
	s.gotLimit = limit

	return s.recent, s.err
}

func (*stubHistoryStore) Add(context.Context, string, string) error { return nil }

func TestHistoryValidator(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")
	// The plaintext encoder makes Matches a plain string comparison, so stored
	// "hashes" are the plaintext passwords — fast and deterministic for the test.
	encoder := password.NewPlaintextEncoder()

	t.Run("RejectsReusedPassword", func(t *testing.T) {
		store := &stubHistoryStore{recent: []string{"oldpass", "olderpass"}}
		validator := NewHistoryValidator(store, encoder, 5)

		err := validator.Validate(ctx, principal, "oldpass")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "a reused password should be rejected with a result.Error")
		assert.Equal(t, ErrCodePasswordPolicyViolation, resErr.Code, "reuse should carry the password-policy code")
		assert.Equal(t, "u1", store.gotID, "the check should query history for the principal's id")
		assert.Equal(t, 5, store.gotLimit, "the check should bound the query to the configured depth")
	})

	t.Run("AllowsNewPassword", func(t *testing.T) {
		store := &stubHistoryStore{recent: []string{"oldpass"}}
		validator := NewHistoryValidator(store, encoder, 5)

		assert.NoError(t, validator.Validate(ctx, principal, "brandnew"), "a never-used password should pass")
	})

	t.Run("DisabledWhenDepthNonPositive", func(t *testing.T) {
		store := &stubHistoryStore{recent: []string{"oldpass"}}
		validator := NewHistoryValidator(store, encoder, 0)

		assert.NoError(t, validator.Validate(ctx, principal, "oldpass"), "depth 0 should disable the check")
		assert.False(t, store.called, "a disabled check should not query the store")
	})

	t.Run("NilPrincipalPasses", func(t *testing.T) {
		store := &stubHistoryStore{recent: []string{"oldpass"}}
		validator := NewHistoryValidator(store, encoder, 5)

		assert.NoError(t, validator.Validate(ctx, nil, "oldpass"), "a nil principal should impose no history check")
		assert.False(t, store.called, "a nil principal should not query the store")
	})

	t.Run("StoreErrorPropagates", func(t *testing.T) {
		store := &stubHistoryStore{err: errors.New("db down")}
		validator := NewHistoryValidator(store, encoder, 5)

		require.ErrorIs(t, validator.Validate(ctx, principal, "x"), store.err, "a store error should propagate")
	})
}

func TestChainValidator(t *testing.T) {
	ctx := context.Background()
	principal := NewUser("u1", "Alice")

	t.Run("ReturnsFirstViolation", func(t *testing.T) {
		chain := NewChainValidator(
			NewRuleBasedValidator(NewMinLengthRule(8)),
			NewRuleBasedValidator(NewCharacterClassRule(true, false, false, false, 0)),
		)

		err := chain.Validate(ctx, principal, "short")
		resErr, ok := result.AsErr(err)
		require.True(t, ok, "the first failing validator should report a result.Error")
		assert.Equal(t, ErrPasswordTooShort(8).Message, resErr.Message, "the length validator should run before the class validator")
	})

	t.Run("SkipsNilValidators", func(t *testing.T) {
		chain := NewChainValidator(nil, NewRuleBasedValidator(NewMinLengthRule(4)))

		assert.NoError(t, chain.Validate(ctx, principal, "longenough"), "nil validators should be skipped")
	})

	t.Run("PassesWhenAllPass", func(t *testing.T) {
		chain := NewChainValidator(
			NewRuleBasedValidator(NewMinLengthRule(4)),
			NewRuleBasedValidator(NewMaxLengthRule(20)),
		)

		assert.NoError(t, chain.Validate(ctx, principal, "justright"), "a password satisfying every validator should pass")
	})
}
