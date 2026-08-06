package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const maxNodeDepth = 100

type nodeDepthKey struct{}

// FlowEngine is the core engine for processing approval workflows.
type FlowEngine struct {
	registry     *strategy.StrategyRegistry
	processors   map[approval.NodeKind]NodeProcessor
	bus          event.Bus
	userResolver approval.UserInfoResolver
	hooks        *LifecycleHookRunner
	flowCache    *FlowCache
}

// NewFlowEngine creates a new flow engine. Duplicate NodeKind registrations
// panic on construction — silent overwrite is a deployment bug we want to
// surface at boot rather than mask at runtime.
//
// flowCache is required: node/edge traversal resolves entirely through the
// compiled-flow cache (a series of map lookups over the immutable published
// version), so callers must supply one. Tests that exercise traversal build
// it with engine.NewFlowCache(db, cache.NewMemory[*CompiledFlow]()).
func NewFlowEngine(
	registry *strategy.StrategyRegistry,
	processors []NodeProcessor,
	bus event.Bus,
	userResolver approval.UserInfoResolver,
	hooks *LifecycleHookRunner,
	flowCache *FlowCache,
) *FlowEngine {
	engine := &FlowEngine{
		registry:     registry,
		processors:   make(map[approval.NodeKind]NodeProcessor, len(processors)),
		bus:          bus,
		userResolver: userResolver,
		hooks:        hooks,
		flowCache:    flowCache,
	}

	for _, p := range processors {
		kind := p.NodeKind()
		if existing, dup := engine.processors[kind]; dup {
			panic(fmt.Sprintf("approval: duplicate node processor for kind %q: %T and %T", kind, existing, p))
		}

		engine.processors[kind] = p
	}

	return engine
}

// LifecycleHooks exposes the aggregated host-registered hooks so callers
// outside the engine (e.g. start_instance) can fire OnInstanceCreated at
// the right transactional moment.
func (e *FlowEngine) LifecycleHooks() *LifecycleHookRunner { return e.hooks }

// publishEvents emits domain events as the traversal produces them, through
// the shared collector-or-transactional-publish entry point. Visibility hinges
// on the caller committing. Returns nil when there are no events or no bus
// configured (test fixtures).
func (e *FlowEngine) publishEvents(ctx context.Context, db orm.DB, events ...approval.DomainEvent) error {
	return behavior.EmitEvents(ctx, e.bus, db, events...)
}

// StartProcess starts a flow process by finding the start node and processing it.
func (e *FlowEngine) StartProcess(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	compiled, err := e.flowCache.Get(ctx, instance.FlowVersionID)
	if err != nil {
		return fmt.Errorf("compile flow: %w", err)
	}

	if compiled.StartNode == nil {
		return fmt.Errorf("%w: %s", ErrFlowMissingStartNode, instance.FlowVersionID)
	}

	return e.ProcessNode(ctx, db, instance, compiled.StartNode)
}

// ProcessNode dispatches a node to the appropriate processor.
func (e *FlowEngine) ProcessNode(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	depth, _ := ctx.Value(nodeDepthKey{}).(int)
	if depth >= maxNodeDepth {
		return fmt.Errorf("%w: depth=%d, node=%s", ErrMaxNodeDepth, depth, node.ID)
	}

	ctx = context.WithValue(ctx, nodeDepthKey{}, depth+1)

	processor, ok := e.processors[node.Kind]
	if !ok {
		return fmt.Errorf("%w: %s", ErrProcessorNotFound, node.Kind)
	}

	// Begin the visit before the processor runs so tasks created during
	// processing can reference it; the visit is concluded by whichever path
	// decides the node's outcome.
	visit, err := beginNodeVisit(ctx, db, instance, node)
	if err != nil {
		return err
	}

	pc := &ProcessContext{
		DB:            db,
		Instance:      instance,
		Node:          node,
		Visit:         visit,
		FormData:      approval.NewFormData(instance.FormData),
		ApplicantID:   instance.ApplicantID,
		ApplicantName: instance.ApplicantName,
		UserResolver:  e.userResolver,
		Registry:      e.registry,
	}

	result, err := processor.Process(ctx, pc)
	if err != nil {
		return err
	}

	return e.handleProcessResult(ctx, db, instance, node, visit, result)
}

func (e *FlowEngine) handleProcessResult(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, visit *approval.NodeVisit, result *ProcessResult) error {
	if err := e.publishEvents(ctx, db, result.Events...); err != nil {
		return fmt.Errorf("publish processor events: %w", err)
	}

	switch result.Action {
	case NodeActionWait:
		instance.CurrentNodeID = new(node.ID)

		_, err := db.NewUpdate().
			Model(instance).
			Select("current_node_id").
			WherePK().
			Exec(ctx)

		return err

	case NodeActionContinue:
		if err := concludeNodeVisit(ctx, db, visit, approval.NodeVisitPassed); err != nil {
			return err
		}

		return e.AdvanceToNextNode(ctx, db, instance, node, result.BranchID)

	case NodeActionComplete:
		// An end node completes the instance as approved (visit passed); an
		// auto-reject execution completes it as rejected (visit rejected).
		visitStatus := approval.NodeVisitPassed
		if *result.FinalStatus == approval.InstanceRejected {
			visitStatus = approval.NodeVisitRejected
		}

		if err := concludeNodeVisit(ctx, db, visit, visitStatus); err != nil {
			return err
		}

		instance.CurrentNodeID = new(node.ID)
		instance.FinishedAt = new(timex.Now())

		// ApplyInstanceTransitionWithHooks centralizes both the state-
		// machine UPDATE and lifecycle hook fan-out so every final-status
		// path (here, pass-rule rejection, admin terminate, …) fires the
		// same host extensions inside the same tx.
		if err := ApplyInstanceTransitionWithHooks(
			ctx, db, instance, *result.FinalStatus, e.hooks,
			"current_node_id", "finished_at",
		); err != nil {
			return fmt.Errorf("apply completion transition: %w", err)
		}

		if err := e.publishEvents(
			ctx, db,
			approval.NewInstanceCompletedEvent(instance, *result.FinalStatus),
		); err != nil {
			return fmt.Errorf("publish instance completed event: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("%w: %d", errUnknownNodeAction, result.Action)
	}
}

// AdvanceToNextNode finds the matching edge from the current node and advances to the next one.
// BranchID is used by condition nodes to select the edge matching the branch.
func (e *FlowEngine) AdvanceToNextNode(ctx context.Context, db orm.DB, instance *approval.Instance, fromNode *approval.FlowNode, branchID *string) error {
	compiled, err := e.flowCache.Get(ctx, instance.FlowVersionID)
	if err != nil {
		return fmt.Errorf("compile flow: %w", err)
	}

	edge, err := compiled.FindOutgoing(fromNode.ID, branchID)
	if err != nil {
		return err
	}

	nextNode, ok := compiled.Nodes[edge.TargetNodeID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrFlowMissingTargetNode, edge.TargetNodeID)
	}

	return e.ProcessNode(ctx, db, instance, nextNode)
}

// EvaluateNodeCompletion evaluates whether a node is complete based on its
// tasks and pass rule. Counting is scoped to the node's open visit, so tasks
// left behind by an earlier traversal — e.g. an approval that survived a
// peer-initiated rollback — cannot contaminate the redo round's decision.
func (e *FlowEngine) EvaluateNodeCompletion(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode) (approval.PassRuleResult, error) {
	visit, err := FindActiveNodeVisit(ctx, db, instance.ID, node.ID)
	if err != nil {
		return approval.PassRulePending, err
	}

	var tasks []approval.Task

	err = db.NewSelect().
		Model(&tasks).
		Select("status").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("visit_id", visit.ID)
		}).
		Scan(ctx)
	if err != nil {
		return approval.PassRulePending, fmt.Errorf("query tasks: %w", err)
	}

	return e.evaluatePassRule(node, tasks)
}

// EvaluatePassRuleWithTasks evaluates the pass rule for a node using the provided tasks.
// This is used for simulation (e.g., checking if removing an assignee would deadlock the node).
func (e *FlowEngine) EvaluatePassRuleWithTasks(node *approval.FlowNode, tasks []approval.Task) (approval.PassRuleResult, error) {
	return e.evaluatePassRule(node, tasks)
}

// evaluatePassRule applies the node's pass rule to the given task set,
// including the deadlock guard. Both the DB-backed EvaluateNodeCompletion and
// the in-memory EvaluatePassRuleWithTasks (used by the remove-assignee
// simulation) route through here so the two cannot diverge on guard
// semantics — a removal the engine would let through must not be rejected by
// the simulation, and vice versa.
func (e *FlowEngine) evaluatePassRule(node *approval.FlowNode, tasks []approval.Task) (approval.PassRuleResult, error) {
	passStrategy, err := e.registry.GetPassRuleStrategy(node.PassRule)
	if err != nil {
		return approval.PassRulePending, err
	}

	prc := buildPassRuleContext(node, tasks)

	// Deadlock guard: if all tasks on this node ended up in non-actionable
	// states (transferred / canceled / removed / skipped / rolled_back),
	// no further decision is possible. Treat the node as passed so the
	// instance can advance instead of getting stuck at PassRulePending.
	if len(tasks) > 0 && prc.TotalCount == 0 {
		return approval.PassRulePassed, nil
	}

	return passStrategy.Evaluate(prc), nil
}

func buildPassRuleContext(node *approval.FlowNode, tasks []approval.Task) approval.PassRuleContext {
	ctx := approval.PassRuleContext{
		// PassRatio is stored as a percentage in (0, 100] — the single
		// storage convention enforced by deploy validation — and consumed
		// verbatim by RatioPassStrategy.
		PassRatio: node.PassRatio.InexactFloat64(),
	}

	for _, t := range tasks {
		// Exclude non-actionable tasks from the total count: transferred,
		// canceled, removed, skipped, and rolled_back no longer participate.
		switch t.Status {
		case approval.TaskTransferred, approval.TaskCanceled, approval.TaskRemoved, approval.TaskSkipped, approval.TaskRolledBack:
			continue
		}

		ctx.TotalCount++

		switch t.Status {
		case approval.TaskApproved, approval.TaskHandled:
			ctx.ApprovedCount++
		case approval.TaskRejected:
			ctx.RejectedCount++
		}
	}

	return ctx
}
