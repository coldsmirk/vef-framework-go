package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// registry holds all service test suite factories, populated by init() in each suite file.
var registry = testx.NewRegistry[testx.DBEnv]()

// baseFactory runs approval migrations and returns the DBEnv.
func baseFactory(env *testx.DBEnv) *testx.DBEnv {
	require.NoError(env.T, migration.Migrate(env.Ctx, env.DB, env.DS.Kind), "Should run approval migration")

	return env
}

// TestAll runs every registered service suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}

// --- Shared fixture ---

// SvcFixture holds IDs of records created to satisfy FK constraints.
type SvcFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
	NodeIDs    []string

	instanceSeq int
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func setupSvcFixture(t testing.TB, ctx context.Context, db orm.DB) *SvcFixture {
	t.Helper()

	cat := &approval.FlowCategory{TenantID: "default", Code: "svc-test-cat", Name: "Svc Test Cat"}
	_, err := db.NewInsert().Model(cat).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	flow := &approval.Flow{
		TenantID: "default", CategoryID: cat.ID, Code: "svc-test-flow", Name: "Svc Test Flow",
		BindingMode: approval.BindingStandalone, IsAllInitiationAllowed: true, IsActive: true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	var nodeIDs []string
	for i := range 6 {
		node := &approval.FlowNode{
			FlowVersionID: version.ID, Key: fmt.Sprintf("svc-node-%c", 'a'+i),
			Kind: approval.NodeApproval, Name: "Svc Node",
		}
		_, err = db.NewInsert().Model(node).Exec(ctx)
		require.NoError(t, err, "Should not return error")

		nodeIDs = append(nodeIDs, node.ID)
	}

	return &SvcFixture{CategoryID: cat.ID, FlowID: flow.ID, VersionID: version.ID, NodeIDs: nodeIDs}
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func (f *SvcFixture) createInstance(t testing.TB, ctx context.Context, db orm.DB, status approval.InstanceStatus) *approval.Instance {
	t.Helper()

	f.instanceSeq++
	inst := &approval.Instance{
		TenantID: "default", FlowID: f.FlowID, FlowVersionID: f.VersionID,
		Title: "Svc Test", InstanceNo: fmt.Sprintf("SVC-%04d", f.instanceSeq),
		ApplicantID: "applicant", Status: status,
	}
	_, err := db.NewInsert().Model(inst).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	return inst
}

// --- Shared insert helpers ---

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func insertTask(t testing.TB, ctx context.Context, db orm.DB, fix *SvcFixture, status approval.TaskStatus) *approval.Task {
	t.Helper()
	inst := fix.createInstance(t, ctx, db, approval.InstanceRunning)
	task := &approval.Task{
		TenantID: "default", InstanceID: inst.ID, NodeID: fix.NodeIDs[0],
		AssigneeID: "user-svc-test", SortOrder: 1, Status: status,
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	return task
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func insertTaskWithDetails(t testing.TB, ctx context.Context, db orm.DB, instanceID, nodeID string, status approval.TaskStatus, sortOrder int) *approval.Task {
	t.Helper()

	task := &approval.Task{
		TenantID: "default", InstanceID: instanceID, NodeID: nodeID,
		AssigneeID: fmt.Sprintf("user-default-%d", sortOrder), SortOrder: sortOrder, Status: status,
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	return task
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func insertTaskWithAssignee(t testing.TB, ctx context.Context, db orm.DB, instanceID, nodeID string, status approval.TaskStatus, sortOrder int, assigneeID string) *approval.Task {
	t.Helper()

	task := &approval.Task{
		TenantID: "default", InstanceID: instanceID, NodeID: nodeID,
		AssigneeID: assigneeID, SortOrder: sortOrder, Status: status,
	}
	_, err := db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	return task
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func setupPrepareOperationData(
	t testing.TB,
	ctx context.Context,
	db orm.DB,
	fix *SvcFixture,
	instanceStatus approval.InstanceStatus,
	taskStatus approval.TaskStatus,
	assigneeID string,
) (nodeID, instanceID, taskID string) {
	t.Helper()

	node := &approval.FlowNode{
		FlowVersionID: fix.VersionID,
		Key:           "prep-node-" + assigneeID,
		Kind:          approval.NodeApproval,
		Name:          "Prep Node",
	}
	_, err := db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	instance := &approval.Instance{
		TenantID: "default", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Prep Instance", InstanceNo: "PREP-" + assigneeID,
		ApplicantID: "applicant", Status: instanceStatus,
		CurrentNodeID: &node.ID,
	}
	_, err = db.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	task := &approval.Task{
		TenantID: "default", InstanceID: instance.ID, NodeID: node.ID,
		AssigneeID: assigneeID, SortOrder: 1, Status: taskStatus,
	}
	_, err = db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should not return error")

	return node.ID, instance.ID, task.ID
}

// --- Cleanup ---

func deleteAll(ctx context.Context, db orm.DB, models ...any) {
	for _, model := range models {
		_, _ = db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	}
}

// cleanAllServiceData removes all approval-related data from the database.
func cleanAllServiceData(ctx context.Context, db orm.DB) {
	deleteAll(ctx, db,
		(*approval.EventOutbox)(nil),
		(*approval.ActionLog)(nil),
		(*approval.UrgeRecord)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
		(*approval.FlowInitiator)(nil),
		(*approval.Flow)(nil),
		(*approval.FlowCategory)(nil),
	)
}
