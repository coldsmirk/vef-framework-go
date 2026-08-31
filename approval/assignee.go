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

// AssigneeResolveContext is what an AssigneeResolver resolves against: the
// node-level runtime snapshot plus the one assignee rule being resolved. The
// rule's Kind is carried so a resolver registered for several kinds — or one
// logging its own decisions — can tell which rule it was handed.
type AssigneeResolveContext struct {
	NodeResolveContext

	// Kind is the assignee kind of the rule being resolved.
	Kind AssigneeKind
	// IDs are the designer-selected IDs of the rule. Their meaning belongs to
	// the kind: user IDs, role IDs, department IDs, or whatever a custom
	// kind's picker stores.
	IDs []string
	// FormField names the form field carrying the assignee IDs, set only for
	// kinds whose SelectionMode is SelectionFormField.
	FormField *string
}

// AssigneeResolver turns one assignee rule into the concrete people who
// receive tasks. The framework registers a resolver per built-in kind and
// hosts add their own with vef.ProvideApprovalAssigneeResolver; a host
// resolver whose kind matches a built-in replaces it.
//
// Registration is what makes a kind deployable: flow validation accepts
// exactly the registered kinds, and the flow designer offers exactly their
// descriptors. There is no separate enum to extend.
type AssigneeResolver interface {
	// Describe returns the kind this resolver handles together with the
	// designer metadata for it — the label to show and the input to collect.
	Describe() KindDescriptor[AssigneeKind]
	// Resolve returns the assignees for the rule in rc. Returning none is a
	// valid answer: the node's EmptyAssigneeAction then decides what happens,
	// which is how an optional approver step is expressed. Return an error
	// only when the rule could not be evaluated — a failure fails the
	// approval action rather than silently dropping approvers.
	Resolve(ctx context.Context, rc *AssigneeResolveContext) ([]ResolvedAssignee, error)
}
