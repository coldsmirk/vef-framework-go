package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/security"
)

// DataPermission handles data permission resolution.
// It resolves data scope for the current principal and permission token,
// then injects a RequestScopedDataPermApplier into the context.
type DataPermission struct {
	resolver security.DataPermissionResolver
}

// NewDataPermission creates a new data permission middleware.
func NewDataPermission(resolver security.DataPermissionResolver) api.Middleware {
	return &DataPermission{
		resolver: resolver,
	}
}

// Name returns the middleware name.
func (*DataPermission) Name() string {
	return "data_permission"
}

// Order returns the middleware order.
// Runs after authentication (-100) but before rate limiting (-80).
func (*DataPermission) Order() int {
	return -80
}

// Process handles the data permission resolution.
func (m *DataPermission) Process(ctx fiber.Ctx) error {
	op := shared.Operation(ctx)
	if op == nil {
		contextx.Logger(ctx).Errorf("Data permission check failed: %v", ErrOperationNotFound)

		return fiber.ErrUnauthorized
	}

	principal := contextx.Principal(ctx)
	if principal == nil {
		contextx.Logger(ctx).Errorf("Data permission check failed: %v", ErrPrincipalNotFound)

		return fiber.ErrUnauthorized
	}

	if principal.Type != security.PrincipalTypeSystem {
		if permToken := permTokenFromOperation(op); permToken != "" {
			if err := m.resolveDataScope(ctx, principal, permToken); err != nil {
				return err
			}
		}
	}

	return ctx.Next()
}

func (m *DataPermission) resolveDataScope(ctx fiber.Ctx, principal *security.Principal, permToken string) error {
	if m.resolver == nil {
		return fmt.Errorf(
			"%w: %w",
			fiber.ErrForbidden, ErrDataPermissionResolverNotProvided,
		)
	}

	ds, err := m.resolver.ResolveDataScope(ctx.Context(), principal, permToken)
	if err != nil {
		return fmt.Errorf(
			"%w: %w, principal=%q, permission=%q: %w",
			fiber.ErrForbidden, ErrDataScopeResolutionFailed, principal.ID, permToken, err,
		)
	}

	lgr := contextx.Logger(ctx)
	if ds != nil {
		lgr.Debugf("Resolved data scope: scope=%q, principal=%q", ds.Key(), principal.ID)
	} else {
		lgr.Debugf("No data scope resolved: principal=%q, permission=%q", principal.ID, permToken)
	}

	applier := security.NewRequestScopedDataPermApplier(principal, ds, lgr)

	contextx.SetDataPermApplier(ctx, applier)
	ctx.SetContext(contextx.SetDataPermApplier(ctx.Context(), applier))

	return nil
}
