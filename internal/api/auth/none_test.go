package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
)

// TestNoneStrategyName verifies the strategy identifier.
func TestNoneStrategyName(t *testing.T) {
	s := NewNone()
	assert.Equal(t, "none", s.Name(), "NoneStrategy name should be 'none'")
}

// TestNoneStrategyAuthenticate covers the anonymous principal behavior.
func TestNoneStrategyAuthenticate(t *testing.T) {
	t.Run("ReturnsAnonymousPrincipal", func(t *testing.T) {
		s := NewNone()
		principal, err := s.Authenticate(nil, nil)
		require.NoError(t, err, "Authenticate should not return an error")
		require.NotNil(t, principal, "Authenticate should return a non-nil principal")
		assert.Equal(t, security.PrincipalAnonymous.ID, principal.ID, "Principal ID should match anonymous ID")
		assert.Equal(t, security.PrincipalAnonymous.Name, principal.Name, "Principal Name should match anonymous name")
	})

	t.Run("ReturnsFreshCopyEachCall", func(t *testing.T) {
		s := NewNone()
		p1, err := s.Authenticate(nil, nil)
		require.NoError(t, err, "First Authenticate should not return an error")
		p2, err := s.Authenticate(nil, nil)
		require.NoError(t, err, "Second Authenticate should not return an error")

		assert.NotSame(t, p1, p2, "Authenticate must return a distinct pointer on each call to avoid shared-state corruption")
		assert.NotSame(t, p1, security.PrincipalAnonymous, "Authenticate must not return the shared PrincipalAnonymous singleton")
	})

	t.Run("MutatingReturnedPrincipalDoesNotCorruptSingleton", func(t *testing.T) {
		s := NewNone()
		principal, err := s.Authenticate(nil, nil)
		require.NoError(t, err, "Authenticate should not return an error")

		originalID := security.PrincipalAnonymous.ID
		principal.ID = "mutated"

		assert.Equal(t, originalID, security.PrincipalAnonymous.ID,
			"Mutating the returned principal must not change the global PrincipalAnonymous singleton")
	})
}
