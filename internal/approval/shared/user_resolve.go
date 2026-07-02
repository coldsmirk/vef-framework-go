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

// ResolveUserInfoMap batch-resolves user IDs to a map of ID→UserInfo (name
// plus optional department, per the host resolver). Missing IDs are simply
// absent — indexing the map yields a zero UserInfo whose fields are empty.
// Returns an error if the resolver fails.
func ResolveUserInfoMap(ctx context.Context, resolver approval.UserInfoResolver, ids []string) (map[string]approval.UserInfo, error) {
	infos := make(map[string]approval.UserInfo, len(ids))
	if resolver == nil || len(ids) == 0 {
		return infos, nil
	}

	resolved, err := resolver.ResolveUsers(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		if info, ok := resolved[id]; ok {
			infos[id] = info
		}
	}

	return infos, nil
}

// ResolveUserInfoMapSilent batch-resolves user IDs to a map of ID→UserInfo.
// Silently returns an empty map on resolver failure (best-effort for
// display-only fields).
func ResolveUserInfoMapSilent(ctx context.Context, resolver approval.UserInfoResolver, ids []string) map[string]approval.UserInfo {
	infos, _ := ResolveUserInfoMap(ctx, resolver, ids)

	return infos
}

// ResolveUserInfo resolves a single user ID to its display info. Returns a
// zero UserInfo on failure (best-effort for display-only fields); the ID field
// is always populated so callers can snapshot it verbatim.
func ResolveUserInfo(ctx context.Context, resolver approval.UserInfoResolver, userID string) approval.UserInfo {
	if userID == "" {
		return approval.UserInfo{}
	}

	info := ResolveUserInfoMapSilent(ctx, resolver, []string{userID})[userID]
	info.ID = userID

	return info
}

// UserInfoNames projects an ID→UserInfo map onto the ID→Name map carried by
// event payloads.
func UserInfoNames(infos map[string]approval.UserInfo) map[string]string {
	names := make(map[string]string, len(infos))
	for id, info := range infos {
		names[id] = info.Name
	}

	return names
}

// UserInfos builds the ordered person list for the given IDs from a resolved
// info map. An ID missing from the map still yields an entry carrying the ID,
// so unresolvable users stay visible in the record.
func UserInfos(ids []string, infos map[string]approval.UserInfo) []approval.UserInfo {
	users := make([]approval.UserInfo, len(ids))

	for i, id := range ids {
		info := infos[id]
		info.ID = id
		users[i] = info
	}

	return users
}
