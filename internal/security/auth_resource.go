package security

import (
	"cmp"
	"context"
	"slices"

	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// AuthResourceParams holds the dependencies for AuthResource construction.
type AuthResourceParams struct {
	fx.In

	AuthManager         security.AuthManager
	TokenGenerator      security.TokenGenerator
	ChallengeTokenStore security.ChallengeTokenStore
	UserInfoLoader      security.UserInfoLoader      `optional:"true"`
	ChallengeProviders  []security.ChallengeProvider `group:"vef:security:challenge_providers"`
	Bus                 event.Bus
	SecurityConfig      *config.SecurityConfig
}

// NewAuthResource creates a new authentication resource with the provided auth manager and token generator.
func NewAuthResource(params AuthResourceParams) api.Resource {
	slices.SortFunc(params.ChallengeProviders, func(a, b security.ChallengeProvider) int {
		return cmp.Compare(a.Order(), b.Order())
	})

	return &AuthResource{
		authManager:         params.AuthManager,
		tokenGenerator:      params.TokenGenerator,
		challengeTokenStore: params.ChallengeTokenStore,
		userInfoLoader:      params.UserInfoLoader,
		challengeProviders:  params.ChallengeProviders,
		bus:                 params.Bus,

		Resource: api.NewRPCResource(
			"security/auth",
			api.WithOperations(
				api.OperationSpec{
					Action:    "login",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.LoginRateLimit},
				},
				api.OperationSpec{
					Action:    "refresh",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.RefreshRateLimit},
				},
				api.OperationSpec{
					Action: "logout",
				},
				api.OperationSpec{
					Action:    "resolve_challenge",
					Public:    true,
					RateLimit: &api.RateLimitConfig{Max: params.SecurityConfig.LoginRateLimit},
				},
				api.OperationSpec{
					Action: "get_user_info",
				},
			),
		),
	}
}

// AuthResource handles authentication-related API endpoints.
type AuthResource struct {
	api.Resource

	authManager         security.AuthManager
	tokenGenerator      security.TokenGenerator
	challengeTokenStore security.ChallengeTokenStore
	userInfoLoader      security.UserInfoLoader
	challengeProviders  []security.ChallengeProvider
	bus                 event.Bus
}

// LoginParams represents the request parameters for user login.
type LoginParams struct {
	api.P

	Type        string `json:"type" validate:"required" label_i18n:"auth_type"`
	Principal   string `json:"principal" validate:"required" label_i18n:"auth_principal"`
	Credentials any    `json:"credentials" validate:"required" label_i18n:"auth_credentials"`
}

// Login authenticates a user and returns a LoginResult.
// When challenge providers are configured and applicable, the result contains
// a challenge token and pending challenges instead of auth tokens.
func (a *AuthResource) Login(ctx fiber.Ctx, params LoginParams) error {
	principal, err := a.authManager.Authenticate(ctx.Context(), security.Authentication{
		Type:        params.Type,
		Principal:   params.Principal,
		Credentials: params.Credentials,
	})
	if err != nil {
		a.publishLoginFailure(ctx, params.Type, params.Principal, err)

		return err
	}

	pending := streams.MapTo(
		streams.FromSlice(a.challengeProviders),
		func(p security.ChallengeProvider) string { return p.Type() },
	).Collect()

	challenge, pending, err := a.evaluateNextChallenge(ctx.Context(), principal, pending)
	if err != nil {
		return err
	}

	if challenge != nil {
		challengeToken, err := a.challengeTokenStore.Generate(ctx.Context(), principal, params.Principal, pending, nil)
		if err != nil {
			return err
		}

		return result.Ok(&security.LoginResult{
			ChallengeToken: challengeToken,
			Challenge:      challenge,
		}).Response(ctx)
	}

	tokens, err := a.tokenGenerator.Generate(principal)
	if err != nil {
		return err
	}

	a.publishLoginSuccess(ctx, params.Type, params.Principal, principal)

	return result.Ok(&security.LoginResult{Tokens: tokens}).Response(ctx)
}

// RefreshParams represents the request parameters for token refresh operation.
type RefreshParams struct {
	api.P

	RefreshToken string `json:"refreshToken" validate:"required" label_i18n:"auth_refresh_token"`
}

// Refresh refreshes the access token using a valid refresh token.
// User data reload logic is handled by JwtRefreshAuthenticator.
func (a *AuthResource) Refresh(ctx fiber.Ctx, params RefreshParams) error {
	principal, err := a.authManager.Authenticate(ctx.Context(), security.Authentication{
		Type:      AuthTypeRefresh,
		Principal: params.RefreshToken,
	})
	if err != nil {
		return err
	}

	credentials, err := a.tokenGenerator.Generate(principal)
	if err != nil {
		return err
	}

	return result.Ok(credentials).Response(ctx)
}

// Logout returns success immediately.
// Token invalidation should be handled on the client side by removing stored tokens.
func (*AuthResource) Logout(ctx fiber.Ctx) error {
	return result.Ok().Response(ctx)
}

// ResolveChallengeParams represents the request for resolving a login challenge.
type ResolveChallengeParams struct {
	api.P

	ChallengeToken string `json:"challengeToken" validate:"required" label_i18n:"auth_challenge_token"`
	Type           string `json:"type" validate:"required" label_i18n:"auth_challenge_type"`
	Response       any    `json:"response" validate:"required" label_i18n:"auth_challenge_response"`
}

// ResolveChallenge validates a user's response to a login challenge.
// On success, either issues real auth tokens (all challenges resolved)
// or evaluates the next challenge sequentially.
//
// A ChallengeProvider may reject a response by returning a typed result.Error
// (e.g. security.ErrOTPCodeInvalid) to control the client-facing code; a bare
// error is normalized to security.ErrChallengeResolveFailed (code
// security.ErrCodeChallengeResolveFailed).
func (a *AuthResource) ResolveChallenge(ctx fiber.Ctx, params ResolveChallengeParams) error {
	state, err := a.challengeTokenStore.Parse(ctx.Context(), params.ChallengeToken)
	if err != nil {
		return security.ErrChallengeTokenInvalid
	}

	if len(state.Pending) == 0 || state.Pending[0] != params.Type {
		return security.ErrChallengeTypeInvalid
	}

	provider := a.findProvider(params.Type)
	if provider == nil {
		return security.ErrChallengeTypeInvalid
	}

	// provider.Resolve is the second-factor analog of authManager.Authenticate:
	// a rejection here is a genuine failed credential attempt, so audit it like
	// a failed login. The earlier guards (invalid/expired token, wrong type) are
	// protocol/tampering errors that Login's analogous infra paths do not audit,
	// so they are deliberately left unaudited.
	principal, err := provider.Resolve(ctx.Context(), state.Principal, params.Response)
	if err != nil {
		// Providers that return a typed result.Error keep their chosen code
		// (e.g. ErrOTPCodeInvalid); a bare error is normalized to the stable
		// challenge-resolve-failed code so the framework never leaks an opaque
		// 500 from a custom ChallengeProvider.
		if _, ok := result.AsErr(err); !ok {
			err = security.ErrChallengeResolveFailed
		}

		a.publishLoginFailure(ctx, params.Type, state.Username, err)

		return err
	}

	resolved := append(state.Resolved, params.Type)
	remaining := state.Pending[1:]

	challenge, remaining, err := a.evaluateNextChallenge(ctx.Context(), principal, remaining)
	if err != nil {
		return err
	}

	if challenge != nil {
		challengeToken, err := a.challengeTokenStore.Generate(ctx.Context(), principal, state.Username, remaining, resolved)
		if err != nil {
			return err
		}

		return result.Ok(&security.LoginResult{
			ChallengeToken: challengeToken,
			Challenge:      challenge,
		}).Response(ctx)
	}

	tokens, err := a.tokenGenerator.Generate(principal)
	if err != nil {
		return err
	}

	a.publishLoginSuccess(ctx, params.Type, state.Username, principal)

	return result.Ok(&security.LoginResult{Tokens: tokens}).Response(ctx)
}

// GetUserInfo retrieves user information via UserInfoLoader.
// Requires a UserInfoLoader implementation to be provided.
func (a *AuthResource) GetUserInfo(ctx fiber.Ctx, principal *security.Principal, params api.Params) error {
	if a.userInfoLoader == nil {
		return result.ErrNotImplemented(i18n.T(security.ErrMessageUserInfoLoaderNotImplemented))
	}

	userInfo, err := a.userInfoLoader.LoadUserInfo(ctx.Context(), principal, params)
	if err != nil {
		return err
	}

	return result.Ok(userInfo).Response(ctx)
}

// publishLoginSuccess publishes a successful-login audit event. username is the
// original login identifier (threaded through the challenge state on MFA flows)
// so success events carry the same identifier regardless of whether a challenge
// was involved.
func (a *AuthResource) publishLoginSuccess(ctx fiber.Ctx, authType, username string, principal *security.Principal) {
	loginEvent := security.NewLoginEvent(security.LoginEventParams{
		AuthType:  authType,
		UserID:    &principal.ID,
		Username:  username,
		LoginIP:   httpx.GetIP(ctx),
		UserAgent: ctx.Get(fiber.HeaderUserAgent),
		TraceID:   contextx.RequestID(ctx),
		IsOk:      true,
	})
	_ = a.bus.Publish(ctx.Context(), loginEvent, event.WithAsync())
}

// publishLoginFailure publishes a failed-login audit event, deriving the failure
// reason and business code from err. It mirrors publishLoginSuccess so password
// and challenge-step failures land in the same audit pipeline with the same
// username semantics.
func (a *AuthResource) publishLoginFailure(ctx fiber.Ctx, authType, username string, err error) {
	failReason := err.Error()
	errorCode := result.ErrCodeUnknown

	if resErr, ok := result.AsErr(err); ok {
		failReason = resErr.Message
		errorCode = resErr.Code
	}

	loginEvent := security.NewLoginEvent(security.LoginEventParams{
		AuthType:   authType,
		Username:   username,
		LoginIP:    httpx.GetIP(ctx),
		UserAgent:  ctx.Get(fiber.HeaderUserAgent),
		TraceID:    contextx.RequestID(ctx),
		IsOk:       false,
		FailReason: failReason,
		ErrorCode:  errorCode,
	})
	_ = a.bus.Publish(ctx.Context(), loginEvent, event.WithAsync())
}

// findProvider returns the challenge provider matching the given type, or nil.
func (a *AuthResource) findProvider(challengeType string) security.ChallengeProvider {
	return streams.FromSlice(a.challengeProviders).
		FindFirst(func(cp security.ChallengeProvider) bool {
			return cp.Type() == challengeType
		}).
		GetOrElse(nil)
}

// evaluateNextChallenge walks pending types sequentially and returns the first
// applicable challenge. Providers that return nil (challenge not needed) are
// skipped, and their types are removed from pending.
func (a *AuthResource) evaluateNextChallenge(ctx context.Context, principal *security.Principal, pending []string) (*security.LoginChallenge, []string, error) {
	for len(pending) > 0 {
		provider := a.findProvider(pending[0])
		if provider == nil {
			pending = pending[1:]

			continue
		}

		challenge, err := provider.Evaluate(ctx, principal)
		if err != nil {
			return nil, nil, err
		}

		if challenge != nil {
			return challenge, pending, nil
		}

		pending = pending[1:]
	}

	return nil, nil, nil
}
