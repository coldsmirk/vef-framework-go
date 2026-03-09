package middleware

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
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

	strategy, found := m.registry.Get(op.Auth.Strategy)
	if !found {
		contextx.Logger(ctx).Errorf("Authentication failed: %v, strategy=%s", ErrAuthStrategyNotFound, op.Auth.Strategy)

		return fiber.ErrUnauthorized
	}

	principal, err := strategy.Authenticate(ctx, op.Auth.Options)
	if err != nil {
		return err
	}

	contextx.SetPrincipal(ctx, principal)
	ctx.SetContext(contextx.SetPrincipal(ctx.Context(), principal))

	return m.checkPermission(ctx, op, principal)
}

func (m *Auth) checkPermission(ctx fiber.Ctx, op *api.Operation, principal *security.Principal) error {
	if principal.Type == security.PrincipalTypeSystem {
		return ctx.Next()
	}

	if permToken := permTokenFromOperation(op); permToken != "" {
		if err := m.doCheck(ctx.Context(), principal, permToken); err != nil {
			return err
		}
	}

	return ctx.Next()
}

func (m *Auth) doCheck(ctx context.Context, principal *security.Principal, permToken string) error {
	if m.checker == nil {
		return fmt.Errorf(
			"%w: %w, permission=%q",
			fiber.ErrForbidden, ErrPermissionCheckerNotProvided, permToken,
		)
	}

	granted, err := m.checker.HasPermission(ctx, principal, permToken)
	if err != nil {
		return fmt.Errorf(
			"%w: %w, principal=%q, permission=%q: %w",
			fiber.ErrForbidden, ErrPermissionCheckFailed, principal.ID, permToken, err,
		)
	}

	if !granted {
		return fmt.Errorf(
			"%w: %w, principal=%q (type=%s), permission=%q",
			fiber.ErrForbidden, ErrPermissionDenied, principal.ID, principal.Type, permToken,
		)
	}

	return nil
}

// permTokenFromOperation extracts the permission token from an operation's auth options.
func permTokenFromOperation(op *api.Operation) string {
	if token, ok := op.Auth.Options[shared.AuthOptionPermToken].(string); ok {
		return token
	}

	return ""
}
