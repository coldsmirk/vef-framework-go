package strategy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// --- Test Helpers ---

// MockAssigneeService is a configurable mock for approval.AssigneeService.
type MockAssigneeService struct {
	superiors   map[string]approval.UserInfo
	deptLeaders map[string][]approval.UserInfo
	roleUsers   map[string][]approval.UserInfo
}

func (m *MockAssigneeService) GetSuperior(_ context.Context, userID string) (*approval.UserInfo, error) {
	if s, ok := m.superiors[userID]; ok {
		return &s, nil
	}

	return nil, nil
}

func (m *MockAssigneeService) GetDeptLeaders(_ context.Context, deptID string) ([]approval.UserInfo, error) {
	if leaders, ok := m.deptLeaders[deptID]; ok {
		return leaders, nil
	}

	return nil, nil
}

func (m *MockAssigneeService) GetRoleUsers(_ context.Context, roleID string) ([]approval.UserInfo, error) {
	if users, ok := m.roleUsers[roleID]; ok {
		return users, nil
	}

	return nil, nil
}

// ErrAssigneeService always returns errors for all methods.
type ErrAssigneeService struct{}

var errAssigneeSvc = errors.New("assignee service error")

func (*ErrAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, errAssigneeSvc
}

func (*ErrAssigneeService) GetDeptLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errAssigneeSvc
}

func (*ErrAssigneeService) GetRoleUsers(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errAssigneeSvc
}

// assertUserIDs asserts that the resolved assignees match the expected user IDs in order.
func assertUserIDs(t *testing.T, result []approval.ResolvedAssignee, expected ...string) {
	t.Helper()
	require.Len(t, result, len(expected), "Should resolve expected number of assignees")

	for i, uid := range expected {
		assert.Equal(t, uid, result[i].UserID, "Assignee[%d] should be %s", i, uid)
	}
}

// assertResolvedAssignees asserts that the resolved assignees match the expected user IDs and names in order.
func assertResolvedAssignees(t *testing.T, result []approval.ResolvedAssignee, expected []approval.UserInfo) {
	t.Helper()
	require.Len(t, result, len(expected), "Should resolve expected number of assignees")

	for i, exp := range expected {
		assert.Equal(t, exp.ID, result[i].UserID, "Assignee[%d].UserID should be %s", i, exp.ID)
		assert.Equal(t, exp.Name, result[i].UserName, "Assignee[%d].UserName should be %s", i, exp.Name)
	}
}

// --- UserAssigneeResolver ---

func TestUserAssigneeResolver(t *testing.T) {
	r := NewUserAssigneeResolver()
	assert.Equal(t, approval.AssigneeUser, r.Kind(), "Kind should be AssigneeUser")

	tests := []struct {
		name     string
		ids      []string
		expected []string
	}{
		{"SingleID", []string{"u1"}, []string{"u1"}},
		{"MultipleIDs", []string{"u1", "u2", "u3"}, []string{"u1", "u2", "u3"}},
		{"IgnoreEmptyIDs", []string{"", "u1", "", "u2"}, []string{"u1", "u2"}},
		{"TrimWhitespaceIDs", []string{"  u1  ", "\t", "u2 "}, []string{"u1", "u2"}},
		{"EmptyIDs", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.Resolve(context.Background(), &ResolveContext{IDs: tt.ids})
			require.NoError(t, err, "Should resolve without error")
			assertUserIDs(t, result, tt.expected...)
		})
	}
}

// --- SelfAssigneeResolver ---

func TestSelfAssigneeResolver(t *testing.T) {
	r := NewSelfAssigneeResolver()
	assert.Equal(t, approval.AssigneeSelf, r.Kind(), "Kind should be AssigneeSelf")

	t.Run("WithApplicant", func(t *testing.T) {
		result, err := r.Resolve(context.Background(), &ResolveContext{ApplicantID: "applicant1"})
		require.NoError(t, err, "Should resolve without error")
		assertUserIDs(t, result, "applicant1")
	})

	t.Run("EmptyApplicant", func(t *testing.T) {
		_, err := r.Resolve(context.Background(), &ResolveContext{})
		require.ErrorIs(t, err, ErrApplicantIDEmpty, "Should return ErrApplicantIDEmpty")
	})
}

// --- RoleAssigneeResolver ---

func TestRoleAssigneeResolver(t *testing.T) {
	svc := &MockAssigneeService{
		roleUsers: map[string][]approval.UserInfo{
			"admin":   {{ID: "u1", Name: "User U1"}, {ID: "u2", Name: "User U2"}},
			"manager": {{ID: "u3", Name: "User U3"}},
		},
	}
	r := NewRoleAssigneeResolver(svc)
	assert.Equal(t, approval.AssigneeRole, r.Kind(), "Kind should be AssigneeRole")

	tests := []struct {
		name     string
		ids      []string
		expected []approval.UserInfo
	}{
		{"SingleRole", []string{"admin"}, []approval.UserInfo{{ID: "u1", Name: "User U1"}, {ID: "u2", Name: "User U2"}}},
		{"MultipleRoles", []string{"admin", "manager"}, []approval.UserInfo{{ID: "u1", Name: "User U1"}, {ID: "u2", Name: "User U2"}, {ID: "u3", Name: "User U3"}}},
		{"UnknownRole", []string{"unknown"}, nil},
		{"EmptyIDs", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.Resolve(context.Background(), &ResolveContext{IDs: tt.ids})
			require.NoError(t, err, "Should resolve without error")
			assertResolvedAssignees(t, result, tt.expected)
		})
	}

	t.Run("NilService", func(t *testing.T) {
		_, err := NewRoleAssigneeResolver(nil).Resolve(context.Background(), &ResolveContext{IDs: []string{"r1"}})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Should return ErrAssigneeServiceNil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		_, err := NewRoleAssigneeResolver(&ErrAssigneeService{}).Resolve(context.Background(), &ResolveContext{IDs: []string{"r1"}})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap underlying service error")
	})
}

// --- SuperiorAssigneeResolver ---

func TestSuperiorAssigneeResolver(t *testing.T) {
	svc := &MockAssigneeService{
		superiors: map[string]approval.UserInfo{"emp1": {ID: "mgr1", Name: "Manager 1"}},
	}
	r := NewSuperiorAssigneeResolver(svc)
	assert.Equal(t, approval.AssigneeSuperior, r.Kind(), "Kind should be AssigneeSuperior")

	t.Run("WithSuperior", func(t *testing.T) {
		result, err := r.Resolve(context.Background(), &ResolveContext{ApplicantID: "emp1"})
		require.NoError(t, err, "Should resolve without error")
		assertResolvedAssignees(t, result, []approval.UserInfo{{ID: "mgr1", Name: "Manager 1"}})
	})

	t.Run("NoSuperior", func(t *testing.T) {
		r := NewSuperiorAssigneeResolver(&MockAssigneeService{})
		result, err := r.Resolve(context.Background(), &ResolveContext{ApplicantID: "emp1"})
		require.NoError(t, err, "Should resolve without error when no superior found")
		assert.Empty(t, result, "Should return empty result")
	})

	t.Run("NilService", func(t *testing.T) {
		_, err := NewSuperiorAssigneeResolver(nil).Resolve(context.Background(), &ResolveContext{ApplicantID: "emp1"})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Should return ErrAssigneeServiceNil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		_, err := NewSuperiorAssigneeResolver(&ErrAssigneeService{}).Resolve(context.Background(), &ResolveContext{ApplicantID: "emp1"})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap underlying service error")
	})
}

// --- DeptLeaderAssigneeResolver ---

func TestDeptLeaderAssigneeResolver(t *testing.T) {
	svc := &MockAssigneeService{
		deptLeaders: map[string][]approval.UserInfo{
			"dept1": {{ID: "leader1", Name: "Leader 1"}, {ID: "leader2", Name: "Leader 2"}},
		},
	}
	r := NewDeptLeaderAssigneeResolver(svc)
	assert.Equal(t, approval.AssigneeDeptLeader, r.Kind(), "Kind should be AssigneeDeptLeader")

	t.Run("WithLeaders", func(t *testing.T) {
		result, err := r.Resolve(context.Background(), &ResolveContext{ApplicantDeptID: new("dept1")})
		require.NoError(t, err, "Should resolve without error")
		assertResolvedAssignees(t, result, []approval.UserInfo{{ID: "leader1", Name: "Leader 1"}, {ID: "leader2", Name: "Leader 2"}})
	})

	t.Run("UnknownDept", func(t *testing.T) {
		result, err := r.Resolve(context.Background(), &ResolveContext{ApplicantDeptID: new("unknown")})
		require.NoError(t, err, "Should resolve without error")
		assertUserIDs(t, result)
	})

	t.Run("NilService", func(t *testing.T) {
		_, err := NewDeptLeaderAssigneeResolver(nil).Resolve(context.Background(), &ResolveContext{ApplicantDeptID: new("dept1")})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Should return ErrAssigneeServiceNil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		_, err := NewDeptLeaderAssigneeResolver(&ErrAssigneeService{}).Resolve(context.Background(), &ResolveContext{ApplicantDeptID: new("dept1")})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap underlying service error")
	})
}

// --- DeptAssigneeResolver ---

func TestDeptAssigneeResolver(t *testing.T) {
	svc := &MockAssigneeService{
		deptLeaders: map[string][]approval.UserInfo{
			"dept1": {{ID: "leader1", Name: "Leader 1"}},
			"dept2": {{ID: "leader2", Name: "Leader 2"}, {ID: "leader3", Name: "Leader 3"}},
		},
	}
	r := NewDeptAssigneeResolver(svc)
	assert.Equal(t, approval.AssigneeDept, r.Kind(), "Kind should be AssigneeDept")

	tests := []struct {
		name     string
		ids      []string
		expected []approval.UserInfo
	}{
		{"SingleDept", []string{"dept1"}, []approval.UserInfo{{ID: "leader1", Name: "Leader 1"}}},
		{"MultipleDepts", []string{"dept1", "dept2"}, []approval.UserInfo{{ID: "leader1", Name: "Leader 1"}, {ID: "leader2", Name: "Leader 2"}, {ID: "leader3", Name: "Leader 3"}}},
		{"UnknownDept", []string{"unknown"}, nil},
		{"EmptyIDs", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.Resolve(context.Background(), &ResolveContext{IDs: tt.ids})
			require.NoError(t, err, "Should resolve without error")
			assertResolvedAssignees(t, result, tt.expected)
		})
	}

	t.Run("NilService", func(t *testing.T) {
		_, err := NewDeptAssigneeResolver(nil).Resolve(context.Background(), &ResolveContext{IDs: []string{"dept1"}})
		require.ErrorIs(t, err, ErrAssigneeServiceNil, "Should return ErrAssigneeServiceNil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		_, err := NewDeptAssigneeResolver(&ErrAssigneeService{}).Resolve(context.Background(), &ResolveContext{IDs: []string{"dept1"}})
		require.ErrorIs(t, err, errAssigneeSvc, "Should wrap underlying service error")
	})
}

// --- FormFieldAssigneeResolver ---

func TestFormFieldAssigneeResolver(t *testing.T) {
	r := NewFormFieldAssigneeResolver()
	assert.Equal(t, approval.AssigneeFormField, r.Kind(), "Kind should be AssigneeFormField")

	// Success cases: supported value types that resolve correctly.
	successTests := []struct {
		name     string
		field    string
		formData approval.FormData
		expected []string
	}{
		{"StringValue", "approver", approval.FormData{"approver": "user1"}, []string{"user1"}},
		{"StringValueWithWhitespace", "approver", approval.FormData{"approver": " user1 "}, []string{"user1"}},
		{"StringSlice", "approvers", approval.FormData{"approvers": []string{"u1", "u2"}}, []string{"u1", "u2"}},
		{"StringSliceWithEmptyElement", "approvers", approval.FormData{"approvers": []string{"u1", "", "u2"}}, []string{"u1", "u2"}},
		{"StringSliceWithWhitespaceElement", "approvers", approval.FormData{"approvers": []string{" u1 ", " ", "u2"}}, []string{"u1", "u2"}},
		{"AnySlice", "approvers", approval.FormData{"approvers": []any{"u1", "u2"}}, []string{"u1", "u2"}},
		{"AnySliceWithEmptyElement", "approvers", approval.FormData{"approvers": []any{"u1", "", "u2"}}, []string{"u1", "u2"}},
		{"AnySliceWithWhitespaceElement", "approvers", approval.FormData{"approvers": []any{" u1 ", " ", "u2"}}, []string{"u1", "u2"}},
		{"NilValue", "missing", approval.FormData{}, nil},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ResolveContext{FormField: new(tt.field), FormData: tt.formData}
			result, err := r.Resolve(context.Background(), rc)
			require.NoError(t, err, "Should resolve without error")
			assertUserIDs(t, result, tt.expected...)
		})
	}

	// Error cases: invalid field name, empty values, unsupported types.
	errorTests := []struct {
		name      string
		rc        *ResolveContext
		wantError error
	}{
		{
			"NilFormFieldName",
			&ResolveContext{FormData: approval.FormData{"approver": "user1"}},
			ErrFormFieldNameEmpty,
		},
		{
			"EmptyFormFieldName",
			&ResolveContext{FormField: new(""), FormData: approval.FormData{"approver": "user1"}},
			ErrFormFieldNameEmpty,
		},
		{
			"WhitespaceFormFieldName",
			&ResolveContext{FormField: new("   "), FormData: approval.FormData{"approver": "user1"}},
			ErrFormFieldNameEmpty,
		},
		{
			"EmptyStringValue",
			&ResolveContext{FormField: new("approver"), FormData: approval.FormData{"approver": ""}},
			ErrFormFieldValueEmpty,
		},
		{
			"WhitespaceStringValue",
			&ResolveContext{FormField: new("approver"), FormData: approval.FormData{"approver": "   "}},
			ErrFormFieldValueEmpty,
		},
		{
			"UnsupportedValueType",
			&ResolveContext{FormField: new("count"), FormData: approval.FormData{"count": 42}},
			ErrUnsupportedFieldValueType,
		},
		{
			"UnsupportedMapType",
			&ResolveContext{FormField: new("meta"), FormData: approval.FormData{"meta": map[string]string{"k": "v"}}},
			ErrUnsupportedFieldValueType,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Resolve(context.Background(), tt.rc)
			require.ErrorIs(t, err, tt.wantError, "Should return %v", tt.wantError)
		})
	}
}

// --- CompositeAssigneeResolver ---

func TestCompositeAssigneeResolver(t *testing.T) {
	svc := &MockAssigneeService{
		roleUsers:   map[string][]approval.UserInfo{"admin": {{ID: "u1", Name: "User U1"}}},
		superiors:   map[string]approval.UserInfo{"emp1": {ID: "mgr1", Name: "Manager 1"}},
		deptLeaders: map[string][]approval.UserInfo{"dept1": {{ID: "leader1", Name: "Leader 1"}}},
	}

	t.Run("MultipleKinds", func(t *testing.T) {
		composite := NewCompositeAssigneeResolver(NewUserAssigneeResolver(), NewSelfAssigneeResolver())
		configs := []approval.FlowNodeAssignee{
			{Kind: approval.AssigneeUser, IDs: []string{"u1", "u2"}},
			{Kind: approval.AssigneeSelf},
		}

		result, err := composite.ResolveAll(context.Background(), configs, &ResolveContext{ApplicantID: "applicant1"})
		require.NoError(t, err, "Should resolve all without error")
		assertUserIDs(t, result, "u1", "u2", "applicant1")
	})

	t.Run("AllResolverKinds", func(t *testing.T) {
		composite := NewCompositeAssigneeResolver(
			NewUserAssigneeResolver(),
			NewSelfAssigneeResolver(),
			NewRoleAssigneeResolver(svc),
		)
		configs := []approval.FlowNodeAssignee{
			{Kind: approval.AssigneeUser, IDs: []string{"u1"}},
			{Kind: approval.AssigneeSelf},
			{Kind: approval.AssigneeRole, IDs: []string{"admin"}},
		}

		result, err := composite.ResolveAll(context.Background(), configs, &ResolveContext{ApplicantID: "applicant1"})
		require.NoError(t, err, "Should resolve all kinds")
		assertUserIDs(t, result, "u1", "applicant1", "u1")
	})

	t.Run("EmptyConfigs", func(t *testing.T) {
		composite := NewCompositeAssigneeResolver(NewUserAssigneeResolver())

		result, err := composite.ResolveAll(context.Background(), nil, &ResolveContext{})
		require.NoError(t, err, "Should resolve empty configs without error")
		assert.Empty(t, result, "Should return empty result")
	})

	t.Run("UnknownKind", func(t *testing.T) {
		composite := NewCompositeAssigneeResolver(NewUserAssigneeResolver())
		configs := []approval.FlowNodeAssignee{
			{Kind: approval.AssigneeRole, IDs: []string{"r1"}},
		}

		_, err := composite.ResolveAll(context.Background(), configs, &ResolveContext{})
		require.ErrorIs(t, err, ErrAssigneeResolverNotFound, "Should return ErrAssigneeResolverNotFound")
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		composite := NewCompositeAssigneeResolver(NewSuperiorAssigneeResolver(&ErrAssigneeService{}))
		configs := []approval.FlowNodeAssignee{
			{Kind: approval.AssigneeSuperior},
		}

		_, err := composite.ResolveAll(context.Background(), configs, &ResolveContext{ApplicantID: "emp1"})
		require.ErrorIs(t, err, errAssigneeSvc, "Should propagate wrapped service error")
	})
}
