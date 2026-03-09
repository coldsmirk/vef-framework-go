package shared

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

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
