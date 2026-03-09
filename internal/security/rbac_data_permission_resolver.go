package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/security"
)

// RBACDataPermissionResolver implements role-based data permission resolution.
type RBACDataPermissionResolver struct {
	loader security.RolePermissionsLoader
}

// NewRBACDataPermissionResolver creates a new RBAC data permission resolver.
func NewRBACDataPermissionResolver(loader security.RolePermissionsLoader) security.DataPermissionResolver {
	return &RBACDataPermissionResolver{
		loader: loader,
	}
}

// ResolveDataScope resolves the applicable DataScope for the given principal and permission token.
// When a user has multiple roles with the same permission token but different data scopes,
// the scope with the highest priority wins. Returns nil if no matching permission is found.
func (r *RBACDataPermissionResolver) ResolveDataScope(
	ctx context.Context,
	principal *security.Principal,
	permToken string,
) (security.DataScope, error) {
	if r.loader == nil {
		return nil, nil
	}

	if principal == nil || len(principal.Roles) == 0 {
		return nil, nil
	}

	var (
		selectedScope security.DataScope
		maxPriority   = -1
	)

	// Sequential loading is sufficient since most users have only 1-3 roles.
	for _, role := range principal.Roles {
		permissions, err := r.loader.LoadPermissions(ctx, role)
		if err != nil {
			return nil, err
		}

		if dataScope, exists := permissions[permToken]; exists && dataScope != nil {
			if priority := dataScope.Priority(); priority > maxPriority {
				maxPriority = priority
				selectedScope = dataScope
			}
		}
	}

	return selectedScope, nil
}
