package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Sentinel errors for security configuration validation.
var (
	ErrInvalidLockoutStrategy = errors.New("invalid lockout strategy")
	ErrInvalidLockoutKey      = errors.New("invalid lockout key")
	ErrInvalidTokenType       = errors.New("invalid token type")
	ErrInvalidSessionOnExceed = errors.New("invalid session on_exceed policy")

	ErrTrustLoginPathInvalid     = errors.New("trust login path must start with '/'")
	ErrTrustLoginAppsRequired    = errors.New("trust login is enabled but no external app is allowed to initiate a handoff")
	ErrTrustLoginRedirectsEmpty  = errors.New("trust login app declares no redirect URLs")
	ErrTrustLoginRedirectInvalid = errors.New("trust login redirect URL must be absolute and carry no query or fragment")
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
	// APIKeys names the static API keys served by the framework's default
	// security.APIKeyLoader; the built-in "api_key" auth strategy resolves the
	// presented key against them. Note: the config layer lowercases TOML keys,
	// so key names are effectively lowercase.
	APIKeys map[string]APIKeyConfig `config:"api_keys"`
	// BasicAccounts names the static service accounts served by the framework's
	// default security.BasicAccountLoader; the built-in "http_basic" auth
	// strategy verifies the presented Authorization: Basic credentials against
	// them. Note: the config layer lowercases TOML keys, so usernames are
	// effectively lowercase.
	BasicAccounts map[string]BasicAccountConfig `config:"basic_accounts"`
	// Lockout configures brute-force protection on the login endpoint.
	Lockout LockoutConfig `config:"lockout"`
	// PasswordPolicy configures strength rules enforced when a password is set.
	PasswordPolicy PasswordPolicyConfig `config:"password_policy"`
	// TokenType selects the login token mechanism: stateless "jwt_token"
	// (default) or stateful "opaque_token". Session control (concurrency limits,
	// force-offline, renewal) is only available with opaque tokens.
	TokenType TokenType `config:"token_type"`
	// Session configures opaque-token session behavior; it has no effect under
	// the jwt_token mechanism.
	Session SessionConfig `config:"session"`
	// TrustLogin configures the trust-login single sign-on gateway.
	TrustLogin TrustLoginConfig `config:"trust_login"`
}

// APIKeyConfig defines one static API key under vef.security.api_keys.
type APIKeyConfig struct {
	// Key is the secret value the client presents; treat it like a password
	// and use a high-entropy random string.
	Key string `config:"key"`
	// Roles are granted to the authenticated principal.
	Roles []string `config:"roles"`
}

// BasicAccountConfig defines one static service account under
// vef.security.basic_accounts; the map key is the username.
type BasicAccountConfig struct {
	// Password is the account secret; this is a machine-to-machine credential,
	// so use a high-entropy random string, not a human password.
	Password string `config:"password"`
	// Roles are granted to the authenticated principal.
	Roles []string `config:"roles"`
}

// TokenType selects the login token mechanism.
type TokenType string

const (
	// TokenTypeJWT issues stateless, self-contained JWT access/refresh tokens.
	TokenTypeJWT TokenType = "jwt_token"
	// TokenTypeOpaque issues stateful opaque tokens backed by a session store.
	TokenTypeOpaque TokenType = "opaque_token"
)

// EffectiveTokenType returns TokenType or its default (jwt_token).
func (c *SecurityConfig) EffectiveTokenType() TokenType {
	if c.TokenType == "" {
		return TokenTypeJWT
	}

	return c.TokenType
}

// SessionExceedPolicy selects what happens when a login exceeds the concurrent
// session limit.
type SessionExceedPolicy string

const (
	// SessionExceedReject denies the new login once the limit is reached.
	SessionExceedReject SessionExceedPolicy = "reject"
	// SessionExceedEvictOldest revokes the oldest session to admit the new login.
	SessionExceedEvictOldest SessionExceedPolicy = "evict_oldest"
)

// Default values for SessionConfig, applied by the Effective* accessors.
const (
	DefaultSessionIdleTTL     = 30 * time.Minute
	DefaultSessionMaxLifetime = 7 * 24 * time.Hour
)

// SessionConfig configures opaque-token sessions. Zero values resolve to
// defaults through the Effective* accessors.
type SessionConfig struct {
	// MaxConcurrent bounds simultaneous sessions per account; 0 is unlimited.
	// Enforcement is best-effort under concurrent logins: a burst of simultaneous
	// logins for one account may briefly exceed the limit by the number of racing
	// requests before it settles.
	MaxConcurrent int `config:"max_concurrent"`
	// OnExceed selects reject vs. evict-oldest when the limit is hit.
	// Default: evict_oldest (a new login kicks the earliest device offline).
	OnExceed SessionExceedPolicy `config:"on_exceed"`
	// IdleTTL is the inactivity timeout and the sliding renewal window applied on
	// each authenticated request. Default: 30m.
	IdleTTL time.Duration `config:"idle_ttl"`
	// MaxLifetime caps a session's total age regardless of renewal; 0 is
	// uncapped. Default: 7 days.
	MaxLifetime time.Duration `config:"max_lifetime"`
	// Sliding enables idle-timeout renewal on each request. A nil pointer
	// resolves to enabled.
	Sliding *bool `config:"sliding"`
}

// EffectiveOnExceed returns OnExceed or its default.
func (c *SessionConfig) EffectiveOnExceed() SessionExceedPolicy {
	if c.OnExceed == "" {
		return SessionExceedEvictOldest
	}

	return c.OnExceed
}

// EffectiveIdleTTL returns IdleTTL or its default.
func (c *SessionConfig) EffectiveIdleTTL() time.Duration {
	return coalescePositive(c.IdleTTL, DefaultSessionIdleTTL)
}

// EffectiveMaxLifetime returns MaxLifetime or its default.
func (c *SessionConfig) EffectiveMaxLifetime() time.Duration {
	return coalescePositive(c.MaxLifetime, DefaultSessionMaxLifetime)
}

// IsSliding reports whether idle-timeout renewal is enabled. Omitted means on.
func (c *SessionConfig) IsSliding() bool {
	return c.Sliding == nil || *c.Sliding
}

// Validate rejects out-of-enum token type and session policy so a configuration
// typo fails fast at boot.
func (c *SecurityConfig) Validate() error {
	switch c.EffectiveTokenType() {
	case TokenTypeJWT, TokenTypeOpaque:
	default:
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidTokenType, c.TokenType, TokenTypeJWT, TokenTypeOpaque)
	}

	switch c.Session.EffectiveOnExceed() {
	case SessionExceedReject, SessionExceedEvictOldest:
	default:
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidSessionOnExceed, c.Session.OnExceed, SessionExceedReject, SessionExceedEvictOldest)
	}

	return c.TrustLogin.Validate()
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
	// RequireSymbol requires at least one symbol (non-space, non-letter,
	// non-digit; caseless letters such as CJK do not count).
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

// Default values for TrustLoginConfig, applied by the Effective* accessors.
const (
	DefaultTrustLoginPath    = "/sso/trust"
	DefaultTrustLoginCodeTTL = 60 * time.Second

	DefaultTrustLoginRateLimitMax    = 120
	DefaultTrustLoginRateLimitPeriod = time.Minute
)

// TrustLoginConfig configures the trust-login single sign-on gateway: a
// browser-facing endpoint an external system links to, carrying a signed user
// identifier, which trades the handoff for an ordinary login session.
//
// Participation is per external app. An app must appear in Apps to initiate a
// handoff, which keeps browser single sign-on a grant distinct from the API
// access the same app ID may already hold through security.ExternalAppLoader —
// being able to call the API never implies being able to log a user in.
type TrustLoginConfig struct {
	// Enabled mounts the gateway route and registers the code-exchange
	// authenticator. Off by default; while off, the route does not exist and
	// the trust_code login mechanism is refused as unsupported.
	Enabled bool `config:"enabled"`
	// Path is the gateway route. Default: /sso/trust. It is part of the signed
	// payload, so changing it invalidates every link the external system has
	// already generated.
	//
	// The external system must sign — and request — this path exactly. Fiber
	// runs with StrictRouting off, so "/sso/trust/" still reaches the gateway
	// but hashes as a different path: a trailing slash produces a permanent,
	// opaque 401 rather than a routing error. It fails closed, but it is worth
	// stating to whoever implements the signing side.
	Path string `config:"path"`
	// CodeTTL bounds how long the one-time code stays redeemable. Default: 60s.
	// Keep it short: the code rides a redirect URL, so it lands in browser
	// history and in any intermediary's access logs.
	CodeTTL time.Duration `config:"code_ttl"`
	// BindUserAgent requires the browser redeeming a code to present the same
	// User-Agent the gateway redirected. A nil pointer resolves to enabled: the
	// header cannot change within one redirect, so the binding costs nothing
	// and blocks a code lifted out of a URL and replayed from elsewhere.
	BindUserAgent *bool `config:"bind_user_agent"`
	// BindClientIP additionally requires the same source address. Off by
	// default: a mobile client can change networks mid-redirect, where the
	// resulting failure reads as a broken integration rather than a defense.
	BindClientIP bool `config:"bind_client_ip"`
	// Apps names the external systems allowed to initiate a handoff, keyed by
	// app ID. Note: the config layer lowercases TOML keys, so app IDs are
	// effectively lowercase.
	Apps map[string]TrustLoginAppConfig `config:"apps"`
	// RateLimit bounds handoff attempts. The gateway is public and does an
	// ExternalAppLoader lookup — typically a database round trip — before it
	// can check anything, so it needs the same floodgate the integration
	// inbound gateway has.
	RateLimit TrustLoginRateLimitConfig `config:"rate_limit"`
}

// TrustLoginRateLimitConfig bounds trust-login handoff throughput. The limiter
// counts per (app ID, client IP) per node, so one flooding source cannot
// starve the other apps.
//
// The default is deliberately generous: the key includes the client IP, and a
// whole organization behind one NAT legitimately signs in through the same
// address at the start of a shift. It bounds a flood, it does not police
// logins — a stolen handoff URL is stopped by the nonce store and the code TTL,
// not by this.
type TrustLoginRateLimitConfig struct {
	// Max is the number of handoffs allowed per Period. Default: 120.
	Max int `config:"max"`
	// Period is the sliding window. Default: 1 minute.
	Period time.Duration `config:"period"`
}

// EffectiveMax returns Max or its default.
func (c *TrustLoginRateLimitConfig) EffectiveMax() int {
	return coalescePositive(c.Max, DefaultTrustLoginRateLimitMax)
}

// EffectivePeriod returns Period or its default.
func (c *TrustLoginRateLimitConfig) EffectivePeriod() time.Duration {
	return coalescePositive(c.Period, DefaultTrustLoginRateLimitPeriod)
}

// TrustLoginAppConfig is one external system's trust-login policy.
//
// The app's signing secret is deliberately absent: it is loaded through
// security.ExternalAppLoader, the same source the API signature authenticator
// reads, so a secret never has to live in a configuration file.
type TrustLoginAppConfig struct {
	// RedirectURLs allowlists where a handoff may land. Each entry is an
	// absolute URL; a requested target must match an entry's scheme and host
	// exactly and sit under its path. At least one entry is required — an app
	// with none could never complete a handoff.
	RedirectURLs []string `config:"redirect_urls"`
}

// EffectivePath returns Path or its default.
func (c *TrustLoginConfig) EffectivePath() string {
	if c.Path == "" {
		return DefaultTrustLoginPath
	}

	return c.Path
}

// EffectiveCodeTTL returns CodeTTL or its default.
func (c *TrustLoginConfig) EffectiveCodeTTL() time.Duration {
	return coalescePositive(c.CodeTTL, DefaultTrustLoginCodeTTL)
}

// IsUserAgentBound reports whether a code is bound to the redirected browser's
// User-Agent. Omitted means on.
func (c *TrustLoginConfig) IsUserAgentBound() bool {
	return c.BindUserAgent == nil || *c.BindUserAgent
}

// Validate rejects a trust-login configuration that could never authenticate a
// handoff. Shape checks run even while the feature is disabled so a typo
// surfaces at boot rather than when it is switched on; the "at least one app"
// requirement is the one check that only applies once enabled.
func (c *TrustLoginConfig) Validate() error {
	if path := c.EffectivePath(); !strings.HasPrefix(path, "/") {
		return fmt.Errorf("%w: %q", ErrTrustLoginPathInvalid, path)
	}

	if c.Enabled && len(c.Apps) == 0 {
		return ErrTrustLoginAppsRequired
	}

	for appID, app := range c.Apps {
		if err := app.validate(appID); err != nil {
			return err
		}
	}

	return nil
}

// validate checks one app's redirect allowlist.
func (c *TrustLoginAppConfig) validate(appID string) error {
	if len(c.RedirectURLs) == 0 {
		return fmt.Errorf("%w: app %q", ErrTrustLoginRedirectsEmpty, appID)
	}

	for _, entry := range c.RedirectURLs {
		parsed, err := url.Parse(entry)
		if err != nil {
			return fmt.Errorf("%w: app %q entry %q: %w", ErrTrustLoginRedirectInvalid, appID, entry, err)
		}

		if parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("%w: app %q entry %q", ErrTrustLoginRedirectInvalid, appID, entry)
		}
	}

	return nil
}
