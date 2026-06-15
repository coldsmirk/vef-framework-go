package shared_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// checkerAssigneeService adds the RoleMembershipChecker capability on top of a
// FakeAssigneeService and flags any (wrong) fallback to GetRoleUsers, so tests
// can assert UserHasRole takes the direct capability fast path.
type checkerAssigneeService struct {
	FakeAssigneeService

	member       bool
	checkErr     error
	getRoleUsers bool
}

func (s *checkerAssigneeService) UserHasRole(context.Context, string, string) (bool, error) {
	if s.checkErr != nil {
		return false, s.checkErr
	}

	return s.member, nil
}

func (s *checkerAssigneeService) GetRoleUsers(ctx context.Context, roleID string) ([]approval.UserInfo, error) {
	s.getRoleUsers = true

	return s.FakeAssigneeService.GetRoleUsers(ctx, roleID)
}

func TestUserHasRole(t *testing.T) {
	ctx := context.Background()

	t.Run("NilServiceReportsNoMembership", func(t *testing.T) {
		member, err := shared.UserHasRole(ctx, nil, "u1", "role-1")
		require.NoError(t, err, "Nil service must not error")
		assert.False(t, member, "Nil service reports no membership")
	})

	t.Run("FallbackMatchViaGetRoleUsers", func(t *testing.T) {
		svc := &FakeAssigneeService{
			RoleUsers: map[string][]approval.UserInfo{"role-1": {{ID: "u1"}, {ID: "u2"}}},
		}

		member, err := shared.UserHasRole(ctx, svc, "u2", "role-1")
		require.NoError(t, err, "Fallback membership check should not error")
		assert.True(t, member, "User present in the role's user list is a member")
	})

	t.Run("FallbackNoMatch", func(t *testing.T) {
		svc := &FakeAssigneeService{
			RoleUsers: map[string][]approval.UserInfo{"role-1": {{ID: "u1"}}},
		}

		member, err := shared.UserHasRole(ctx, svc, "stranger", "role-1")
		require.NoError(t, err, "Fallback membership check should not error")
		assert.False(t, member, "User absent from the role's user list is not a member")
	})

	t.Run("FallbackPropagatesError", func(t *testing.T) {
		sentinel := errors.New("role listing failed")
		svc := &FakeAssigneeService{Err: sentinel}

		_, err := shared.UserHasRole(ctx, svc, "u1", "role-1")
		require.ErrorIs(t, err, sentinel, "GetRoleUsers error must propagate")
	})

	t.Run("FastPathMember", func(t *testing.T) {
		checker := &checkerAssigneeService{member: true}

		member, err := shared.UserHasRole(ctx, checker, "u1", "role-1")
		require.NoError(t, err, "Fast-path membership check should not error")
		assert.True(t, member, "Fast-path checker reporting membership returns true")
		assert.False(t, checker.getRoleUsers, "Fast path must NOT fall back to GetRoleUsers")
	})

	t.Run("FastPathNonMember", func(t *testing.T) {
		checker := &checkerAssigneeService{member: false}

		member, err := shared.UserHasRole(ctx, checker, "u1", "role-1")
		require.NoError(t, err, "Fast-path membership check should not error")
		assert.False(t, member, "Fast-path checker reporting non-membership returns false")
		assert.False(t, checker.getRoleUsers, "Fast path must NOT fall back to GetRoleUsers")
	})

	t.Run("FastPathPropagatesError", func(t *testing.T) {
		sentinel := errors.New("membership check failed")
		checker := &checkerAssigneeService{checkErr: sentinel}

		_, err := shared.UserHasRole(ctx, checker, "u1", "role-1")
		require.ErrorIs(t, err, sentinel, "UserHasRole error must propagate")
		assert.False(t, checker.getRoleUsers, "Fast path must NOT fall back to GetRoleUsers on error")
	})
}
