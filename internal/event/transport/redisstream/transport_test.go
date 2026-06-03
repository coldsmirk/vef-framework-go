package redisstream

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBusyGroup(t *testing.T) {
	t.Run("nil error returns false", func(t *testing.T) {
		assert.False(t, isBusyGroup(nil), "nil error must not be treated as BUSYGROUP")
	})

	t.Run("error prefixed with BUSYGROUP returns true", func(t *testing.T) {
		err := errors.New("BUSYGROUP Consumer Group name already exists")
		assert.True(t, isBusyGroup(err), "error starting with BUSYGROUP must be detected")
	})

	t.Run("error containing BUSYGROUP mid-message returns false", func(t *testing.T) {
		err := errors.New("ERR something BUSYGROUP something")
		assert.False(t, isBusyGroup(err), "BUSYGROUP not at start must not match")
	})

	t.Run("unrelated error returns false", func(t *testing.T) {
		err := errors.New("ERR no such key")
		assert.False(t, isBusyGroup(err), "unrelated error must return false")
	})
}

func TestValidateEventType(t *testing.T) {
	t.Run("valid type passes", func(t *testing.T) {
		require.NoError(t, validateEventType("order.created"), "alphanumeric dot type must be valid")
		require.NoError(t, validateEventType("user_signup"), "underscore type must be valid")
		require.NoError(t, validateEventType("payment-failed"), "hyphen type must be valid")
		require.NoError(t, validateEventType("A"), "single uppercase letter must be valid")
	})

	t.Run("empty string is rejected", func(t *testing.T) {
		err := validateEventType("")
		require.Error(t, err, "empty event type must be rejected")
		assert.ErrorIs(t, err, errInvalidEventType, "error must wrap errInvalidEventType")
	})

	t.Run("type with spaces is rejected", func(t *testing.T) {
		err := validateEventType("order created")
		require.Error(t, err, "event type with space must be rejected")
		assert.ErrorIs(t, err, errInvalidEventType, "error must wrap errInvalidEventType")
	})

	t.Run("type with invalid characters is rejected", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input string
		}{
			{"slash", "order/created"},
			{"hash", "order#1"},
			{"at sign", "@events"},
			{"asterisk wildcard", "order.*"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := validateEventType(tc.input)
				require.Error(t, err, "event type %q must be rejected", tc.input)
				assert.ErrorIs(t, err, errInvalidEventType, "error must wrap errInvalidEventType for %q", tc.input)
			})
		}
	})
}
