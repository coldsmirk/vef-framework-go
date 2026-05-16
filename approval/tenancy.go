package approval

import (
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
