package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// NodeService provides node-level domain operations.
type NodeService struct {
	engine       *engine.FlowEngine
	publisher    *dispatcher.EventPublisher
	taskSvc      *TaskService
	userResolver approval.UserInfoResolver
}

// NewNodeService creates a new NodeService.
func NewNodeService(eng *engine.FlowEngine, pub *dispatcher.EventPublisher, taskSvc *TaskService, userResolver approval.UserInfoResolver) *NodeService {
	return &NodeService{engine: eng, publisher: pub, taskSvc: taskSvc, userResolver: userResolver}
}

// HandleNodeCompletion evaluates node completion and handles the result.
// On PassRulePassed: advances to the next node and cancels remaining tasks.
// On PassRuleRejected: marks instance as rejected, cancels remaining tasks, and resumes parent flow.
//
// This method mutates instance fields (Status, FinishedAt, CurrentNodeID) in memory.
// The caller is responsible for persisting instance changes to the database.
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

		if err := s.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
			return nil, err
		}

		if err := s.engine.AdvanceToNextNode(ctx, db, instance, node, nil); err != nil {
			return nil, fmt.Errorf("advance to next node: %w", err)
		}

		return nil, nil

	case approval.PassRuleRejected:
		if err := engine.InstanceStateMachine.Transition(instance.Status, approval.InstanceRejected); err != nil {
			return nil, err
		}

		if err := s.TriggerNodeCC(ctx, db, instance, node, approval.PassRuleRejected); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		instance.Status = approval.InstanceRejected
		instance.FinishedAt = new(timex.Now())

		if err := s.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
			return nil, err
		}

		return []approval.DomainEvent{
			approval.NewInstanceCompletedEvent(instance.ID, approval.InstanceRejected),
		}, nil

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

	resolved, err := shared.CollectUniqueCCUserIDs(
		ccConfigs,
		formData,
		engine.ResolveCCUserIDs,
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
	if err != nil {
		return fmt.Errorf("resolve node cc users: %w", err)
	}

	if len(resolved) == 0 {
		return nil
	}

	ccUserNames := shared.ResolveUserNameMapSilent(ctx, s.userResolver, resolved)

	insertedUserIDs, err := shared.InsertAutoCCRecords(ctx, db, instance.ID, node.ID, resolved, ccUserNames)
	if err != nil {
		return fmt.Errorf("insert cc records: %w", err)
	}

	if len(insertedUserIDs) == 0 {
		return nil
	}

	return s.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewCCNotifiedEvent(instance.ID, node.ID, insertedUserIDs, ccUserNames, false),
	})
}

// CheckCCNodeCompletion checks if all CC records for CC nodes are read and advances the flow.
func (s *NodeService) CheckCCNodeCompletion(ctx context.Context, db orm.DB, instanceID string, records []approval.CCRecord) error {
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

	unreadCount, err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", currentNodeID).
				IsNull("read_at")
		}).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("count unread cc records: %w", err)
	}

	if unreadCount > 0 {
		return nil
	}

	if err := s.engine.AdvanceToNextNode(ctx, db, &instance, &node, nil); err != nil {
		return fmt.Errorf("advance cc node: %w", err)
	}

	return nil
}
