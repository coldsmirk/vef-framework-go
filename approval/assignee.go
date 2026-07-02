package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/security"
)

// UserInfo is the approval module's uniform person reference — identity plus
// optional department — used everywhere a user appears: resolver lookups,
// command operators, persisted snapshots (task assignees and delegators, CC
// recipients, urge parties, action-log person lists), and detail projections,
// so a client renders every person with one component and no second lookup.
// The role a value plays is carried by the field holding it (Operator,
// Applicant, User, Delegator, TransferTo, …), never by a separate type.
// Department fields hold whatever the writing side captured at the time and
// are omitted when unknown; hosts opt in by filling them in
// UserInfoResolver.ResolveUsers.
type UserInfo struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	DepartmentID   *string `json:"departmentId,omitempty"`
	DepartmentName *string `json:"departmentName,omitempty"`
}

// UserInfoResolver resolves user display info by IDs (implemented by host app).
type UserInfoResolver interface {
	// ResolveUsers returns user info for the given IDs.
	// Missing IDs should be returned with empty Name (not omitted).
	// Department fields are optional; fill them to enrich the person
	// snapshots the approval module records.
	ResolveUsers(ctx context.Context, userIDs []string) (map[string]UserInfo, error)
}

// AssigneeService resolves approval assignees from organizational data (implemented by host app).
type AssigneeService interface {
	// GetSuperior returns the direct superior's user info for the given user.
	GetSuperior(ctx context.Context, userID string) (*UserInfo, error)
	// GetDepartmentLeaders returns the leader user info for the given department.
	GetDepartmentLeaders(ctx context.Context, departmentID string) ([]UserInfo, error)
	// GetRoleUsers returns all user info for users that have the given role.
	GetRoleUsers(ctx context.Context, roleID string) ([]UserInfo, error)
}

// RoleMembershipChecker is an optional capability of AssigneeService. Hosts
// that can answer "does this user hold this role" directly should implement
// it: membership checks (e.g. role-based initiation permission) then skip
// the GetRoleUsers full-listing fallback, which scales poorly for large
// roles. Detected via type assertion, so existing implementations keep
// working unchanged.
type RoleMembershipChecker interface {
	// UserHasRole reports whether the user currently holds the role.
	UserHasRole(ctx context.Context, userID, roleID string) (bool, error)
}

// ResolvedAssignee represents a resolved assignee with optional delegation
// info: User is who receives the task; Delegator is set when the task arrived
// via delegation and names the original assignee.
type ResolvedAssignee struct {
	User      UserInfo
	Delegator *UserInfo
}

// PrincipalDepartmentResolver resolves department info from a security principal.
// Implemented by host apps since Principal.Details is business-specific.
type PrincipalDepartmentResolver interface {
	// Resolve extracts the department ID and name from the given security principal.
	Resolve(ctx context.Context, principal *security.Principal) (departmentID, departmentName *string, err error)
}

// AssigneeDefinition represents an assignee configuration in the flow definition.
type AssigneeDefinition struct {
	Kind      AssigneeKind `json:"kind"`
	IDs       []string     `json:"ids,omitempty"`
	FormField *string      `json:"formField,omitempty"`
	SortOrder int          `json:"sortOrder"`
}

// CCDefinition represents a CC recipient in node data.
type CCDefinition struct {
	Kind      CCKind   `json:"kind"`
	IDs       []string `json:"ids,omitempty"`
	FormField *string  `json:"formField,omitempty"`
	Timing    CCTiming `json:"timing,omitempty"`
}
