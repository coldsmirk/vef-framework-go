package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// logger is the fallback when the request context carries no logger (the
// request-scoped logger, with its request metadata, always wins).
var logger = logx.Named("api.auth")

// IPStrategy implements api.AuthStrategy for source-IP whitelist
// authentication: the client IP is the credential. An operation opts in with
// api.IPAuth(name); the strategy resolves that name through the
// security.IPWhitelistLoader and matches the Fiber-resolved client IP against
// the returned entries. The validator is rebuilt per request because loader
// data may change between requests; loaders needing to amortize expensive
// lookups cache on their side.
//
// Every authentication failure denies with security.ErrIPNotAllowed (fail
// closed) and configuration faults are additionally logged server-side, so
// the 401 stays opaque to the caller. Loader errors propagate as-is: an
// unavailable backing store is an infrastructure fault, not a credential
// rejection.
type IPStrategy struct {
	loader security.IPWhitelistLoader
}

// NewIP creates a new source-IP whitelist authentication strategy. The loader
// is optional: when the application registers no security.IPWhitelistLoader,
// the strategy falls back to the configuration-backed loader serving
// vef.security.ip_whitelists (whose construction validates the configured
// whitelists eagerly and fails start-up on a fault).
func NewIP(loader security.IPWhitelistLoader, cfg *config.SecurityConfig) (api.AuthStrategy, error) {
	if loader == nil {
		var err error
		if loader, err = isecurity.NewConfigIPWhitelistLoader(cfg); err != nil {
			return nil, err
		}
	}

	return &IPStrategy{loader: loader}, nil
}

// Name returns the strategy name.
func (*IPStrategy) Name() string {
	return api.AuthStrategyIP
}

// Authenticate matches the client IP against the whitelist named in the
// operation's auth options and, on success, returns a synthesized external-app
// principal ("ip:<whitelist>") so audit logs carry a readable identity and
// permission checks still apply when the operation declares one.
func (s *IPStrategy) Authenticate(ctx fiber.Ctx, options map[string]any) (*security.Principal, error) {
	name, _ := options[api.AuthOptionWhitelist].(string)
	if name == "" {
		contextx.Logger(ctx, logger).Errorf("IP authentication failed: auth option %q is missing", api.AuthOptionWhitelist)

		return nil, security.ErrIPNotAllowed
	}

	whitelist, err := s.loader.LoadByName(ctx.Context(), name)
	if err != nil {
		return nil, err
	}

	if whitelist == nil {
		contextx.Logger(ctx, logger).Errorf("IP authentication failed: whitelist %q is not defined", name)

		return nil, security.ErrIPNotAllowed
	}

	validator := security.NewIPWhitelistValidatorFromEntries(whitelist.Entries)
	if validator.IsEmpty() {
		// An empty validator allows every IP, which would turn a misconfigured
		// whitelist into a public endpoint — deny instead.
		contextx.Logger(ctx, logger).Errorf("IP authentication failed: whitelist %q has no usable entries", name)

		return nil, security.ErrIPNotAllowed
	}

	if ip := httpx.GetIP(ctx); ip == "" || !validator.IsAllowed(ip) {
		return nil, security.ErrIPNotAllowed
	}

	return security.NewExternalApp("ip:"+name, name), nil
}
