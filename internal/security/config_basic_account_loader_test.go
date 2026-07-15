package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

func newBasicAccountConfig(accounts map[string]config.BasicAccountConfig) *config.SecurityConfig {
	return &config.SecurityConfig{BasicAccounts: accounts}
}

func TestNewConfigBasicAccountLoader(t *testing.T) {
	t.Run("NoAccountsConfigured", func(t *testing.T) {
		loader, err := NewConfigBasicAccountLoader(newBasicAccountConfig(nil))
		require.NoError(t, err, "An absent basic_accounts section is valid")

		principal, secret, err := loader.LoadByUsername(t.Context(), "svc")
		require.NoError(t, err, "LoadByUsername should not fail on an empty loader")
		assert.Nil(t, principal, "No configured account can match")
		assert.Empty(t, secret, "No secret should be returned for an unknown account")
	})

	t.Run("KnownAccountResolvesToExternalAppPrincipal", func(t *testing.T) {
		loader, err := NewConfigBasicAccountLoader(newBasicAccountConfig(map[string]config.BasicAccountConfig{
			"lis-uploader": {Password: "s3cret", Roles: []string{"lab:write"}},
		}))
		require.NoError(t, err, "Valid config should construct")

		principal, secret, err := loader.LoadByUsername(t.Context(), "lis-uploader")
		require.NoError(t, err, "LoadByUsername should not fail for a known account")
		require.NotNil(t, principal, "A known account must resolve to a principal")
		assert.Equal(t, security.PrincipalTypeExternalApp, principal.Type, "Principal type must be external app")
		assert.Equal(t, "http_basic:lis-uploader", principal.ID, "Principal ID must carry the strategy prefix and username")
		assert.Equal(t, "lis-uploader", principal.Name, "Principal name must be the username")
		assert.Equal(t, []string{"lab:write"}, principal.Roles, "Configured roles must be granted")
		assert.Equal(t, "s3cret", secret, "The stored secret must be returned for framework-side comparison")
	})

	t.Run("UnknownUsernameResolvesToNil", func(t *testing.T) {
		loader, err := NewConfigBasicAccountLoader(newBasicAccountConfig(map[string]config.BasicAccountConfig{
			"lis-uploader": {Password: "s3cret"},
		}))
		require.NoError(t, err, "Valid config should construct")

		principal, _, err := loader.LoadByUsername(t.Context(), "intruder")
		require.NoError(t, err, "An unknown username is not an error")
		assert.Nil(t, principal, "Unknown usernames must resolve to nil")
	})

	t.Run("BlankUsernameFailsConstruction", func(t *testing.T) {
		_, err := NewConfigBasicAccountLoader(newBasicAccountConfig(map[string]config.BasicAccountConfig{
			" ": {Password: "s3cret"},
		}))
		require.ErrorIs(t, err, ErrBasicAccountUsernameBlank, "A blank username must fail start-up")
	})

	t.Run("BlankPasswordFailsConstruction", func(t *testing.T) {
		_, err := NewConfigBasicAccountLoader(newBasicAccountConfig(map[string]config.BasicAccountConfig{
			"lis-uploader": {},
		}))
		require.ErrorIs(t, err, ErrBasicAccountPasswordBlank, "A blank password must fail start-up")
	})
}
