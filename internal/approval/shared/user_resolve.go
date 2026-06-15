package shared

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// UserHasRole reports whether the user currently holds the role. It is the
// single source of truth for role-membership checks: it prefers the host's
// direct RoleMembershipChecker capability and falls back to listing the role's
// members (correct for any host, but linear in role size). Routing every caller
// through it keeps the read and validation paths from answering the same
// question two different ways. A nil service reports no membership.
func UserHasRole(ctx context.Context, svc approval.AssigneeService, userID, roleID string) (bool, error) {
	if svc == nil {
		return false, nil
	}

	if checker, ok := svc.(approval.RoleMembershipChecker); ok {
		member, err := checker.UserHasRole(ctx, userID, roleID)
		if err != nil {
			return false, fmt.Errorf("check role membership %s: %w", roleID, err)
		}

		return member, nil
	}

	users, err := svc.GetRoleUsers(ctx, roleID)
	if err != nil {
		return false, fmt.Errorf("get users by role %s: %w", roleID, err)
	}

	return slices.ContainsFunc(users, func(u approval.UserInfo) bool { return u.ID == userID }), nil
}

// ResolveUserNameMap batch-resolves user IDs to a map of ID→Name.
// Returns an error if the resolver fails.
func ResolveUserNameMap(ctx context.Context, resolver approval.UserInfoResolver, ids []string) (map[string]string, error) {
	names := make(map[string]string, len(ids))
	if resolver == nil || len(ids) == 0 {
		return names, nil
	}

	infos, err := resolver.ResolveUsers(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		if info, ok := infos[id]; ok {
			names[id] = info.Name
		}
	}

	return names, nil
}

// ResolveUserNameMapSilent batch-resolves user IDs to a map of ID→Name.
// Silently returns an empty map on resolver failure (best-effort for display-only fields).
func ResolveUserNameMapSilent(ctx context.Context, resolver approval.UserInfoResolver, ids []string) map[string]string {
	names, _ := ResolveUserNameMap(ctx, resolver, ids)

	return names
}

// ResolveUserName resolves a single user ID to a display name.
// Returns empty string on failure (best-effort for display-only fields).
func ResolveUserName(ctx context.Context, resolver approval.UserInfoResolver, userID string) string {
	if resolver == nil || userID == "" {
		return ""
	}

	names := ResolveUserNameMapSilent(ctx, resolver, []string{userID})

	return names[userID]
}
