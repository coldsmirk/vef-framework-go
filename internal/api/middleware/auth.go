package middleware

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/fiberx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/security"
)

type Auth struct {
	registry api.AuthStrategyRegistry
	checker  security.PermissionChecker
}

// NewAuth creates a new authentication middleware.
func NewAuth(registry api.AuthStrategyRegistry, checker security.PermissionChecker) api.Middleware {
	return &Auth{
		registry: registry,
		checker:  checker,
	}
}

// Name returns the middleware name.
func (*Auth) Name() string {
	return "auth"
}

// Order returns the middleware order.
// Authentication runs first in the middleware chain.
func (*Auth) Order() int {
	return -100
}

// Process handles authentication and permission checking.
func (m *Auth) Process(ctx fiber.Ctx) error {
	op := shared.Operation(ctx)
	if op == nil {
		contextx.Logger(ctx).Errorf("Authentication failed: %v", ErrOperationNotFound)

		return fiber.ErrUnauthorized
	}

	// Make the resolved client IP and the request method/path available to
	// authenticators via the request context: the signature authenticator
	// uses the IP for its whitelist and binds the method+path into the HMAC.
	reqCtx := contextx.SetRequestIP(ctx.Context(), fiberx.GetIP(ctx))
	reqCtx = contextx.SetRequestMethod(reqCtx, ctx.Method())
	reqCtx = contextx.SetRequestPath(reqCtx, ctx.Path())
	ctx.SetContext(reqCtx)

	strategy, found := m.registry.Get(op.Auth.Strategy)
	if !found {
		contextx.Logger(ctx).Errorf("Authentication failed: %v, strategy=%s", ErrAuthStrategyNotFound, op.Auth.Strategy)

		return fiber.ErrUnauthorized
	}

	principal, err := strategy.Authenticate(ctx, op.Auth.Options)
	if err != nil {
		return err
	}

	// An auth strategy is an application extension point, so its result crosses a
	// trust boundary. A framework-reserved identity must never arrive this way:
	// the system type skips the permission check below outright.
	if principal == nil || principal.IsReserved() {
		contextx.Logger(ctx).Errorf(
			"Authentication rejected: strategy %q returned a nil or framework-reserved principal",
			op.Auth.Strategy,
		)

		return security.ErrPrincipalInvalid(i18n.T("security_reserved_principal_forbidden"))
	}

	contextx.SetPrincipal(ctx, principal)
	ctx.SetContext(contextx.SetPrincipal(ctx.Context(), principal))

	return m.checkPermission(ctx, op, principal)
}

func (m *Auth) checkPermission(ctx fiber.Ctx, op *api.Operation, principal *security.Principal) error {
	// The system identity is the framework's own author for work it performs
	// outside any request, so it is exempt. This is safe only because
	// authentication refuses to produce it — see the reserved-identity guard in
	// Process; without that guard this branch is an unauthenticated backdoor.
	if principal.Type == security.PrincipalTypeSystem {
		return ctx.Next()
	}

	if permission := requiredPermissionFromOperation(op); permission != "" {
		if err := m.doCheck(ctx.Context(), principal, permission); err != nil {
			return err
		}
	}

	return ctx.Next()
}

func (m *Auth) doCheck(ctx context.Context, principal *security.Principal, permission string) error {
	if m.checker == nil {
		return fmt.Errorf(
			"%w: %w, permission=%q",
			fiber.ErrForbidden, ErrPermissionCheckerNotProvided, permission,
		)
	}

	granted, err := m.checker.HasPermission(ctx, principal, permission)
	if err != nil {
		return fmt.Errorf(
			"%w: %w, principal=%q, permission=%q: %w",
			fiber.ErrForbidden, ErrPermissionCheckFailed, principal.ID, permission, err,
		)
	}

	if !granted {
		return fmt.Errorf(
			"%w: %w, principal=%q (type=%s), permission=%q",
			fiber.ErrForbidden, ErrPermissionDenied, principal.ID, principal.Type, permission,
		)
	}

	return nil
}

// requiredPermissionFromOperation extracts the required permission token from an operation's auth options.
func requiredPermissionFromOperation(op *api.Operation) string {
	if token, ok := op.Auth.Options[shared.AuthOptionRequiredPermission].(string); ok {
		return token
	}

	return ""
}
