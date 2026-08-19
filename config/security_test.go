package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
)

func TestSecurityConfigValidate(t *testing.T) {
	t.Run("AcceptsEmptyAndValidEnums", func(t *testing.T) {
		require.NoError(t, (&config.SecurityConfig{}).Validate(), "an empty config should validate on defaults")
		require.NoError(t, (&config.SecurityConfig{
			TokenType: config.TokenTypeOpaque,
			Session:   config.SessionConfig{OnExceed: config.SessionExceedReject},
		}).Validate(), "valid enums should validate")
	})

	t.Run("RejectsInvalidTokenType", func(t *testing.T) {
		err := (&config.SecurityConfig{TokenType: "bogus"}).Validate()
		require.ErrorIs(t, err, config.ErrInvalidTokenType, "an out-of-enum token type should fail fast at boot")
	})

	t.Run("RejectsInvalidOnExceed", func(t *testing.T) {
		err := (&config.SecurityConfig{Session: config.SessionConfig{OnExceed: "bogus"}}).Validate()
		require.ErrorIs(t, err, config.ErrInvalidSessionOnExceed, "an out-of-enum on_exceed should fail fast at boot")
	})

	t.Run("EffectiveTokenType", func(t *testing.T) {
		assert.Equal(t, config.TokenTypeJWT, (&config.SecurityConfig{}).EffectiveTokenType(), "an empty token type should default to jwt_token")
		assert.Equal(t, config.TokenTypeOpaque, (&config.SecurityConfig{TokenType: config.TokenTypeOpaque}).EffectiveTokenType(), "a set token type should be honored")
	})
}

func TestLockoutConfigValidate(t *testing.T) {
	t.Run("RejectsInvalidStrategy", func(t *testing.T) {
		err := (&config.LockoutConfig{Strategy: "bogus"}).Validate()
		require.ErrorIs(t, err, config.ErrInvalidLockoutStrategy, "an out-of-enum strategy should fail fast at boot")
	})

	t.Run("RejectsInvalidKey", func(t *testing.T) {
		err := (&config.LockoutConfig{Key: "bogus"}).Validate()
		require.ErrorIs(t, err, config.ErrInvalidLockoutKey, "an out-of-enum key should fail fast at boot")
	})

	t.Run("AcceptsValidAndEmpty", func(t *testing.T) {
		require.NoError(t, (&config.LockoutConfig{}).Validate(), "an empty lockout config should validate")
		require.NoError(t, (&config.LockoutConfig{Strategy: config.LockoutStrategyBackoff, Key: config.LockoutKeyIP}).Validate(), "valid enums should validate")
	})
}

func TestLockoutConfigIsEnabled(t *testing.T) {
	assert.True(t, (&config.LockoutConfig{}).IsEnabled(), "an omitted toggle must default to enabled — brute-force protection is on by default")
	assert.False(t, (&config.LockoutConfig{Enabled: new(bool)}).IsEnabled(), "an explicit false should disable lockout")
}

func TestSessionConfigDefaults(t *testing.T) {
	t.Run("IsSliding", func(t *testing.T) {
		assert.True(t, (&config.SessionConfig{}).IsSliding(), "an omitted sliding toggle should default to on")
		assert.False(t, (&config.SessionConfig{Sliding: new(bool)}).IsSliding(), "an explicit false should disable sliding renewal")
	})

	t.Run("EffectiveAccessorsDefaultThenHonorSetValues", func(t *testing.T) {
		zero := &config.SessionConfig{}
		assert.Equal(t, config.DefaultSessionIdleTTL, zero.EffectiveIdleTTL(), "a zero idle ttl should fall back to the default")
		assert.Equal(t, config.DefaultSessionMaxLifetime, zero.EffectiveMaxLifetime(), "a zero max lifetime should fall back to the default")
		assert.Equal(t, config.SessionExceedEvictOldest, zero.EffectiveOnExceed(), "an empty on_exceed should default to evict_oldest")

		set := &config.SessionConfig{IdleTTL: time.Minute, MaxLifetime: time.Hour, OnExceed: config.SessionExceedReject}
		assert.Equal(t, time.Minute, set.EffectiveIdleTTL(), "a set idle ttl should be honored")
		assert.Equal(t, time.Hour, set.EffectiveMaxLifetime(), "a set max lifetime should be honored")
		assert.Equal(t, config.SessionExceedReject, set.EffectiveOnExceed(), "a set on_exceed should be honored")
	})
}

func TestTrustLoginConfig(t *testing.T) {
	validApp := config.TrustLoginAppConfig{RedirectURLs: []string{"http://app.local/portal"}}

	t.Run("Defaults", func(t *testing.T) {
		cfg := new(config.TrustLoginConfig)

		assert.Equal(t, config.DefaultTrustLoginPath, cfg.EffectivePath(), "an unset path should resolve to the default")
		assert.Equal(t, config.DefaultTrustLoginCodeTTL, cfg.EffectiveCodeTTL(), "an unset code TTL should resolve to the default")
		assert.True(t, cfg.IsUserAgentBound(), "an omitted bind_user_agent should resolve to enabled")
		assert.False(t, cfg.BindClientIP, "IP binding should be off unless asked for")
	})

	t.Run("Overrides", func(t *testing.T) {
		disabled := false
		cfg := &config.TrustLoginConfig{
			Path:          "/portal/sso",
			CodeTTL:       30 * time.Second,
			BindUserAgent: &disabled,
		}

		assert.Equal(t, "/portal/sso", cfg.EffectivePath(), "an explicit path should win")
		assert.Equal(t, 30*time.Second, cfg.EffectiveCodeTTL(), "an explicit code TTL should win")
		assert.False(t, cfg.IsUserAgentBound(), "an explicit false bind_user_agent should survive")
	})

	t.Run("Validate", func(t *testing.T) {
		t.Run("AcceptsDisabledAndEmpty", func(t *testing.T) {
			require.NoError(t, (&config.TrustLoginConfig{}).Validate(),
				"a disabled, unconfigured gateway should validate")
		})

		t.Run("AcceptsEnabledWithApps", func(t *testing.T) {
			require.NoError(t, (&config.TrustLoginConfig{
				Enabled: true,
				Apps:    map[string]config.TrustLoginAppConfig{"his": validApp},
			}).Validate(), "an enabled gateway with a configured app should validate")
		})

		// Shape checks run even while disabled, so a typo surfaces at boot
		// rather than the day the feature is switched on.
		t.Run("RejectsBadPathWhileDisabled", func(t *testing.T) {
			err := (&config.TrustLoginConfig{Path: "sso/trust"}).Validate()
			require.ErrorIs(t, err, config.ErrTrustLoginPathInvalid, "a path without a leading slash should fail fast")
		})

		t.Run("RejectsEnabledWithoutApps", func(t *testing.T) {
			err := (&config.TrustLoginConfig{Enabled: true}).Validate()
			require.ErrorIs(t, err, config.ErrTrustLoginAppsRequired,
				"an enabled gateway no app may use could never authenticate a handoff")
		})

		t.Run("RejectsAppWithoutRedirects", func(t *testing.T) {
			err := (&config.TrustLoginConfig{
				Enabled: true,
				Apps:    map[string]config.TrustLoginAppConfig{"his": {}},
			}).Validate()
			require.ErrorIs(t, err, config.ErrTrustLoginRedirectsEmpty, "an app with no redirect URL could never complete a handoff")
		})

		t.Run("RejectsMalformedRedirects", func(t *testing.T) {
			for name, entry := range map[string]string{
				"Relative":    "/portal",
				"SchemeOnly":  "http://",
				"WithQuery":   "http://app.local/portal?tab=1",
				"WithFragmnt": "http://app.local/portal#top",
			} {
				t.Run(name, func(t *testing.T) {
					err := (&config.TrustLoginConfig{
						Enabled: true,
						Apps:    map[string]config.TrustLoginAppConfig{"his": {RedirectURLs: []string{entry}}},
					}).Validate()
					require.ErrorIs(t, err, config.ErrTrustLoginRedirectInvalid,
						"a redirect entry must be an absolute URL with no query or fragment")
				})
			}
		})
	})

	// SecurityConfig.Validate is what the security module calls at boot, so the
	// trust-login checks have to be reachable from there.
	t.Run("ReachedFromSecurityConfig", func(t *testing.T) {
		err := (&config.SecurityConfig{TrustLogin: config.TrustLoginConfig{Enabled: true}}).Validate()
		require.ErrorIs(t, err, config.ErrTrustLoginAppsRequired,
			"trust login validation should run as part of the security config validation")
	})
}
