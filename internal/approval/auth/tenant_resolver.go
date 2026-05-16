// Package auth provides the default identity-resolution wiring for the
// approval module. The defaults are intentionally tolerant: hosts that
// store tenant affiliation outside Principal.Details should replace the
// resolver via fx.Replace.
package auth

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/security"
)

// DefaultPrincipalTenantResolver looks up the tenant id from
// Principal.Details when it deserializes to a generic map. Returning an
// empty string falls back to the CallerContext zero-value pass-through, so
// hosts without tenant context still work for single-tenant deployments.
type DefaultPrincipalTenantResolver struct{}

// NewDefaultPrincipalTenantResolver constructs the default resolver.
func NewDefaultPrincipalTenantResolver() approval.PrincipalTenantResolver {
	return new(DefaultPrincipalTenantResolver)
}

// Resolve checks the principal's Details map for tenant_id / tenantId
// keys. Other Details shapes (typed structs) require a host-provided
// resolver via fx.Replace.
func (*DefaultPrincipalTenantResolver) Resolve(_ context.Context, p *security.Principal) (string, error) {
	if p == nil {
		return "", nil
	}

	m, ok := p.Details.(map[string]any)
	if !ok {
		return "", nil
	}

	for _, key := range []string{"tenant_id", "tenantId"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v, nil
		}
	}

	return "", nil
}
