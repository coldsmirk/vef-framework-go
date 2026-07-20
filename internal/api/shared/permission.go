package shared

import "regexp"

// permissionTokenPattern accepts dot-separated segments of letters, digits and
// underscores. Case is deliberately unconstrained — the convention this enforces
// is the separator, since a token is an opaque key that must match the one a
// RolePermissionsLoader returns, and mixing separators silently splits the same
// permission into two.
var permissionTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$`)

// IsValidPermissionToken reports whether a permission token follows the
// framework's dot-separated naming convention.
func IsValidPermissionToken(token string) bool {
	return permissionTokenPattern.MatchString(token)
}
