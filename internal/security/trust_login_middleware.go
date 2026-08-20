package security

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/fiberx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Query parameters of the trust-login handoff URL. They deliberately use the
// same spelling as the keys of the signed payload, so the signing rule an
// external system implements is one sentence: sign every parameter you send
// except the signature itself, plus the request's method and path.
const (
	queryTrustAppID     = "app_id"
	queryTrustUserID    = "user_id"
	queryTrustRedirect  = "redirect"
	queryTrustTimestamp = "timestamp"
	queryTrustNonce     = "nonce"
	queryTrustSignature = "signature"

	// queryTrustCode carries the issued code on the redirect back to the SPA.
	queryTrustCode = "code"
)

// trustHandoff is one verified handoff, resolved from the request before any
// code is issued.
type trustHandoff struct {
	appID     string
	principal *security.Principal
	redirect  *url.URL
}

// TrustLoginMiddleware mounts the trust-login gateway: the browser-facing route
// an external system links to, carrying a signed user identifier, which trades
// the handoff for a one-time code the SPA then redeems through the ordinary
// login endpoint.
//
// Two legs rather than one is what keeps the signed URL harmless. The signature
// never reaches the browser's own code, an invalid one is refused at the
// gateway where a person can still be shown why, and the URL that does land in
// browser history carries only a code that is single-use and expires in
// seconds.
type TrustLoginMiddleware struct {
	apps     security.ExternalAppLoader
	resolver security.TrustUserResolver
	users    security.UserLoader
	codes    security.TrustCodeStore
	verifier *security.Signature
	limit    fiber.Handler
	cfg      config.TrustLoginConfig
}

// TrustLoginMiddlewareParams contains dependencies for the trust-login gateway.
type TrustLoginMiddlewareParams struct {
	fx.In

	Apps     security.ExternalAppLoader `optional:"true"`
	Resolver security.TrustUserResolver `optional:"true"`
	Users    security.UserLoader        `optional:"true"`
	Codes    security.TrustCodeStore
	Nonces   security.NonceStore `optional:"true"`
	Security *config.SecurityConfig
}

// NewTrustLoginMiddleware creates the trust-login gateway. It returns nil while
// the feature is disabled, and fails the boot when it is enabled without the
// collaborators it cannot work without — an unauthenticatable gateway that
// answers 401 to every handoff is far harder to diagnose than a refused start.
func NewTrustLoginMiddleware(params TrustLoginMiddlewareParams) (app.Middleware, error) {
	cfg := params.Security.TrustLogin
	if !cfg.Enabled {
		return nil, nil
	}

	if params.Apps == nil {
		return nil, ErrTrustLoginExternalAppLoaderMissing
	}

	if params.Resolver == nil && params.Users == nil {
		return nil, ErrTrustLoginUserResolutionMissing
	}

	// One long-lived verifier so replay protection is real: the nonce store is
	// shared process-wide (and cluster-wide when decorated with the Redis one)
	// rather than rebuilt per request. The per-app secret is supplied at
	// verification time, so the placeholder secret never computes an HMAC.
	var options []security.SignatureOption
	if params.Nonces != nil {
		options = append(options, security.WithNonceStore(params.Nonces))
	}

	verifier, err := security.NewSignature(signatureVerifierPlaceholderSecret, options...)
	if err != nil {
		return nil, err
	}

	middleware := &TrustLoginMiddleware{
		apps:     params.Apps,
		resolver: params.Resolver,
		users:    params.Users,
		codes:    params.Codes,
		verifier: verifier,
		cfg:      cfg,
	}
	// The gateway is public and unauthenticated, and it resolves the app
	// through ExternalAppLoader — typically a database round trip — before it
	// can reject anything. Counting per (app ID, client IP) keeps one flooding
	// source from starving the other apps, mirroring the integration inbound
	// gateway, the framework's other public non-/api route.
	middleware.limit = limiter.New(limiter.Config{
		LimiterMiddleware: limiter.SlidingWindow{},
		Max:               cfg.RateLimit.EffectiveMax(),
		Expiration:        cfg.RateLimit.EffectivePeriod(),
		KeyGenerator: func(ctx fiber.Ctx) string {
			return ctx.Query(queryTrustAppID) + ":" + fiberx.GetIP(ctx)
		},
		LimitReached: func(ctx fiber.Ctx) error {
			return middleware.reject(ctx, result.ErrTooManyRequests)
		},
	})

	return middleware, nil
}

func (*TrustLoginMiddleware) Name() string {
	return "trust-login"
}

// Order places the gateway alongside the framework's other real routes, after
// the API engine and before the SPA fallback.
func (*TrustLoginMiddleware) Order() int {
	return 460
}

func (m *TrustLoginMiddleware) Apply(router fiber.Router) {
	router.Get(m.cfg.EffectivePath(), m.limit, m.handle)
	logger.Infof("Trust login gateway registered at %s for %d external app(s)", m.cfg.EffectivePath(), len(m.cfg.Apps))
}

// handle verifies a handoff and redirects the browser to the requested target
// carrying a freshly issued code.
func (m *TrustLoginMiddleware) handle(ctx fiber.Ctx) error {
	// This route lives outside /api, so nothing has populated the request
	// metadata the app policy check reads. Populate it here — an app's IP
	// whitelist must apply to the gateway exactly as it applies to the API.
	ctx.SetContext(contextx.SetRequestIP(ctx.Context(), fiberx.GetIP(ctx)))

	handoff, err := m.verify(ctx)
	if err != nil {
		return m.reject(ctx, err)
	}

	code, err := m.codes.Issue(ctx.Context(), security.TrustCodeState{
		Principal: handoff.principal,
		AppID:     handoff.appID,
		UserAgent: strings.Clone(ctx.Get(fiber.HeaderUserAgent)),
		ClientIP:  fiberx.GetIP(ctx),
	}, m.cfg.EffectiveCodeTTL())
	if err != nil {
		logger.Errorf("Trust login failed to issue a code for app %q: %v", handoff.appID, err)

		return m.reject(ctx, security.ErrTrustAuthFailed)
	}

	logger.Infof("Trust login issued a code for app %q, user %q", handoff.appID, handoff.principal.ID)

	// The Location header carries a bearer code, and the request URL it answers
	// carries the signature. no-store keeps the redirect out of any cache that
	// would hold the code past its TTL; no-referrer applies to the rest of the
	// redirect chain, so the signed handoff URL is not handed to the
	// destination as a Referer.
	ctx.Set(fiber.HeaderCacheControl, "no-store")
	ctx.Set(fiber.HeaderReferrerPolicy, "no-referrer")

	return ctx.Redirect().Status(fiber.StatusFound).To(withTrustCode(handoff.redirect, handoff.appID, code))
}

// verify runs the handoff through every gate in the order that leaks least:
// authenticate first, so only a caller that already holds the app's secret can
// learn anything from the redirect and user-resolution verdicts.
func (m *TrustLoginMiddleware) verify(ctx fiber.Ctx) (*trustHandoff, error) {
	// Fiber runs with Immutable off, so a query value is a view into the pooled
	// request buffer and stops being the caller's data the moment this request
	// ends. Every value below outlives the request — parked in the code store,
	// or handed to an application-supplied resolver that may keep it — so each
	// is copied out here, once, at the boundary.
	appID := strings.Clone(ctx.Query(queryTrustAppID))
	userID := strings.Clone(ctx.Query(queryTrustUserID))
	redirect := strings.Clone(ctx.Query(queryTrustRedirect))

	policy, ok := m.cfg.Apps[appID]
	if !ok {
		logger.Warnf("Trust login rejected: app %q is not configured under vef.security.trust_login.apps", appID)

		return nil, security.ErrTrustAuthFailed
	}

	if err := m.verifySignature(ctx, appID, userID, redirect); err != nil {
		return nil, err
	}

	target, err := resolveRedirect(redirect, policy.RedirectURLs)
	if err != nil {
		logger.Warnf("Trust login rejected for app %q: redirect %q matches no allowlist entry", appID, redirect)

		return nil, err
	}

	principal, err := m.resolveUser(ctx.Context(), appID, userID)
	if err != nil {
		return nil, err
	}

	return &trustHandoff{appID: appID, principal: principal, redirect: target}, nil
}

// verifySignature authenticates the handoff against the app's secret. Every
// failure — unknown or disabled app, blocked source address, malformed,
// expired, replayed or simply wrong signature — collapses into
// ErrTrustAuthFailed; the detail stays in the log.
func (m *TrustLoginMiddleware) verifySignature(ctx fiber.Ctx, appID, userID, redirect string) error {
	principal, secret, err := m.apps.LoadByID(ctx.Context(), appID)
	if err != nil {
		logger.Warnf("Trust login rejected: loading app %q failed: %v", appID, err)

		return security.ErrTrustAuthFailed
	}

	if principal == nil || secret == "" {
		logger.Warnf("Trust login rejected: app %q is unknown to the ExternalAppLoader", appID)

		return security.ErrTrustAuthFailed
	}

	if err := validateExternalAppPolicy(ctx.Context(), principal); err != nil {
		logger.Warnf("Trust login rejected for app %q: %v", appID, err)

		return security.ErrTrustAuthFailed
	}

	timestamp, err := strconv.ParseInt(ctx.Query(queryTrustTimestamp), 10, 64)
	if err != nil {
		logger.Warnf("Trust login rejected for app %q: malformed timestamp", appID)

		return security.ErrTrustAuthFailed
	}

	err = m.verifier.VerifyWithSecret(ctx.Context(), secret,
		security.SignatureRequest{
			AppID:       appID,
			Method:      ctx.Method(),
			Path:        ctx.Path(),
			BoundParams: map[string]string{queryTrustUserID: userID, queryTrustRedirect: redirect},
		},
		security.SignatureCredentials{
			Timestamp: timestamp,
			Nonce:     ctx.Query(queryTrustNonce),
			Signature: ctx.Query(queryTrustSignature),
		})
	if err != nil {
		logger.Warnf("Trust login rejected for app %q: %v", appID, err)

		return security.ErrTrustAuthFailed
	}

	return nil
}

// resolveUser maps the external identifier onto a local principal, through the
// registered TrustUserResolver or — when none is registered — the UserLoader,
// which is already correct when both systems key users by the same identifier.
func (m *TrustLoginMiddleware) resolveUser(ctx context.Context, appID, userID string) (*security.Principal, error) {
	var (
		principal *security.Principal
		err       error
	)

	if m.resolver != nil {
		principal, err = m.resolver.ResolveUser(ctx, appID, userID)
	} else {
		principal, err = m.users.LoadByID(ctx, userID)
	}

	if err != nil {
		logger.Warnf("Trust login rejected for app %q: resolving user %q failed: %v", appID, userID, err)

		return nil, security.ErrTrustUserNotResolved
	}

	if principal == nil {
		logger.Warnf("Trust login rejected for app %q: no local user matches %q", appID, userID)

		return nil, security.ErrTrustUserNotResolved
	}

	// A resolver is an application extension point, and the code it feeds is
	// redeemed by an authenticator the AuthManager's own reserved-identity gate
	// never sees. Refuse here, at the point of issue.
	if principal.IsReserved() {
		logger.Errorf("Trust login rejected: user resolution for app %q returned the framework-reserved identity %q", appID, principal.ID)

		return nil, security.ErrTrustUserNotResolved
	}

	return principal, nil
}

// reject renders a gateway failure for a browser: the error's own status with
// its message as plain text. The endpoint is navigated to directly, so a JSON
// result envelope would land in the address bar as a wall of punctuation.
func (*TrustLoginMiddleware) reject(ctx fiber.Ctx, err error) error {
	resErr, ok := result.AsErr(err)
	if !ok {
		return err
	}

	return ctx.Status(resErr.Status).SendString(resErr.Message)
}

// resolveRedirect parses the requested target and admits it only when one
// allowlist entry covers it. Entries are configuration, already validated as
// absolute URLs at boot.
func resolveRedirect(redirect string, allowed []string) (*url.URL, error) {
	target, err := url.Parse(redirect)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, security.ErrTrustRedirectNotAllowed
	}

	for _, entry := range allowed {
		allowedURL, err := url.Parse(entry)
		if err != nil {
			continue
		}

		if strings.EqualFold(target.Scheme, allowedURL.Scheme) &&
			strings.EqualFold(target.Host, allowedURL.Host) &&
			pathCovers(allowedURL.Path, target.Path) {
			return target, nil
		}
	}

	return nil, security.ErrTrustRedirectNotAllowed
}

// pathCovers reports whether an allowlist entry's path admits a target path.
// The comparison runs on the cleaned target so "/app/../admin" is judged as
// "/admin", and matches whole segments so "/app" never covers "/application".
func pathCovers(entryPath, targetPath string) bool {
	entryPath = strings.TrimSuffix(entryPath, "/")
	if entryPath == "" {
		return true
	}

	cleaned := path.Clean("/" + strings.TrimPrefix(targetPath, "/"))

	return cleaned == entryPath || strings.HasPrefix(cleaned, entryPath+"/")
}

// withTrustCode appends the issued code and the app that earned it to the
// redirect target, preserving whatever query the target already carried.
func withTrustCode(target *url.URL, appID, code string) string {
	query := target.Query()
	query.Set(queryTrustAppID, appID)
	query.Set(queryTrustCode, code)

	redirect := *target
	redirect.RawQuery = query.Encode()

	return redirect.String()
}
