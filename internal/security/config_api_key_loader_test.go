package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

func newAPIKeyConfig(keys map[string]config.APIKeyConfig) *config.SecurityConfig {
	return &config.SecurityConfig{APIKeys: keys}
}

func TestNewConfigAPIKeyLoader(t *testing.T) {
	t.Run("NoKeysConfigured", func(t *testing.T) {
		loader, err := NewConfigAPIKeyLoader(newAPIKeyConfig(nil))
		require.NoError(t, err, "An absent api_keys section is valid")

		principal, err := loader.LoadByKey(t.Context(), "anything")
		require.NoError(t, err, "LoadByKey should not fail on an empty loader")
		assert.Nil(t, principal, "No configured key can match")
	})

	t.Run("MatchResolvesToExternalAppPrincipal", func(t *testing.T) {
		loader, err := NewConfigAPIKeyLoader(newAPIKeyConfig(map[string]config.APIKeyConfig{
			"reporting": {Key: "sk-report-123", Roles: []string{"report:read"}},
		}))
		require.NoError(t, err, "Valid config should construct")

		principal, err := loader.LoadByKey(t.Context(), "sk-report-123")
		require.NoError(t, err, "LoadByKey should not fail for a known key")
		require.NotNil(t, principal, "A matching key must resolve to a principal")
		assert.Equal(t, security.PrincipalTypeExternalApp, principal.Type, "Principal type must be external app")
		assert.Equal(t, "api_key:reporting", principal.ID, "Principal ID must carry the strategy prefix and key name")
		assert.Equal(t, "reporting", principal.Name, "Principal name must be the key name")
		assert.Equal(t, []string{"report:read"}, principal.Roles, "Configured roles must be granted")
	})

	t.Run("UnknownKeyResolvesToNil", func(t *testing.T) {
		loader, err := NewConfigAPIKeyLoader(newAPIKeyConfig(map[string]config.APIKeyConfig{
			"reporting": {Key: "sk-report-123"},
		}))
		require.NoError(t, err, "Valid config should construct")

		principal, err := loader.LoadByKey(t.Context(), "sk-wrong")
		require.NoError(t, err, "An unmatched key is not an error")
		assert.Nil(t, principal, "Unmatched keys must resolve to nil")
	})

	t.Run("BlankNameFailsConstruction", func(t *testing.T) {
		_, err := NewConfigAPIKeyLoader(newAPIKeyConfig(map[string]config.APIKeyConfig{
			"  ": {Key: "sk-x"},
		}))
		require.ErrorIs(t, err, ErrAPIKeyNameBlank, "A blank key name must fail start-up")
	})

	t.Run("BlankKeyValueFailsConstruction", func(t *testing.T) {
		_, err := NewConfigAPIKeyLoader(newAPIKeyConfig(map[string]config.APIKeyConfig{
			"reporting": {},
		}))
		require.ErrorIs(t, err, ErrAPIKeyValueBlank, "A blank key value must fail start-up")
	})
}
