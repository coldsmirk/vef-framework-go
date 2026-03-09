package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/security"
)

type RBACPermissionChecker struct {
	loader security.RolePermissionsLoader
}

func NewRBACPermissionChecker(loader security.RolePermissionsLoader) security.PermissionChecker {
	return &RBACPermissionChecker{
		loader: loader,
	}
}

// HasPermission uses sequential role loading rather than parallel to optimize for common case (1-3 roles).
func (c *RBACPermissionChecker) HasPermission(
	ctx context.Context,
	principal *security.Principal,
	permissionToken string,
) (bool, error) {
	if principal == nil || len(principal.Roles) == 0 {
		return false, nil
	}

	for _, role := range principal.Roles {
		permissions, err := c.loader.LoadPermissions(ctx, role)
		if err != nil {
			return false, err
		}

		if _, exists := permissions[permissionToken]; exists {
			return true, nil
		}
	}

	return false, nil
}
