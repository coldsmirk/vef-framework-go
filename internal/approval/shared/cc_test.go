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

// FakeAssigneeService is a test double for approval.AssigneeService.
type FakeAssigneeService struct {
	RoleUsers   map[string][]approval.UserInfo
	DeptLeaders map[string][]approval.UserInfo
	Err         error
}

func (*FakeAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, nil
}

func (f *FakeAssigneeService) GetDepartmentLeaders(_ context.Context, departmentID string) ([]approval.UserInfo, error) {
	if f.Err != nil {
		return nil, f.Err
	}

	return f.DeptLeaders[departmentID], nil
}

func (f *FakeAssigneeService) GetRoleUsers(_ context.Context, roleID string) ([]approval.UserInfo, error) {
	if f.Err != nil {
		return nil, f.Err
	}

	return f.RoleUsers[roleID], nil
}

func TestCCRecipientResolver(t *testing.T) {
	ctx := context.Background()
	svc := &FakeAssigneeService{
		RoleUsers: map[string][]approval.UserInfo{
			"role-a": {{ID: "u1", Name: "U1"}, {ID: "u2", Name: "U2"}},
			"role-b": {{ID: "u2", Name: "U2"}, {ID: "u3", Name: "U3"}},
		},
		DeptLeaders: map[string][]approval.UserInfo{
			"dept-1": {{ID: "leader-1", Name: "Leader"}},
		},
	}
	resolver := shared.NewCCRecipientResolver(svc)

	t.Run("Role resolves via AssigneeService and dedups across roles", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCRole, IDs: []string{"role-a", "role-b"}}, nil)
		require.NoError(t, err, "role CC resolution should not error")
		assert.Equal(t, []string{"u1", "u2", "u3"}, got, "role CC should resolve and dedup users in first-seen order")
	})

	t.Run("Department resolves to leaders", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCDepartment, IDs: []string{"dept-1"}}, nil)
		require.NoError(t, err, "department CC resolution should not error")
		assert.Equal(t, []string{"leader-1"}, got, "department CC should resolve to department leaders")
	})

	t.Run("User kind still resolves statically", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCUser, IDs: []string{"a", "b", "a"}}, nil)
		require.NoError(t, err, "user CC resolution should not error")
		assert.Equal(t, []string{"a", "b"}, got, "user CC should resolve static unique IDs")
	})

	t.Run("Org lookup error is skipped best-effort, not fatal to the approval", func(t *testing.T) {
		failing := shared.NewCCRecipientResolver(&FakeAssigneeService{Err: errors.New("boom")})
		got, err := failing.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCRole, IDs: []string{"role-a"}}, nil)
		require.NoError(t, err, "a CC org-lookup error must not fail the approval — CC is a best-effort notification")
		assert.Empty(t, got, "role CC yields no recipients when the org lookup fails")
	})
}

// TestCCRecipientResolverNilService pins the best-effort contract for role /
// department CC: a missing AssigneeService is logged and resolves to no
// recipients rather than failing the approval that triggered the CC. The
// earlier release made it fatal, which wedged every approval transition on any
// flow using role/department CC without an org service registered.
func TestCCRecipientResolverNilService(t *testing.T) {
	resolver := shared.NewCCRecipientResolver(nil)
	ctx := context.Background()

	t.Run("Role without service skips best-effort", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCRole, IDs: []string{"role-a"}}, nil)
		require.NoError(t, err, "role CC without an AssigneeService must not fail the approval")
		assert.Empty(t, got, "role CC resolves to no recipients when no AssigneeService is registered")
	})

	t.Run("Department without service skips best-effort", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCDepartment, IDs: []string{"dept-1"}}, nil)
		require.NoError(t, err, "department CC without an AssigneeService must not fail the approval")
		assert.Empty(t, got, "department CC resolves to no recipients when no AssigneeService is registered")
	})

	t.Run("User kind still works without a service", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCUser, IDs: []string{"a"}}, nil)
		require.NoError(t, err, "user CC needs no AssigneeService")
		assert.Equal(t, []string{"a"}, got, "user CC should still resolve without a service")
	})
}
