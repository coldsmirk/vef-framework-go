package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/security"
)

type RBACDataPermissionResolverTestSuite struct {
	suite.Suite
}

// TestResolveDataScope verifies all data scope resolution paths.
func (s *RBACDataPermissionResolverTestSuite) TestResolveDataScope() {
	ctx := context.Background()

	s.Run("NilLoader", func() {
		resolver := NewRBACDataPermissionResolver(nil)

		scope, err := resolver.ResolveDataScope(ctx, security.NewUser("user1", "Alice", "admin"), "user:read")
		s.Require().NoError(err, "Should not return error for nil loader")
		s.Nil(scope, "Should return nil scope when loader is nil")
	})

	s.Run("NilPrincipal", func() {
		loader := new(MockRolePermissionsLoader)
		resolver := NewRBACDataPermissionResolver(loader)

		scope, err := resolver.ResolveDataScope(ctx, nil, "user:read")
		s.Require().NoError(err, "Should not return error for nil principal")
		s.Nil(scope, "Should return nil scope for nil principal")
	})

	s.Run("NoRoles", func() {
		loader := new(MockRolePermissionsLoader)
		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error for empty roles")
		s.Nil(scope, "Should return nil scope when user has no roles")
	})

	s.Run("PermissionNotFound", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{"dashboard:read": nil}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error")
		s.Nil(scope, "Should return nil when permission not found")
		loader.AssertExpectations(s.T())
	})

	s.Run("PermissionFound", func() {
		deptScope := new(MockDataScope)
		deptScope.On("Priority").Return(10)

		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": deptScope}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error")
		s.NotNil(scope, "Should return data scope")
		s.Equal(10, scope.Priority(), "Should return correct scope")
		loader.AssertExpectations(s.T())
	})

	s.Run("HighestPriorityWins", func() {
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
		s.Require().NoError(err, "Should not return error")
		s.Require().NotNil(scope, "Should return data scope")
		s.Equal(20, scope.Priority(), "Should select scope with highest priority")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsError", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("cache failure"))

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		_, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().Error(err, "Should propagate loader error")
		s.Equal("cache failure", err.Error(), "Should preserve error message")
		loader.AssertExpectations(s.T())
	})

	s.Run("ErrorOnSecondRoleStopsEarly", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{}, nil,
		)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("timeout"))

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "viewer", "admin")

		_, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().Error(err, "Should propagate error from second role")
		loader.AssertExpectations(s.T())
	})

	s.Run("OnlyOneRoleHasPermission", func() {
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
		s.Require().NoError(err, "Should not return error")
		s.Require().NotNil(scope, "Should find scope from second role")
		s.Equal(10, scope.Priority(), "Should return correct scope")
		loader.AssertExpectations(s.T())
	})

	s.Run("NilDataScopeValue", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": nil}, nil,
		)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not panic on nil DataScope value")
		s.Nil(scope, "Should return nil when DataScope value is nil")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsNilMap", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, nil)

		resolver := NewRBACDataPermissionResolver(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		scope, err := resolver.ResolveDataScope(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error for nil map")
		s.Nil(scope, "Should return nil scope for nil map")
		loader.AssertExpectations(s.T())
	})
}

func TestRBACDataPermissionResolver(t *testing.T) {
	suite.Run(t, new(RBACDataPermissionResolverTestSuite))
}
