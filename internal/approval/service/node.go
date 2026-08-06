package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NodeService provides node-level domain operations.
type NodeService struct {
	engine       *engine.FlowEngine
	bus          event.Bus
	taskSvc      *TaskService
	userResolver approval.UserInfoResolver
	ccResolver   *shared.CCRecipientResolver
}

// NewNodeService creates a new NodeService.
func NewNodeService(
	engine *engine.FlowEngine,
	bus event.Bus,
	taskSvc *TaskService,
	userResolver approval.UserInfoResolver,
	ccResolver *shared.CCRecipientResolver,
) *NodeService {
	return &NodeService{
		engine:       engine,
		bus:          bus,
		taskSvc:      taskSvc,
		userResolver: userResolver,
		ccResolver:   ccResolver,
	}
}

// HandleNodeCompletion evaluates the node's pass rule and settles the outcome.
// Both outcomes trigger completion-timing CC, cancel the remaining tasks, and
// conclude the open node visit; PassRulePassed then advances to the next node
// while PassRuleRejected finishes the instance as rejected. A pending result is
// a no-op.
//
// Every event this produces is emitted here, in occurrence order, before the
// step that follows it — the cancellations land before AdvanceToNextNode lets
// the engine announce a completed instance, and before the rejection's own
// completion event. The returned slice is what was emitted, handed back only so
// the caller can reconcile the activations it computed beforehand (see
// SuppressSupersededActivations); re-adding it to the collector would publish
// each event twice.
//
// This method persists status transitions and engine-driven node changes in the
// caller's transaction while keeping the supplied instance in sync.
func (s *NodeService) HandleNodeCompletion(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	node *approval.FlowNode,
) ([]approval.DomainEvent, error) {
	completionResult, err := s.engine.EvaluateNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return nil, fmt.Errorf("evaluate node completion: %w", err)
	}

	switch completionResult {
	case approval.PassRulePassed:
		if err := s.TriggerNodeCC(ctx, db, instance, node, approval.PassRulePassed); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		canceledEvents, err := s.taskSvc.CancelRemainingTasks(ctx, db, instance, node, "节点已通过，剩余任务无需处理")
		if err != nil {
			return nil, err
		}

		// Emitted before the advance: whatever the next node decides — up to
		// completing the instance — happens after these tasks were canceled.
		if err := behavior.EmitEvents(ctx, s.bus, db, canceledEvents...); err != nil {
			return nil, err
		}

		if err := engine.ConcludeActiveNodeVisit(ctx, db, instance.ID, node.ID, approval.NodeVisitPassed); err != nil {
			return nil, err
		}

		if err := s.engine.AdvanceToNextNode(ctx, db, instance, node, nil); err != nil {
			return nil, fmt.Errorf("advance to next node: %w", err)
		}

		return canceledEvents, nil

	case approval.PassRuleRejected:
		if err := s.TriggerNodeCC(ctx, db, instance, node, approval.PassRuleRejected); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		canceledEvents, err := s.taskSvc.CancelRemainingTasks(ctx, db, instance, node, "节点已拒绝，剩余任务无需处理")
		if err != nil {
			return nil, err
		}

		if err := behavior.EmitEvents(ctx, s.bus, db, canceledEvents...); err != nil {
			return nil, err
		}

		if err := engine.ConcludeActiveNodeVisit(ctx, db, instance.ID, node.ID, approval.NodeVisitRejected); err != nil {
			return nil, err
		}

		instance.FinishedAt = new(timex.Now())
		// Route through the transition helper so the business projection and
		// host-registered InstanceLifecycleHook implementations see the
		// rejection (same as NodeActionComplete in the engine).
		if err := engine.ApplyInstanceTransitionWithHooks(
			ctx, db, instance, approval.InstanceRejected, s.engine.LifecycleHooks(), "finished_at",
		); err != nil {
			return nil, fmt.Errorf("apply rejection transition: %w", err)
		}

		completed := approval.NewInstanceCompletedEvent(instance, approval.InstanceRejected)
		if err := behavior.EmitEvents(ctx, s.bus, db, completed); err != nil {
			return nil, err
		}

		return append(canceledEvents, completed), nil

	default:
		return nil, nil
	}
}

// TriggerNodeCC creates CC records when a node completes, based on CCTiming configuration.
func (s *NodeService) TriggerNodeCC(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, completionResult approval.PassRuleResult) error {
	var ccConfigs []approval.FlowNodeCC

	if err := db.NewSelect().
		Model(&ccConfigs).
		Select("timing", "kind", "ids", "form_field").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", node.ID)
		}).
		Scan(ctx); err != nil {
		return fmt.Errorf("load cc configs for node %s: %w", node.ID, err)
	}

	if len(ccConfigs) == 0 {
		return nil
	}

	formData := approval.NewFormData(instance.FormData)

	// CC resolution is best-effort (unresolvable configs are logged and skipped);
	// it never fails the approval whose completion triggered the CC.
	resolved := shared.CollectUniqueCCUserIDs(
		ctx,
		ccConfigs,
		formData,
		s.ccResolver.Resolve,
		func(cfg approval.FlowNodeCC) bool {
			switch cfg.Timing {
			case approval.CCTimingAlways:
				return true
			case approval.CCTimingOnApprove:
				return completionResult == approval.PassRulePassed
			case approval.CCTimingOnReject:
				return completionResult == approval.PassRuleRejected
			default:
				return false
			}
		},
	)

	if len(resolved) == 0 {
		return nil
	}

	ccUserInfos := shared.ResolveUserInfoMapSilent(ctx, s.userResolver, resolved)

	// Completion-timing CC belongs to the traversal being concluded; the
	// visit is still open here (HandleNodeCompletion concludes it after).
	visit, err := engine.FindActiveNodeVisit(ctx, db, instance.ID, node.ID)
	if err != nil {
		return err
	}

	insertedUserIDs, err := shared.InsertAutoCCRecords(ctx, db, instance.ID, node.ID, visit.ID, resolved, ccUserInfos)
	if err != nil {
		return fmt.Errorf("insert cc records: %w", err)
	}

	if len(insertedUserIDs) == 0 {
		return nil
	}

	evt := approval.NewCCNotifiedEvent(instance, node, shared.UserInfos(insertedUserIDs, ccUserInfos), false)

	return behavior.EmitEvents(ctx, s.bus, db, evt)
}

// AdvanceCCNodeIfAllRead checks if all CC records for CC nodes are read and advances the flow.
func (s *NodeService) AdvanceCCNodeIfAllRead(ctx context.Context, db orm.DB, instanceID string, records []approval.CCRecord) error {
	nodeIDs := shared.NewOrderedUnique[string](len(records))
	for _, record := range records {
		if record.NodeID == nil {
			continue
		}

		nodeIDs.Add(*record.NodeID)
	}

	if nodeIDs.Len() == 0 {
		return nil
	}

	var instance approval.Instance

	instance.ID = instanceID
	if err := db.NewSelect().
		Model(&instance).
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		return fmt.Errorf("find instance for cc advance: %w", err)
	}

	// Guard against duplicate advancement caused by concurrent read-confirm actions.
	if instance.Status != approval.InstanceRunning {
		return nil
	}

	if instance.CurrentNodeID == nil {
		return nil
	}

	currentNodeID := *instance.CurrentNodeID
	if !nodeIDs.Contains(currentNodeID) {
		return nil
	}

	var node approval.FlowNode

	node.ID = currentNodeID
	if err := db.NewSelect().
		Model(&node).
		WherePK().
		Scan(ctx); err != nil {
		return fmt.Errorf("load cc node %s: %w", currentNodeID, err)
	}

	if node.Kind != approval.NodeCC || !node.IsReadConfirmRequired {
		return nil
	}

	visit, err := engine.FindActiveNodeVisit(ctx, db, instanceID, currentNodeID)
	if err != nil {
		return err
	}

	hasUnread, err := shared.HasUnreadCCRecords(ctx, db, instanceID, currentNodeID, visit.ID)
	if err != nil {
		return err
	}

	if hasUnread {
		return nil
	}

	// The read-confirm gate has cleared: the CC node's visit concludes here,
	// not in the engine — AdvanceToNextNode enters the next node directly.
	if err := engine.ConcludeActiveNodeVisit(ctx, db, instanceID, currentNodeID, approval.NodeVisitPassed); err != nil {
		return err
	}

	if err := s.engine.AdvanceToNextNode(ctx, db, &instance, &node, nil); err != nil {
		return fmt.Errorf("advance cc node: %w", err)
	}

	return nil
}
