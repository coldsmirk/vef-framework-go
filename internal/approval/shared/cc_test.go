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

	t.Run("Propagates AssigneeService error instead of swallowing it", func(t *testing.T) {
		failing := shared.NewCCRecipientResolver(&FakeAssigneeService{Err: errors.New("boom")})
		_, err := failing.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCRole, IDs: []string{"role-a"}}, nil)
		require.Error(t, err, "role CC should surface AssigneeService errors, not drop recipients silently")
	})
}

// TestCCRecipientResolverNilService pins the F5 fix: role/department CC must
// fail loudly when no AssigneeService is registered, rather than the old
// behavior of silently resolving to nobody.
func TestCCRecipientResolverNilService(t *testing.T) {
	resolver := shared.NewCCRecipientResolver(nil)
	ctx := context.Background()

	t.Run("Role without service errors loudly", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCRole, IDs: []string{"role-a"}}, nil)
		require.Error(t, err, "role CC without an AssigneeService must error, not silently notify nobody")
	})

	t.Run("Department without service errors loudly", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCDepartment, IDs: []string{"dept-1"}}, nil)
		require.Error(t, err, "department CC without an AssigneeService must error, not silently notify nobody")
	})

	t.Run("User kind still works without a service", func(t *testing.T) {
		got, err := resolver.Resolve(ctx, approval.FlowNodeCC{Kind: approval.CCUser, IDs: []string{"a"}}, nil)
		require.NoError(t, err, "user CC needs no AssigneeService")
		assert.Equal(t, []string{"a"}, got, "user CC should still resolve without a service")
	})
}
