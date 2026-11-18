package engine

import (
	"cmp"
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// CheckPicks rejects an explicitly selected user who has handed over the work.
func (r *ActorResolution) CheckPicks(ctx context.Context, ids []string, users approval.UserInfoResolver) error {
	changes, err := r.Handovers(ctx, ids)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if h, ok := changes[id]; ok {
			user := shared.ResolveUserInfo(ctx, users, id)

			params := map[string]any{"user": cmp.Or(user.Name, id)}
			if h.Successor != nil && h.Successor.ID != id {
				params["successor"] = cmp.Or(h.Successor.Name, h.Successor.ID)
			}

			return result.Err(i18n.T(shared.ErrMessageUserHandedOver, params), result.WithCode(approval.ErrCodeUserHandedOver))
		}
	}

	return nil
}

// CCRecipients maps recipients through handovers without consulting delegations.
func (r *ActorResolution) CCRecipients(ctx context.Context, ids []string, users approval.UserInfoResolver) ([]string, map[string]approval.UserInfo, error) {
	changes, err := r.Handovers(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	recipients := shared.NewOrderedUnique[string](len(ids))

	infos := shared.ResolveUserInfoMapSilent(ctx, users, ids)
	if infos == nil {
		infos = make(map[string]approval.UserInfo)
	}

	for _, id := range ids {
		if h, ok := changes[id]; ok {
			if h.Successor == nil {
				actorLogger.Warnf("Skipping CC for user %s without a successor on instance %s", id, r.scope.Instance.ID)

				continue
			}

			if h.Successor.ID != id {
				id = h.Successor.ID
				infos[id] = *h.Successor
			}
		}

		recipients.Add(id)
	}

	return recipients.ToSlice(), infos, nil
}
