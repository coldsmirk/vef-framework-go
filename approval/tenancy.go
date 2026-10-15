package approval

import (
	"context"
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
	// TenantID is the caller's resolved tenant. A super-admin acts across
	// tenants yet may still belong to one, which is where its writes default
	// to; system-internal callers carry none. Every other caller must carry a
	// tenant — an ordinary caller with an empty one is denied everything.
	TenantID string
	// IsSuperAdmin grants cross-tenant access regardless of TenantID.
	IsSuperAdmin bool
	// IsSystemInternal marks a caller without an HTTP / RPC principal whose
	// scope is established by other means, so Authorize passes
	// unconditionally. Resource paths must NEVER populate this: it is the
	// marker for in-process system code, which since the introduction of
	// Service means host automation calling the approval engine directly
	// (see SystemCaller). The framework's own system paths — timeout scanner,
	// projection worker — act on already-loaded, trusted rows and never
	// construct a CallerContext at all.
	IsSystemInternal bool
}

// SystemCaller is the canonical CallerContext for callers that bypass tenant
// scoping by carrying IsSystemInternal — host automation driving Service
// without a principal, and test fixtures. Use it instead of the zero value so
// the bypass intent is explicit at the call site; a zero CallerContext is
// denied, not waved through. Anything acting for a person should carry that
// person's tenant instead: CallerContext{TenantID: …}.
var SystemCaller = CallerContext{IsSystemInternal: true}

// Authorize reports whether the caller is allowed to act on an entity owned
// by entityTenantID. Super-admin callers always pass; system-internal
// callers pass too (no HTTP principal exists, scope is enforced upstream).
// Every other caller must belong to a tenant that matches the entity tenant
// exactly — zero-value contexts are rejected.
func (c CallerContext) Authorize(entityTenantID string) error {
	if c.IsSuperAdmin || c.IsSystemInternal {
		return nil
	}

	tenantID, ok := c.memberTenant()
	if !ok || tenantID != entityTenantID {
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
// ordinary caller that resolves to no tenant fails closed with
// ErrCrossTenantAccess, so a forgotten or empty PrincipalTenantResolver result
// can never silently widen a list query to every tenant's rows — mirroring
// Authorize's rejection of a zero-value context. The override is honored only for privileged callers; ordinary
// callers are always pinned to their own tenant regardless of what the client
// sent. This is the single source of truth for list queries — the resource
// layer must never read params.TenantID directly.
func (c CallerContext) TenantScopeFilter(override string) (*string, error) {
	if c.IsSuperAdmin || c.IsSystemInternal {
		if override == "" {
			return nil, nil
		}

		scoped := override

		return &scoped, nil
	}

	tenantID, ok := c.memberTenant()
	if !ok {
		return nil, ErrCrossTenantAccess
	}

	return &tenantID, nil
}

// ResolveWriteTenant returns the tenant a new instance is stamped with. Every
// caller but a privileged one is pinned to its own tenant whatever it
// requested, and fails closed with ErrCrossTenantAccess when it has none, so it
// can never write into another tenant. A super-admin or system-internal caller
// writes into the requested tenant; naming none, into its own tenant when it
// has one, else DefaultTenantID. Only Service callers
// request a tenant: the HTTP resources start in the caller's own.
func (c CallerContext) ResolveWriteTenant(requested string) (string, error) {
	if c.IsSuperAdmin || c.IsSystemInternal {
		if requested != "" {
			return requested, nil
		}

		if tenantID, ok := c.memberTenant(); ok {
			return tenantID, nil
		}

		return DefaultTenantID, nil
	}

	tenantID, ok := c.memberTenant()
	if !ok {
		return "", ErrCrossTenantAccess
	}

	return tenantID, nil
}

// memberTenant returns the tenant an ordinary caller acts within. It reports
// false when the tenant is unresolved (empty).
func (c CallerContext) memberTenant() (string, bool) {
	return c.TenantID, c.TenantID != ""
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
