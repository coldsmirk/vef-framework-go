package shared_test

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
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
