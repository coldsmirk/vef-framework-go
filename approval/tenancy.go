package approval

import (
	"context"
	"errors"
	"slices"

	"github.com/coldsmirk/vef-framework-go/security"
)

// DefaultTenantID is the tenant identifier used when a caller does not carry
// an explicit tenant — the conventional single-tenant deployment value.
const DefaultTenantID = "default"

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
	// IsSystemInternal marks a caller without an HTTP / RPC principal whose
	// scope is established by other means, so Authorize passes
	// unconditionally. Resource paths must NEVER populate this. In the
	// current tree only test fixtures set it (the in-tree system paths —
	// timeout scanner, binding listener — act directly on already-loaded,
	// trusted rows and never construct a CallerContext); it remains the
	// intended marker for any host or future in-process system code that
	// legitimately needs to bypass tenant scoping.
	IsSystemInternal bool
}

// SystemCaller is the canonical CallerContext for callers that bypass tenant
// scoping by carrying IsSystemInternal. In the current tree its only
// consumers are test fixtures (production system paths operate on trusted,
// pre-scoped rows without a CallerContext); use it instead of the zero value
// so the bypass intent is explicit at the call site.
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

// TenantScopeFilter resolves the tenant filter for a LIST / metrics query,
// applying the same fail-closed posture as Authorize. A non-nil result scopes
// the query to that tenant; a nil result means unfiltered cross-tenant access
// and is only ever returned to super-admin / system-internal callers. An
// ordinary caller that resolves to an empty tenant fails closed with
// ErrCrossTenantAccess, so a forgotten or empty PrincipalTenantResolver result
// can never silently widen a list query to every tenant's rows — mirroring
// Authorize's rejection of a zero-value context. The override is honored only
// for privileged callers; ordinary callers are always pinned to their own
// tenant regardless of what the client sent. This is the single source of truth
// for list queries — the resource layer must never read params.TenantID
// directly.
func (c CallerContext) TenantScopeFilter(override string) (*string, error) {
	if c.IsSuperAdmin || c.IsSystemInternal {
		if override == "" {
			return nil, nil
		}

		scoped := override

		return &scoped, nil
	}

	if c.TenantID == "" {
		return nil, ErrCrossTenantAccess
	}

	scoped := c.TenantID

	return &scoped, nil
}

// ResolveWriteTenant returns the tenant a new or mutated entity must be stamped
// with. Super-admin / system-internal callers may target any tenant, so the
// client-supplied value is honored verbatim; every other caller is pinned to
// their own tenant and fails closed with ErrCrossTenantAccess when they have
// none — preventing a non-privileged caller from writing into another tenant.
func (c CallerContext) ResolveWriteTenant(clientTenant string) (string, error) {
	if c.IsSuperAdmin || c.IsSystemInternal {
		return clientTenant, nil
	}

	if c.TenantID == "" {
		return "", ErrCrossTenantAccess
	}

	return c.TenantID, nil
}

// PrincipalTenantResolver extracts the caller's tenant ID from a security
// principal. Implemented by host applications because Principal.Details is
// schema-less; the framework cannot know where the host stores tenant
// affiliation.
//
// Returning an empty string is only meaningful for super-admin or
// system-internal callers, which bypass tenant scoping. For an ordinary
// authenticated principal an empty tenant fails closed: CallerContext.Authorize
// returns ErrCrossTenantAccess and TenantScopeFilter / ResolveWriteTenant
// likewise reject it, rather than waving the request through.
type PrincipalTenantResolver interface {
	// Resolve returns the tenant identifier for the given principal.
	Resolve(ctx context.Context, principal *security.Principal) (string, error)
}
