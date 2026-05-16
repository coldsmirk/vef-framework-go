package approval

import (
	"context"
	"errors"
	"slices"

	"github.com/coldsmirk/vef-framework-go/security"
)

// SuperAdminRole is the role string that grants cross-tenant access to
// admin queries and operations. Hosts assign this role to platform-level
// operators that legitimately need to act across tenants (audit teams,
// billing, etc.). Without it, admin endpoints reject requests that lack a
// tenant filter — guarding against accidental cross-tenant data exposure.
const SuperAdminRole = "approval:super_admin"

// IsSuperAdmin reports whether the principal carries the cross-tenant
// override role. Nil principal returns false.
func IsSuperAdmin(p *security.Principal) bool {
	if p == nil {
		return false
	}

	return slices.Contains(p.Roles, SuperAdminRole)
}

// ErrCrossTenantAccess is returned when a non-super-admin caller attempts
// to act on an entity owned by a different tenant. Resource and command
// handlers use CallerContext.Authorize to surface it consistently.
var ErrCrossTenantAccess = errors.New("approval: cross-tenant access denied")

// CallerContext bundles the tenant authority of a single API call. Resource
// handlers resolve it from the security principal (via
// PrincipalTenantResolver + IsSuperAdmin) and pass it into commands /
// queries through their struct fields so handlers can enforce data
// ownership without re-parsing principal details.
//
// Production code MUST populate exactly one of TenantID / IsSuperAdmin /
// IsSystemInternal. A zero-value CallerContext is treated as
// **unauthorized** so a forgotten resolver call surfaces as a deny rather
// than silently waving the request through — this is the deliberate fail-
// closed posture introduced after the security audit caught fail-open
// regressions in the default principal resolver.
type CallerContext struct {
	// TenantID is the caller's resolved tenant. Empty for super-admin or
	// for system-internal callers; must be non-empty for every authenticated
	// API request.
	TenantID string
	// IsSuperAdmin grants cross-tenant access regardless of TenantID.
	IsSuperAdmin bool
	// IsSystemInternal marks a caller without an HTTP / RPC principal
	// (timeout scanner, binding listener, engine-internal saga, test
	// fixtures). Authorize passes unconditionally for such callers; resource
	// paths must NEVER populate this — it's only legitimate for in-process
	// system code that already established scope by other means.
	IsSystemInternal bool
}

// SystemCaller is the canonical CallerContext for in-process system code
// (timeout scanner, binding listener, engine-internal sagas, test fixtures
// that intentionally bypass tenant scoping). Use this instead of the zero
// value so the intent is explicit at the call site.
var SystemCaller = CallerContext{IsSystemInternal: true}

// Authorize reports whether the caller is allowed to act on an entity owned
// by entityTenantID. Super-admin callers always pass; system-internal
// callers pass too (no HTTP principal exists, scope is enforced upstream).
// Every other caller must have a non-empty TenantID that matches the
// entity tenant exactly — zero-value contexts are rejected.
func (c CallerContext) Authorize(entityTenantID string) error {
	if c.IsSuperAdmin || c.IsSystemInternal {
		return nil
	}

	if c.TenantID == "" || c.TenantID != entityTenantID {
		return ErrCrossTenantAccess
	}

	return nil
}

// Allows is the bool variant of Authorize. Query handlers use it when a
// failed authorization must mimic "not found" rather than surface an error
// — the typical multi-tenant pattern that avoids leaking entity existence
// across tenants.
func (c CallerContext) Allows(entityTenantID string) bool {
	return c.Authorize(entityTenantID) == nil
}

// EffectiveTenantID returns the tenant filter the caller is actually
// allowed to query. Non-super-admin callers always operate within their
// own tenant; their override (if any) is ignored. Super-admin and
// system-internal callers may pass a specific tenant through override, or
// empty for cross-tenant visibility. This is the single source of truth
// for list queries — the resource layer should never read params.TenantID
// directly.
func (c CallerContext) EffectiveTenantID(override string) string {
	if c.IsSuperAdmin || c.IsSystemInternal {
		return override
	}

	return c.TenantID
}

// PrincipalTenantResolver extracts the caller's tenant ID from a security
// principal. Implemented by host applications because Principal.Details is
// schema-less; the framework cannot know where the host stores tenant
// affiliation.
//
// Returning an empty string is valid (e.g. system principals, platform
// super-admins, or anonymous callers) — CallerContext.Authorize then falls
// back to its zero-value pass-through.
type PrincipalTenantResolver interface {
	// Resolve returns the tenant identifier for the given principal.
	Resolve(ctx context.Context, principal *security.Principal) (string, error)
}
