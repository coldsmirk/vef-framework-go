package security

import (
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/security"
)

var logger = logx.Named("security")

var Module = fx.Module(
	"vef:security",
	fx.Decorate(func(cfg *config.SecurityConfig) *config.SecurityConfig {
		if cfg.TokenExpires <= 0 {
			cfg.TokenExpires = RefreshTokenExpires
		}

		if cfg.RefreshNotBefore <= 0 {
			cfg.RefreshNotBefore = AccessTokenExpires / 2
		}

		if cfg.LoginRateLimit <= 0 {
			cfg.LoginRateLimit = 6
		}

		if cfg.RefreshRateLimit <= 0 {
			cfg.RefreshRateLimit = 1
		}

		return cfg
	}),
	fx.Decorate(
		fx.Annotate(
			func(loader security.RolePermissionsLoader, bus event.Bus) security.RolePermissionsLoader {
				if loader == nil {
					return nil
				}

				return security.NewCachedRolePermissionsLoader(loader, bus)
			},
			fx.ParamTags(`optional:"true"`),
		),
	),
	fx.Provide(
		password.NewBcryptEncoder,
		newLoginGuard,
		fx.Annotate(
			newPasswordValidator,
			fx.ParamTags(``, ``, `optional:"true"`),
		),
		newJWT,
		fx.Annotate(
			newTokenAuthenticators,
			fx.ParamTags(``, ``, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators,flatten"`),
		),
		NewJWTTokenGenerator,
		NewOpaqueTokenGenerator,
		fx.Annotate(
			security.NewSessionRevocationNotifier,
			fx.ParamTags(`group:"vef:security:session_revocation_listeners"`),
		),
		newSessionStore,
		newNonceStore,
		newSessionPolicy,
		newTokenGenerator,
		security.NewJWTChallengeTokenStore,
		fx.Annotate(
			NewSignatureAuthenticator,
			fx.ParamTags(`optional:"true"`, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewPasswordAuthenticator,
			fx.ParamTags(`optional:"true"`, `optional:"true"`, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewAuthManager,
			fx.ParamTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewRBACPermissionChecker,
			fx.ParamTags(`optional:"true"`),
		),
		fx.Annotate(
			NewRBACDataPermissionResolver,
			fx.ParamTags(`optional:"true"`),
		),
		fx.Annotate(
			NewAuthResource,
			fx.ResultTags(`group:"vef:api:resources"`),
		),
	),
)

// newLoginGuard builds the default in-memory brute-force guard for the login
// endpoint from configuration. It returns nil when lockout is disabled (the
// AuthResource treats a nil guard as "no protection"), and fails fast on an
// out-of-enum strategy or key so a config typo surfaces at boot. Multi-node
// deployments override this with security.NewRedisLoginGuard via fx.Decorate so
// the failure counters are shared across nodes.
func newLoginGuard(cfg *config.SecurityConfig) (security.LoginGuard, error) {
	// Validate unconditionally so a typo'd strategy/key surfaces at boot even
	// when lockout is currently disabled (the operator may flip it on later).
	if err := cfg.Lockout.Validate(); err != nil {
		return nil, err
	}

	if !cfg.Lockout.IsEnabled() {
		return nil, nil
	}

	return security.NewMemoryLoginGuard(security.LockoutPolicy{
		MaxFailures:  cfg.Lockout.EffectiveMaxFailures(),
		Window:       cfg.Lockout.EffectiveWindow(),
		LockDuration: cfg.Lockout.EffectiveLockDuration(),
		Strategy:     security.LockoutStrategy(cfg.Lockout.EffectiveStrategy()),
		BackoffBase:  cfg.Lockout.EffectiveBackoffBase(),
		BackoffMax:   cfg.Lockout.EffectiveBackoffMax(),
		Key:          security.LockoutKey(cfg.Lockout.EffectiveKey()),
	}), nil
}

// newPasswordValidator builds the config-backed password validator injected into
// password-setting flows (e.g. the forced-change challenge): strength rules,
// plus a history-reuse check when a PasswordHistoryStore is registered and
// history_depth > 0. Every rule is opt-in, so with no policy configured the
// validator accepts any password, preserving zero-config behavior. Applications
// can inject the resulting security.PasswordValidator into their own flows.
func newPasswordValidator(
	cfg *config.SecurityConfig,
	encoder password.Encoder,
	historyStore security.PasswordHistoryStore,
) security.PasswordValidator {
	policy := cfg.PasswordPolicy

	validators := []security.PasswordValidator{newStrengthValidator(policy)}

	if historyStore != nil && policy.HistoryDepth > 0 {
		validators = append(validators, security.NewHistoryValidator(historyStore, encoder, policy.HistoryDepth))
	}

	return security.NewChainValidator(validators...)
}

// newStrengthValidator assembles the opt-in strength rules from config.
func newStrengthValidator(policy config.PasswordPolicyConfig) security.PasswordValidator {
	var rules []security.PasswordRule
	if policy.MinLength > 0 {
		rules = append(rules, security.NewMinLengthRule(policy.MinLength))
	}

	if policy.MaxLength > 0 {
		rules = append(rules, security.NewMaxLengthRule(policy.MaxLength))
	}

	if policy.RequireUpper || policy.RequireLower || policy.RequireDigit || policy.RequireSymbol || policy.MinCharClasses > 0 {
		rules = append(rules, security.NewCharacterClassRule(
			policy.RequireUpper,
			policy.RequireLower,
			policy.RequireDigit,
			policy.RequireSymbol,
			policy.MinCharClasses,
		))
	}

	if policy.DisallowUsername {
		rules = append(rules, security.NewDisallowIdentityRule())
	}

	if len(policy.Blocklist) > 0 {
		rules = append(rules, security.NewBlocklistRule(policy.Blocklist))
	}

	return security.NewRuleBasedValidator(rules...)
}

// newSessionStore provides the default in-memory opaque-token session store.
// Multi-node deployments override it with security.NewRedisSessionStore via
// fx.Decorate so sessions are shared across nodes.
func newSessionStore() security.SessionStore {
	return security.NewMemorySessionStore()
}

// newNonceStore provides the default in-memory replay-protection nonce store,
// shared by every framework signature verifier (the API signature
// authenticator and the integration inbound signature scheme). Multi-node
// deployments override it with security.NewRedisNonceStore via fx.Decorate so
// nonces are shared across nodes and a request cannot be replayed against a
// second node inside the timestamp tolerance.
func newNonceStore() security.NonceStore {
	return security.NewMemoryNonceStore()
}

// newSessionPolicy resolves the opaque-token session behavior from config.
func newSessionPolicy(cfg *config.SecurityConfig) security.SessionPolicy {
	session := cfg.Session

	return security.SessionPolicy{
		MaxConcurrent: session.MaxConcurrent,
		OnExceed:      security.SessionExceedPolicy(session.EffectiveOnExceed()),
		IdleTTL:       session.EffectiveIdleTTL(),
		MaxLifetime:   session.EffectiveMaxLifetime(),
		Sliding:       session.IsSliding(),
	}
}

// newTokenAuthenticators registers only the configured login-token mechanism's
// authenticators: the JWT access + refresh pair under jwt_token, the opaque
// session authenticator under opaque_token. Keeping the inactive mechanism out
// of the authenticator group closes its surfaces entirely — after a deployment
// switches to opaque tokens, a leftover JWT (access or refresh) can no longer
// authenticate a request or mint fresh sessions through the refresh flow.
func newTokenAuthenticators(
	cfg *config.SecurityConfig,
	jwt *security.JWT,
	userLoader security.UserLoader,
	store security.SessionStore,
	policy security.SessionPolicy,
) []security.Authenticator {
	if cfg.EffectiveTokenType() == config.TokenTypeOpaque {
		return []security.Authenticator{NewOpaqueTokenAuthenticator(store, policy)}
	}

	return []security.Authenticator{
		NewJWTAuthenticator(jwt),
		NewJWTRefreshAuthenticator(jwt, userLoader),
	}
}

// newTokenGenerator selects the active login-token mechanism from
// vef.security.token_type, validating the token-type and session config so a
// typo fails fast at boot. Both underlying generators are always constructed;
// only the configured one issues login tokens.
func newTokenGenerator(
	cfg *config.SecurityConfig,
	jwtGenerator *JWTTokenGenerator,
	opaqueGenerator *OpaqueTokenGenerator,
) (security.TokenGenerator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.EffectiveTokenType() == config.TokenTypeOpaque {
		return opaqueGenerator, nil
	}

	return jwtGenerator, nil
}

// newJWT builds the JWT signer from configuration. It never silently falls back
// to the built-in public DefaultJWTSecret: an unset secret yields an ephemeral
// per-process key (with a warning), which keeps development zero-config while
// forcing production to set vef.security.secret.
func newJWT(appCfg *config.AppConfig, secCfg *config.SecurityConfig) (*security.JWT, error) {
	secret := secCfg.Secret

	switch secret {
	case "":
		generated, err := security.GenerateSecret()
		if err != nil {
			return nil, err
		}

		secret = generated

		logger.Warnf("vef.security.secret is not set; generated an ephemeral signing key. Tokens will not survive a restart or work across nodes — set vef.security.secret in production.")

	case security.DefaultJWTSecret:
		logger.Warnf("vef.security.secret is the built-in public default; replace it with a private key in production.")
	}

	return security.NewJWT(&security.JWTConfig{
		Secret:   secret,
		Audience: lo.SnakeCase(appCfg.Name),
	})
}
