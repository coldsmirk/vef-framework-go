package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// PermissionChecker verifies if a Principal has a specific permission.
// Used by authorization middleware to enforce access control on API endpoints.
type PermissionChecker interface {
	// HasPermission returns true if the Principal has the specified permission token.
	HasPermission(ctx context.Context, principal *Principal, permToken string) (bool, error)
}

// RolePermissionsLoader retrieves permissions associated with a role.
// Used by RBAC implementations to build the permission set for authorization checks.
type RolePermissionsLoader interface {
	// LoadPermissions returns a map of permission tokens to their DataScope for a role.
	LoadPermissions(ctx context.Context, role string) (map[string]DataScope, error)
}

// DataScope defines row-level data access restrictions.
// Implementations filter query results based on the Principal's permissions,
// enabling multi-tenant data isolation or hierarchical data access control.
type DataScope interface {
	// Key returns a unique identifier for this data scope type.
	Key() string
	// Priority determines the order when multiple scopes apply (lower = higher priority).
	Priority() int
	// Supports returns true if this scope applies to the given Principal and table.
	Supports(principal *Principal, table *orm.Table) bool
	// Apply modifies the query to enforce the data scope restrictions.
	Apply(principal *Principal, query orm.SelectQuery) error
}

// DataPermissionResolver determines the applicable DataScope for a permission.
// Used to translate permission tokens into concrete data filtering rules.
type DataPermissionResolver interface {
	// ResolveDataScope returns the DataScope that should be applied for the permission.
	ResolveDataScope(ctx context.Context, principal *Principal, permToken string) (DataScope, error)
}

// DataPermissionApplier applies data permission filters to database queries.
// Wraps the resolution and application of DataScope into a single operation.
type DataPermissionApplier interface {
	// Apply adds data permission filters to the query based on the current context.
	Apply(query orm.SelectQuery) error
}
