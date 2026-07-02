package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func newLoaderConfig(whitelists map[string][]string) *config.SecurityConfig {
	return &config.SecurityConfig{IPWhitelists: whitelists}
}

func TestNewConfigIPWhitelistLoader(t *testing.T) {
	t.Run("NoWhitelistsConfigured", func(t *testing.T) {
		loader, err := NewConfigIPWhitelistLoader(newLoaderConfig(nil))
		require.NoError(t, err, "An absent ip_whitelists section is valid")

		whitelist, err := loader.LoadByName(t.Context(), "internal")
		require.NoError(t, err, "LoadByName should not fail on an empty loader")
		assert.Nil(t, whitelist, "Unknown names must resolve to nil")
	})

	t.Run("ValidWhitelists", func(t *testing.T) {
		loader, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {"10.0.0.0/8", "192.168.1.1"},
			"partners": {"2001:db8::/32", "::1"},
		}))
		require.NoError(t, err, "IP and CIDR entries in IPv4 and IPv6 form are valid")

		whitelist, err := loader.LoadByName(t.Context(), "internal")
		require.NoError(t, err, "LoadByName should not fail for a known name")
		require.NotNil(t, whitelist, "Known names must resolve to a whitelist")
		assert.Equal(t, []string{"10.0.0.0/8", "192.168.1.1"}, whitelist.Entries, "Entries must be returned verbatim")
	})

	t.Run("UnknownNameResolvesToNil", func(t *testing.T) {
		loader, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {"10.0.0.0/8"},
		}))
		require.NoError(t, err, "Valid config should construct")

		whitelist, err := loader.LoadByName(t.Context(), "unknown")
		require.NoError(t, err, "An unknown name is not an error")
		assert.Nil(t, whitelist, "Unknown names must resolve to nil, not an empty whitelist")
	})

	t.Run("BlankNameFailsConstruction", func(t *testing.T) {
		_, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"  ": {"10.0.0.0/8"},
		}))
		require.Error(t, err, "A blank whitelist name is a configuration fault")
		assert.ErrorIs(t, err, ErrIPWhitelistNameBlank, "The error should be the blank-name sentinel")
	})

	t.Run("EmptyWhitelistFailsConstruction", func(t *testing.T) {
		_, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {},
		}))
		require.Error(t, err, "An empty whitelist can never authenticate and must fail start-up")
		assert.ErrorIs(t, err, ErrIPWhitelistEmpty, "The error should wrap the empty-whitelist sentinel")
		assert.Contains(t, err.Error(), `"internal"`, "The error should name the offending whitelist")
	})

	t.Run("BlankEntriesOnlyFailsConstruction", func(t *testing.T) {
		_, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {"", "   "},
		}))
		require.Error(t, err, "A whitelist of blank entries can never authenticate and must fail start-up")
	})

	t.Run("InvalidCIDRFailsConstruction", func(t *testing.T) {
		_, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {"10.0.0.0/8", "192.168.1.0/33"},
		}))
		require.Error(t, err, "An invalid CIDR range is a configuration fault")
		assert.ErrorIs(t, err, ErrIPWhitelistEntryInvalid, "The error should wrap the invalid-entry sentinel")
		assert.Contains(t, err.Error(), "192.168.1.0/33", "The error should name the offending entry")
	})

	t.Run("InvalidIPFailsConstruction", func(t *testing.T) {
		_, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {"not-an-ip"},
		}))
		require.Error(t, err, "An invalid IP address is a configuration fault")
		assert.ErrorIs(t, err, ErrIPWhitelistEntryInvalid, "The error should wrap the invalid-entry sentinel")
		assert.Contains(t, err.Error(), "not-an-ip", "The error should name the offending entry")
	})

	t.Run("EntriesWithSurroundingWhitespaceAreValid", func(t *testing.T) {
		loader, err := NewConfigIPWhitelistLoader(newLoaderConfig(map[string][]string{
			"internal": {" 10.0.0.0/8 ", "192.168.1.1"},
		}))
		require.NoError(t, err, "Whitespace around entries is tolerated, matching the validator's trimming")

		whitelist, err := loader.LoadByName(t.Context(), "internal")
		require.NoError(t, err, "LoadByName should not fail for a known name")
		require.NotNil(t, whitelist, "Known names must resolve to a whitelist")
	})
}
