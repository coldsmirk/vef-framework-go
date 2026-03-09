package approval

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/security"
)

// UserInfo represents a user identity with ID and display name.
type UserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserInfoResolver resolves user display names by IDs (implemented by host app).
type UserInfoResolver interface {
	// ResolveUsers returns user info for the given IDs.
	// Missing IDs should be returned with empty Name (not omitted).
	ResolveUsers(ctx context.Context, userIDs []string) (map[string]UserInfo, error)
}

// AssigneeService resolves approval assignees from organizational data (implemented by host app).
type AssigneeService interface {
	// GetSuperior returns the direct superior's user info for the given user.
	GetSuperior(ctx context.Context, userID string) (*UserInfo, error)
	// GetDeptLeaders returns the leader user info for the given department.
	GetDeptLeaders(ctx context.Context, deptID string) ([]UserInfo, error)
	// GetRoleUsers returns all user info for users that have the given role.
	GetRoleUsers(ctx context.Context, roleID string) ([]UserInfo, error)
}

// ResolvedAssignee represents a resolved assignee with optional delegation info.
type ResolvedAssignee struct {
	UserID        string
	UserName      string
	DelegatorID   *string
	DelegatorName *string
}

// PrincipalDeptResolver resolves department info from a security principal.
// Implemented by host apps since Principal.Details is business-specific.
type PrincipalDeptResolver interface {
	// Resolve extracts the department ID and name from the given security principal.
	Resolve(ctx context.Context, principal *security.Principal) (deptID, deptName *string, err error)
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
