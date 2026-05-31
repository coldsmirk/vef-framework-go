package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
)

// TestHasPermission verifies all permission check paths.
func TestRBACPermissionCheckerHasPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("NilPrincipal", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		checker := NewRBACPermissionChecker(loader)

		has, err := checker.HasPermission(ctx, nil, "user:read")
		require.NoError(t, err, "Nil principal should not return an error")
		assert.False(t, has, "Nil principal should be denied permission")
	})

	t.Run("NoRoles", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		require.NoError(t, err, "User without roles should not return an error")
		assert.False(t, has, "User without roles should be denied permission")
	})

	t.Run("RoleHasPermission", func(t *testing.T) {
		scope := new(MockDataScope)
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": scope}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		require.NoError(t, err, "Role with requested permission should not return an error")
		assert.True(t, has, "Role with requested permission should be allowed")
		loader.AssertExpectations(t)
	})

	t.Run("RoleLacksPermission", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{"user:read": nil}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "viewer")

		has, err := checker.HasPermission(ctx, principal, "user:write")
		require.NoError(t, err, "Role missing requested permission should not return an error")
		assert.False(t, has, "Role missing requested permission should be denied")
		loader.AssertExpectations(t)
	})

	t.Run("SecondRoleHasPermission", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{}, nil,
		)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:write": nil}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "viewer", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:write")
		require.NoError(t, err, "Second role with requested permission should not return an error")
		assert.True(t, has, "Second role with requested permission should be allowed")
		loader.AssertExpectations(t)
	})

	t.Run("LoaderReturnsError", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("cache failure"))

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		_, err := checker.HasPermission(ctx, principal, "user:read")
		require.Error(t, err, "Loader failure should be returned")
		assert.Equal(t, "cache failure", err.Error(), "Loader error message should be preserved")
		loader.AssertExpectations(t)
	})

	t.Run("LoaderReturnsNilMap", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, nil)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		require.NoError(t, err, "Nil permission map should not return an error")
		assert.False(t, has, "Nil permission map should deny requested permission")
		loader.AssertExpectations(t)
	})

	t.Run("EmptyPermissionsMap", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		require.NoError(t, err, "Empty permission map should not return an error")
		assert.False(t, has, "Empty permission map should deny requested permission")
		loader.AssertExpectations(t)
	})
}
