package engine_test

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ProcessorTestBase provides shared test helpers for processor test suites
// (ApprovalProcessorTestSuite, HandleProcessorTestSuite).
// Embed this struct and call InitFKChain in SetupSuite to set up the FK chain.
type ProcessorTestBase struct {
	Ctx      context.Context
	DB       orm.DB
	Registry *strategy.StrategyRegistry

	FlowID        string
	FlowVersionID string
	NodeID        string
}

// InitRegistry creates a standard StrategyRegistry with a user assignee
// resolver and the built-in pass-rule strategies — the entry-time auto-pass
// sweep evaluates the node's pass rule, so the processors need them.
func (b *ProcessorTestBase) InitRegistry() {
	b.Registry = strategy.NewStrategyRegistry(
		[]approval.PassRuleStrategy{
			strategy.NewAllPassStrategy(),
			strategy.NewAnyPassStrategy(),
			strategy.NewRatioPassStrategy(),
		},
		[]strategy.AssigneeResolver{strategy.NewUserAssigneeResolver()},
		nil,
	)
}

// InitFKChain builds the FK chain: FlowCategory -> Flow -> FlowVersion -> FlowNode.
// The code and nodeKind parameters customize the chain for each test suite.
func (b *ProcessorTestBase) InitFKChain(t require.TestingT, code string, nodeKind approval.NodeKind, nodeName string) {
	category := &approval.FlowCategory{TenantID: "default", Code: code + "-cat", Name: nodeName + " Test"}
	_, err := b.DB.NewInsert().Model(category).Exec(b.Ctx)
	require.NoError(t, err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:              "default",
		CategoryID:            category.ID,
		Code:                  code + "-flow",
		Name:                  nodeName + " Test Flow",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "test",
	}
	_, err = b.DB.NewInsert().Model(flow).Exec(b.Ctx)
	require.NoError(t, err, "Should insert test flow")

	b.FlowID = flow.ID

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionDraft}
	_, err = b.DB.NewInsert().Model(version).Exec(b.Ctx)
	require.NoError(t, err, "Should insert test flow version")

	b.FlowVersionID = version.ID

	node := &approval.FlowNode{
		FlowVersionID:             version.ID,
		Key:                       code + "-node-1",
		Kind:                      nodeKind,
		Name:                      nodeName + " Node",
		ConsecutiveApproverAction: approval.ConsecutiveApproverNone,
	}
	_, err = b.DB.NewInsert().Model(node).Exec(b.Ctx)
	require.NoError(t, err, "Should insert test flow node")

	b.NodeID = node.ID
}

// CleanTransientData removes all transient test data in FK-safe order.
func (b *ProcessorTestBase) CleanTransientData(t require.TestingT) {
	for _, model := range []any{
		(*approval.Task)(nil),
		(*approval.FormSnapshot)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.Instance)(nil),
	} {
		_, err := b.DB.NewDelete().
			Model(model).
			Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
			Exec(b.Ctx)
		require.NoError(t, err, "Should clean transient data")
	}
}

// NewInstance creates and inserts a running test instance.
func (b *ProcessorTestBase) NewInstance(t require.TestingT, applicantID string) *approval.Instance {
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        b.FlowID,
		FlowVersionID: b.FlowVersionID,
		Title:         "Test Instance",
		InstanceNo:    "TEST-001",
		ApplicantID:   applicantID,
		Status:        approval.InstanceRunning,
	}
	_, err := b.DB.NewInsert().Model(instance).Exec(b.Ctx)
	require.NoError(t, err, "Should insert test instance")

	return instance
}

// NewNode builds a FlowNode value with the base's nodeID and optional
// overrides. PassRule mirrors the column default every deployed node carries —
// the entry-time auto-pass sweep evaluates it, so a fixture without one would
// exercise a node shape production never produces.
func (b *ProcessorTestBase) NewNode(opts ...func(*approval.FlowNode)) *approval.FlowNode {
	node := &approval.FlowNode{PassRule: approval.PassAll}
	node.ID = b.NodeID

	for _, opt := range opts {
		opt(node)
	}

	return node
}

// InsertAssigneeConfig inserts a user assignee config for the base's node.
func (b *ProcessorTestBase) InsertAssigneeConfig(t require.TestingT, userIDs []string) {
	cfg := &approval.FlowNodeAssignee{
		NodeID:    b.NodeID,
		Kind:      approval.AssigneeUser,
		IDs:       userIDs,
		SortOrder: 1,
	}
	_, err := b.DB.NewInsert().Model(cfg).Exec(b.Ctx)
	require.NoError(t, err, "Should insert assignee config")
}

// QueryTasks returns tasks for the given instance, ordered by sort_order.
func (b *ProcessorTestBase) QueryTasks(t require.TestingT, instanceID string) []approval.Task {
	var tasks []approval.Task

	err := b.DB.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sort_order").
		Scan(b.Ctx)
	require.NoError(t, err, "Should query tasks")

	return tasks
}

// QueryFormSnapshots returns form snapshots for the given instance.
func (b *ProcessorTestBase) QueryFormSnapshots(t require.TestingT, instanceID string) []approval.FormSnapshot {
	var snapshots []approval.FormSnapshot

	err := b.DB.NewSelect().
		Model(&snapshots).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Scan(b.Ctx)
	require.NoError(t, err, "Should query form snapshots")

	return snapshots
}

// InsertPreviousApprovalNode creates another approval node in the same flow version
// and returns its ID. Used for consecutive approver auto-pass tests.
func (b *ProcessorTestBase) InsertPreviousApprovalNode(t require.TestingT, key string) string {
	node := &approval.FlowNode{
		FlowVersionID: b.FlowVersionID,
		Key:           key,
		Kind:          approval.NodeApproval,
		Name:          "Previous Approval",
	}
	_, err := b.DB.NewInsert().Model(node).Exec(b.Ctx)
	require.NoError(t, err, "Should insert previous approval node")

	return node.ID
}

// EnsureVisit returns the instance's visit of the node with the given status,
// recording it when absent — every inserted task must bind to a visit.
func (b *ProcessorTestBase) EnsureVisit(t require.TestingT, instanceID, nodeID string, status approval.NodeVisitStatus) *approval.NodeVisit {
	var visit approval.NodeVisit

	err := b.DB.NewSelect().Model(&visit).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("status", status)
		}).
		Scan(b.Ctx)
	if err == nil {
		return &visit
	}

	require.True(t, result.IsRecordNotFound(err), "Visit lookup should only miss, not fail: %v", err)

	count, err := b.DB.NewSelect().Model((*approval.NodeVisit)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Count(b.Ctx)
	require.NoError(t, err, "Should count instance visits")

	created := &approval.NodeVisit{
		TenantID:   "default",
		InstanceID: instanceID,
		NodeID:     nodeID,
		Sequence:   int(count) + 1,
		Status:     status,
	}
	_, err = b.DB.NewInsert().Model(created).Exec(b.Ctx)
	require.NoError(t, err, "Should insert node visit")

	return created
}

// InsertApprovedTasks creates approved tasks for a given node/instance, bound
// to a passed visit of that node.
func (b *ProcessorTestBase) InsertApprovedTasks(t require.TestingT, instanceID, nodeID string, assigneeIDs []string) {
	now := timex.Now()
	visit := b.EnsureVisit(t, instanceID, nodeID, approval.NodeVisitPassed)

	for _, assigneeID := range assigneeIDs {
		task := &approval.Task{
			TenantID:   "default",
			InstanceID: instanceID,
			NodeID:     nodeID,
			VisitID:    visit.ID,
			AssigneeID: assigneeID,
			Status:     approval.TaskApproved,
			FinishedAt: new(now),
		}
		_, err := b.DB.NewInsert().Model(task).Exec(b.Ctx)
		require.NoError(t, err, "Should insert approved task")
	}
}

// InsertRejectedTasks creates rejected tasks for a given node/instance, bound
// to a rejected visit of that node.
func (b *ProcessorTestBase) InsertRejectedTasks(t require.TestingT, instanceID, nodeID string, assigneeIDs []string) {
	now := timex.Now()
	visit := b.EnsureVisit(t, instanceID, nodeID, approval.NodeVisitRejected)

	for _, assigneeID := range assigneeIDs {
		task := &approval.Task{
			TenantID:   "default",
			InstanceID: instanceID,
			NodeID:     nodeID,
			VisitID:    visit.ID,
			AssigneeID: assigneeID,
			Status:     approval.TaskRejected,
			FinishedAt: new(now),
		}
		_, err := b.DB.NewInsert().Model(task).Exec(b.Ctx)
		require.NoError(t, err, "Should insert rejected task")
	}
}

// NewProcessContext creates a ProcessContext from the given instance and node,
// opening the node's visit exactly as ProcessNode would before dispatching to
// a processor.
func (b *ProcessorTestBase) NewProcessContext(t require.TestingT, instance *approval.Instance, node *approval.FlowNode) *engine.ProcessContext {
	return &engine.ProcessContext{
		DB:          b.DB,
		Instance:    instance,
		Node:        node,
		Visit:       b.EnsureVisit(t, instance.ID, node.ID, approval.NodeVisitActive),
		FormData:    approval.NewFormData(instance.FormData),
		ApplicantID: instance.ApplicantID,
		Registry:    b.Registry,
	}
}
