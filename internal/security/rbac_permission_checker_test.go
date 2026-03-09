package security

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/security"
)

type RBACPermissionCheckerTestSuite struct {
	suite.Suite
}

// TestHasPermission verifies all permission check paths.
func (s *RBACPermissionCheckerTestSuite) TestHasPermission() {
	ctx := context.Background()

	s.Run("NilPrincipal", func() {
		loader := new(MockRolePermissionsLoader)
		checker := NewRBACPermissionChecker(loader)

		has, err := checker.HasPermission(ctx, nil, "user:read")
		s.Require().NoError(err, "Should not return error for nil principal")
		s.False(has, "Should deny permission for nil principal")
	})

	s.Run("NoRoles", func() {
		loader := new(MockRolePermissionsLoader)
		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error for empty roles")
		s.False(has, "Should deny permission when user has no roles")
	})

	s.Run("RoleHasPermission", func() {
		scope := new(MockDataScope)
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{"user:read": scope}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error")
		s.True(has, "Should grant permission when role has it")
		loader.AssertExpectations(s.T())
	})

	s.Run("RoleLacksPermission", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "viewer").Return(
			map[string]security.DataScope{"user:read": nil}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "viewer")

		has, err := checker.HasPermission(ctx, principal, "user:write")
		s.Require().NoError(err, "Should not return error")
		s.False(has, "Should deny permission when role lacks it")
		loader.AssertExpectations(s.T())
	})

	s.Run("SecondRoleHasPermission", func() {
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
		s.Require().NoError(err, "Should not return error")
		s.True(has, "Should grant permission from second role")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsError", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, errors.New("cache failure"))

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		_, err := checker.HasPermission(ctx, principal, "user:read")
		s.Require().Error(err, "Should propagate loader error")
		s.Equal("cache failure", err.Error(), "Should preserve error message")
		loader.AssertExpectations(s.T())
	})

	s.Run("LoaderReturnsNilMap", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(nil, nil)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error for nil map")
		s.False(has, "Should deny permission when map is nil")
		loader.AssertExpectations(s.T())
	})

	s.Run("EmptyPermissionsMap", func() {
		loader := new(MockRolePermissionsLoader)
		loader.On("LoadPermissions", mock.Anything, "admin").Return(
			map[string]security.DataScope{}, nil,
		)

		checker := NewRBACPermissionChecker(loader)
		principal := security.NewUser("user1", "Alice", "admin")

		has, err := checker.HasPermission(ctx, principal, "user:read")
		s.Require().NoError(err, "Should not return error")
		s.False(has, "Should deny when permissions map is empty")
		loader.AssertExpectations(s.T())
	})
}

func TestRBACPermissionChecker(t *testing.T) {
	suite.Run(t, new(RBACPermissionCheckerTestSuite))
}
