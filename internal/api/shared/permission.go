package shared

import "regexp"

// permissionTokenPattern enforces the dot-separator convention; case is left
// unconstrained. A token is an opaque key that must match the one a
// RolePermissionsLoader returns, and mixing separators silently splits the same
// permission into two.
var permissionTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$`)

// IsValidPermissionToken reports whether a permission token follows the
// framework's dot-separated naming convention.
func IsValidPermissionToken(token string) bool {
	return permissionTokenPattern.MatchString(token)
}
