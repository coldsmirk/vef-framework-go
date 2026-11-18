package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var actorLogger = logx.Named("approval:actors")

// ActorResolution resolves work for one instance and node. Both positive and
// negative host answers are memoized only for the duration of this operation.
type ActorResolution struct {
	scope    approval.HandoverContext
	resolver approval.HandoverResolver
	records  map[string]*approval.Handover
}

func NewActorResolution(instance *approval.Instance, node *approval.FlowNode, resolver approval.HandoverResolver) *ActorResolution {
	return &ActorResolution{
		scope:    approval.HandoverContext{TenantID: instance.TenantID, Instance: instance, Node: node},
		resolver: resolver, records: make(map[string]*approval.Handover),
	}
}

func (r *ActorResolution) load(ctx context.Context, ids []string) error {
	var missing []string
	for _, id := range shared.NormalizeUniqueIDs(ids) {
		if _, ok := r.records[id]; !ok {
			missing = append(missing, id)
		}
	}

	if len(missing) == 0 || r.resolver == nil {
		return nil
	}

	resolved, err := r.resolver.ResolveHandovers(orm.WithoutQuietSQLLog(ctx), &r.scope, missing)
	if err != nil {
		return fmt.Errorf("resolve approval handovers: %w", err)
	}

	for _, id := range missing {
		if h, ok := resolved[id]; ok {
			if h.Successor != nil && strings.TrimSpace(h.Successor.ID) == "" {
				return fmt.Errorf("%w for user %s: empty successor ID", ErrInvalidHandover, id)
			}

			if utf8.RuneCountInString(h.Reason) > 64 {
				return fmt.Errorf("%w for user %s: reason exceeds 64 characters", ErrInvalidHandover, id)
			}

			r.records[id] = &h
		} else {
			r.records[id] = nil
		}
	}

	return nil
}

// Handovers follows all requested chains in batches, preserving the first
// predecessor's reason. Each round asks the host only about unseen identities.
func (r *ActorResolution) Handovers(ctx context.Context, ids []string) (map[string]approval.Handover, error) {
	const maxHops = 10

	frontier := ids

	explored := make(map[string]bool)
	for depth := 0; depth <= maxHops && len(frontier) > 0; depth++ {
		if err := r.load(ctx, frontier); err != nil {
			return nil, err
		}

		var next []string
		for _, id := range frontier {
			explored[id] = true
			if h := r.records[id]; h != nil && h.Successor != nil {
				if !explored[h.Successor.ID] {
					next = append(next, h.Successor.ID)
				}
			}
		}

		frontier = shared.NormalizeUniqueIDs(next)
	}

	result := make(map[string]approval.Handover)
	for _, id := range ids {
		first := r.records[id]
		if first == nil {
			continue
		}

		h := *first
		chain := []string{id}

		people := []*approval.UserInfo{{ID: id}}
		for h.Successor != nil {
			next := h.Successor
			if len(chain) > maxHops {
				return nil, fmt.Errorf("%w for user %s: %v", ErrHandoverDepth, id, append(chain, next.ID))
			}

			if index := slices.Index(chain, next.ID); index >= 0 {
				h.Successor = people[max(index-1, 0)]
				actorLogger.Warnf("Approval handover cycle %v; keeping user %s", append(chain, next.ID), h.Successor.ID)

				break
			}

			chain = append(chain, next.ID)

			people = append(people, next)

			link := r.records[next.ID]
			if link == nil {
				break
			}

			h.Successor = link.Successor
		}

		result[id] = h
	}

	return result, nil
}

// Actors maps seats to their final actors: handover, then delegation. At each
// collision the first seat retains its attribution.
func (r *ActorResolution) Actors(ctx context.Context, db orm.DB, seats []approval.ResolvedAssignee, users approval.UserInfoResolver) ([]approval.ResolvedAssignee, error) {
	seats = deduplicateAssignees(seats)

	ids := make([]string, len(seats))
	for i, s := range seats {
		ids[i] = s.User.ID
	}

	changes, err := r.Handovers(ctx, ids)
	if err != nil {
		return nil, err
	}

	actors := make([]approval.ResolvedAssignee, 0, len(seats))
	for _, seat := range seats {
		if h, ok := changes[seat.User.ID]; ok {
			if h.Successor == nil {
				continue
			}

			if h.Successor.ID != seat.User.ID {
				seat.Predecessor = new(seat.User)
				seat.HandoverReason = h.Reason
				seat.User = *h.Successor
			}
		}

		actors = append(actors, seat)
	}

	actors = deduplicateAssignees(actors)
	if len(actors) == 0 {
		return actors, nil
	}

	actors, err = r.applyDelegation(ctx, db, r.scope.Instance.FlowID, actors, users)
	if err != nil {
		return nil, err
	}

	return deduplicateAssignees(actors), nil
}

// Seats pairs configured user IDs with their display snapshots.
func Seats(ids []string, infos map[string]approval.UserInfo) []approval.ResolvedAssignee {
	seats := make([]approval.ResolvedAssignee, len(ids))
	for i, user := range shared.UserInfos(ids, infos) {
		seats[i].User = user
	}

	return seats
}

// AssignTask replaces the actor, delegation and predecessor snapshots together.
func AssignTask(task *approval.Task, actor approval.ResolvedAssignee) {
	task.AssigneeID = actor.User.ID
	task.AssigneeName = actor.User.Name
	task.AssigneeDepartmentID = actor.User.DepartmentID
	task.AssigneeDepartmentName = actor.User.DepartmentName
	task.DelegatorID, task.DelegatorName, task.DelegatorDepartmentID, task.DelegatorDepartmentName = personFields(actor.Delegator)
	task.PredecessorID, task.PredecessorName, task.PredecessorDepartmentID, task.PredecessorDepartmentName = personFields(actor.Predecessor)

	task.HandoverReason = nil
	if actor.HandoverReason != "" {
		task.HandoverReason = new(actor.HandoverReason)
	}
}

func personFields(u *approval.UserInfo) (id, name, departmentID, departmentName *string) {
	if u == nil {
		return nil, nil, nil, nil
	}

	return new(u.ID), new(u.Name), u.DepartmentID, u.DepartmentName
}
