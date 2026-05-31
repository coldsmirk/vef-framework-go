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

// TestResolveDataScope verifies all data scope resolution paths.
func TestRBACDataPermissionResolverResolveDataScope(t *testing.T) {
	ctx := context.Background()

	t.Run("NilLoader", func(t *testing.T) {
		resolver := NewRBACDataPermissionResolver(nil)

		scope, err := resolver.ResolveDataScope(ctx, security.NewUser("user1", "Alice", "admin"), "user:read")
		require.NoError(t, err, "Nil loader should not return an error")
		assert.Nil(t, scope, "Nil loader should return nil scope")
	})

	t.Run("NilPrincipal", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		resolver := NewRBACDataPermissionResolver(loader)

		scope, err := resolver.ResolveDataScope(ctx, nil, "user:read")
		require.NoError(t, err, "Nil principal should not return an error")
		assert.Nil(t, scope, "Nil principal should return nil scope")
	})

	t.Run("NoRoles", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "User without roles should not return an error")
		assert.Nil(t, scope, "User without roles should return nil scope")
	})

	t.Run("PermissionNotFound", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{"dashboard:read": nil}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Missing permission should not return an error")
		assert.Nil(t, scope, "Missing permission should return nil scope")
		loader.AssertExpectations(t)
	})

	t.Run("PermissionFound", func(t *testing.T) {
		deptScope := new(MockDataScope)
		deptScope.On("Priority").Return(10)

		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": deptScope}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Matching permission should not return an error")
		require.NotNil(t, scope, "Matching permission should return data scope")
		assert.Equal(t, 10, scope.Priority(), "Matching permission should return scope priority")
		loader.AssertExpectations(t)
	})

	t.Run("HighestPriorityWins", func(t *testing.T) {
		lowScope := new(MockDataScope)
		lowScope.On("Priority").Return(5)

		highScope := new(MockDataScope)
		highScope.On("Priority").Return(20)

		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{"user:read": lowScope}, nil,
		)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": highScope}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Multiple matching scopes should not return an error")
		require.NotNil(t, scope, "Multiple matching scopes should return a data scope")
		assert.Equal(t, 20, scope.Priority(), "Highest-priority matching scope should be selected")
		loader.AssertExpectations(t)
	})

	t.Run("LoaderReturnsError", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("cache failure"))

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		_, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.Error(t, err, "Loader failure should be returned")
		assert.Equal(t, "cache failure", err.Error(), "Loader error message should be preserved")
		loader.AssertExpectations(t)
	})

	t.Run("ErrorOnSecondRoleStopsEarly", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{}, nil,
		)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("timeout"))

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer", "admin")

		_, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.Error(t, err, "Second role loader failure should be returned")
		loader.AssertExpectations(t)
	})

	t.Run("OnlyOneRoleHasPermission", func(t *testing.T) {
		deptScope := new(MockDataScope)
		deptScope.On("Priority").Return(10)

		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{}, nil,
		)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": deptScope}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Second role matching permission should not return an error")
		require.NotNil(t, scope, "Second role matching permission should return data scope")
		assert.Equal(t, 10, scope.Priority(), "Second role matching permission should return scope priority")
		loader.AssertExpectations(t)
	})

	t.Run("NilDataScopeValue", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": nil}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Nil DataScope value should not return an error")
		assert.Nil(t, scope, "Nil DataScope value should return nil scope")
		loader.AssertExpectations(t)
	})

	t.Run("LoaderReturnsNilMap", func(t *testing.T) {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, nil)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		require.NoError(t, err, "Nil permission map should not return an error")
		assert.Nil(t, scope, "Nil permission map should return nil scope")
		loader.AssertExpectations(t)
	})
}
