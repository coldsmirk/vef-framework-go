package config

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for lockout configuration validation.
var (
	ErrInvalidLockoutStrategy = errors.New("invalid lockout strategy")
	ErrInvalidLockoutKey      = errors.New("invalid lockout key")
)

// SecurityConfig defines security settings.
type SecurityConfig struct {
	// Secret is the hex-encoded key used to sign and verify JWT tokens.
	// Leave empty in development to have the framework generate an ephemeral
	// key at startup; set a stable per-deployment value in production.
	Secret           string        `config:"secret"`
	TokenExpires     time.Duration `config:"token_expires"`
	RefreshNotBefore time.Duration `config:"refresh_not_before"`
	LoginRateLimit   int           `config:"login_rate_limit"`
	RefreshRateLimit int           `config:"refresh_rate_limit"`
	// IPWhitelists names the source-IP whitelists served by the framework's
	// default security.IPWhitelistLoader; the built-in "ip" auth strategy
	// resolves api.IPAuth(name) against them, and the no-arg api.IPAuth()
	// targets the "default" key. Each entry is a single IP address or CIDR
	// range. Note: the config layer lowercases TOML keys, so whitelist names
	// are effectively lowercase.
	IPWhitelists map[string][]string `config:"ip_whitelists"`
	// Lockout configures brute-force protection on the login endpoint.
	Lockout LockoutConfig `config:"lockout"`
	// PasswordPolicy configures strength rules enforced when a password is set.
	PasswordPolicy PasswordPolicyConfig `config:"password_policy"`
}

// PasswordPolicyConfig configures password strength rules. Every field is
// opt-in: a zero value disables the corresponding rule, so an unconfigured
// policy accepts any password and enabling a rule is an explicit choice.
type PasswordPolicyConfig struct {
	// MinLength requires at least this many characters when > 0.
	MinLength int `config:"min_length"`
	// MaxLength requires at most this many characters when > 0.
	MaxLength int `config:"max_length"`
	// RequireUpper requires at least one uppercase letter.
	RequireUpper bool `config:"require_upper"`
	// RequireLower requires at least one lowercase letter.
	RequireLower bool `config:"require_lower"`
	// RequireDigit requires at least one digit.
	RequireDigit bool `config:"require_digit"`
	// RequireSymbol requires at least one symbol (non-space, non-alphanumeric).
	RequireSymbol bool `config:"require_symbol"`
	// MinCharClasses requires at least this many distinct character classes
	// (uppercase, lowercase, digit, symbol) when > 0.
	MinCharClasses int `config:"min_char_classes"`
	// DisallowUsername rejects a password that contains the account id or name.
	DisallowUsername bool `config:"disallow_username"`
	// Blocklist rejects passwords matching any listed entry (case-insensitive).
	Blocklist []string `config:"blocklist"`
	// HistoryDepth rejects a new password that repeats any of the subject's last
	// N passwords when > 0. Enforcement requires a security.PasswordHistoryStore.
	HistoryDepth int `config:"history_depth"`
	// MaxAge forces a password change once the password is older than this.
	// Zero disables expiry. Enforcement requires a security.PasswordMetadataLoader
	// and a security.ExpiryPasswordChangeChecker wired into the login flow.
	MaxAge time.Duration `config:"max_age"`
}

// HasRules reports whether any strength rule is configured.
func (c *PasswordPolicyConfig) HasRules() bool {
	return c.MinLength > 0 ||
		c.MaxLength > 0 ||
		c.RequireUpper ||
		c.RequireLower ||
		c.RequireDigit ||
		c.RequireSymbol ||
		c.MinCharClasses > 0 ||
		c.DisallowUsername ||
		len(c.Blocklist) > 0
}

// LockoutStrategy selects how repeated login failures are penalized.
type LockoutStrategy string

const (
	// LockoutStrategyLock blocks all further attempts for a fixed duration once
	// the failure threshold is reached.
	LockoutStrategyLock LockoutStrategy = "lock"
	// LockoutStrategyBackoff imposes an exponentially growing delay between
	// attempts past the threshold instead of a hard lock, so a legitimate user
	// is never fully locked out and a mass-lockout denial of service against a
	// targeted account is not possible.
	LockoutStrategyBackoff LockoutStrategy = "backoff"
)

// LockoutKey selects the identity dimension a lockout counter is keyed by.
type LockoutKey string

const (
	// LockoutKeyUser counts failures per login identifier, across all sources.
	LockoutKeyUser LockoutKey = "user"
	// LockoutKeyIP counts failures per source address, across all identifiers.
	LockoutKeyIP LockoutKey = "ip"
	// LockoutKeyUserIP counts failures per identifier-and-source pair. This is
	// the default: it throttles credential guessing while denying an attacker
	// the ability to lock a victim out of every source by guessing one account.
	LockoutKeyUserIP LockoutKey = "user_ip"
)

// Default values for LockoutConfig, applied by the Effective* accessors when a
// field is left at its zero value.
const (
	DefaultLockoutMaxFailures  = 10
	DefaultLockoutWindow       = 15 * time.Minute
	DefaultLockoutLockDuration = 15 * time.Minute
	DefaultLockoutBackoffBase  = 1 * time.Second
	DefaultLockoutBackoffMax   = 15 * time.Minute
)

// LockoutConfig configures brute-force protection on the login endpoint.
//
// Zero values resolve to defaults through the Effective* accessors; treat the
// parsed struct as raw input and read policy through those accessors, never the
// fields directly.
type LockoutConfig struct {
	// Enabled toggles lockout. A nil pointer resolves to enabled, so brute-force
	// protection is on by default; set it to false to switch the feature off.
	Enabled *bool `config:"enabled"`
	// MaxFailures is the number of failed attempts tolerated before the strategy
	// engages. Default: 10.
	MaxFailures int `config:"max_failures"`
	// Window is how long failed attempts are remembered; a spell of no failures
	// this long resets the counter. Default: 15m.
	Window time.Duration `config:"window"`
	// LockDuration is how long attempts are blocked once the threshold is
	// reached under the "lock" strategy. Default: 15m.
	LockDuration time.Duration `config:"lock_duration"`
	// Strategy selects "lock" (hard block) or "backoff" (escalating delay).
	// Default: "lock".
	Strategy LockoutStrategy `config:"strategy"`
	// BackoffBase is the first delay imposed past the threshold under the
	// "backoff" strategy; it doubles per further failure. Default: 1s.
	BackoffBase time.Duration `config:"backoff_base"`
	// BackoffMax caps the per-attempt backoff delay. Default: 15m.
	BackoffMax time.Duration `config:"backoff_max"`
	// Key selects the identity dimension the counter is keyed by. Default:
	// "user_ip".
	Key LockoutKey `config:"key"`
}

// IsEnabled reports whether lockout is active. An omitted toggle means enabled.
func (c *LockoutConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// EffectiveMaxFailures returns MaxFailures or its default.
func (c *LockoutConfig) EffectiveMaxFailures() int {
	return coalescePositive(c.MaxFailures, DefaultLockoutMaxFailures)
}

// EffectiveWindow returns Window or its default.
func (c *LockoutConfig) EffectiveWindow() time.Duration {
	return coalescePositive(c.Window, DefaultLockoutWindow)
}

// EffectiveLockDuration returns LockDuration or its default.
func (c *LockoutConfig) EffectiveLockDuration() time.Duration {
	return coalescePositive(c.LockDuration, DefaultLockoutLockDuration)
}

// EffectiveStrategy returns Strategy or its default.
func (c *LockoutConfig) EffectiveStrategy() LockoutStrategy {
	if c.Strategy == "" {
		return LockoutStrategyLock
	}

	return c.Strategy
}

// EffectiveBackoffBase returns BackoffBase or its default.
func (c *LockoutConfig) EffectiveBackoffBase() time.Duration {
	return coalescePositive(c.BackoffBase, DefaultLockoutBackoffBase)
}

// EffectiveBackoffMax returns BackoffMax or its default.
func (c *LockoutConfig) EffectiveBackoffMax() time.Duration {
	return coalescePositive(c.BackoffMax, DefaultLockoutBackoffMax)
}

// EffectiveKey returns Key or its default.
func (c *LockoutConfig) EffectiveKey() LockoutKey {
	if c.Key == "" {
		return LockoutKeyUserIP
	}

	return c.Key
}

// Validate rejects out-of-enum strategy and key values so a configuration typo
// fails fast at boot instead of silently degrading to a default.
func (c *LockoutConfig) Validate() error {
	switch c.EffectiveStrategy() {
	case LockoutStrategyLock, LockoutStrategyBackoff:
	default:
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidLockoutStrategy, c.Strategy, LockoutStrategyLock, LockoutStrategyBackoff)
	}

	switch c.EffectiveKey() {
	case LockoutKeyUser, LockoutKeyIP, LockoutKeyUserIP:
	default:
		return fmt.Errorf("%w %q (want %q, %q or %q)", ErrInvalidLockoutKey, c.Key, LockoutKeyUser, LockoutKeyIP, LockoutKeyUserIP)
	}

	return nil
}
