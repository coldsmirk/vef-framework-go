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
		newJWT,
		fx.Annotate(
			NewJWTAuthenticator,
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		fx.Annotate(
			NewJWTRefreshAuthenticator,
			fx.ParamTags(``, `optional:"true"`),
			fx.ResultTags(`group:"vef:security:authenticators"`),
		),
		NewJWTTokenGenerator,
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
	if !cfg.Lockout.IsEnabled() {
		return nil, nil
	}

	if err := cfg.Lockout.Validate(); err != nil {
		return nil, err
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
