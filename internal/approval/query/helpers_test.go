package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// QueryFixture holds the minimal set of records (category, flow, version)
// needed to satisfy FK constraints when directly inserting instances and tasks.
type QueryFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
	NodeIDs    []string
}

// setupQueryFixture creates a category → flow → version → nodes chain.
// NodeCount specifies how many approval nodes to create.
//

// ensureActiveVisit returns the node's open visit, recording one when the
// fixture has not opened it yet — every inserted task must bind to a visit.
//
//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func ensureActiveVisit(t testing.TB, ctx context.Context, db orm.DB, tenantID, instanceID, nodeID string) *approval.NodeVisit {
	t.Helper()

	var visit approval.NodeVisit

	err := db.NewSelect().Model(&visit).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("node_id", nodeID).
				Equals("status", approval.NodeVisitActive)
		}).
		Scan(ctx)
	if err == nil {
		return &visit
	}

	require.True(t, result.IsRecordNotFound(err), "Active-visit lookup should only miss, not fail: %v", err)

	count, err := db.NewSelect().Model((*approval.NodeVisit)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Count(ctx)
	require.NoError(t, err, "Should count instance visits")

	created := &approval.NodeVisit{
		TenantID:   tenantID,
		InstanceID: instanceID,
		NodeID:     nodeID,
		Sequence:   int(count) + 1,
		Status:     approval.NodeVisitActive,
	}
	_, err = db.NewInsert().Model(created).Exec(ctx)
	require.NoError(t, err, "Should insert node visit")

	return created
}

//nolint:revive // t testing.TB is conventionally the first parameter in test helpers
func setupQueryFixture(t testing.TB, ctx context.Context, db orm.DB, code string, nodeCount int) *QueryFixture {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     code + "-cat",
		Name:     code + " Category",
	}
	_, err := db.NewInsert().Model(category).Exec(ctx)
	require.NoError(t, err, "Should insert category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   code,
		Name:                   code + " Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert flow")

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should insert version")

	var nodeIDs []string
	for i := range nodeCount {
		node := &approval.FlowNode{
			FlowVersionID: version.ID,
			Key:           code + "-node-" + string(rune('a'+i)),
			Kind:          approval.NodeApproval,
			Name:          code + " Node",
		}
		_, err = db.NewInsert().Model(node).Exec(ctx)
		require.NoError(t, err, "Should insert node")

		nodeIDs = append(nodeIDs, node.ID)
	}

	return &QueryFixture{
		CategoryID: category.ID,
		FlowID:     flow.ID,
		VersionID:  version.ID,
		NodeIDs:    nodeIDs,
	}
}

// deleteAll removes all rows from the given models in order (FK-safe).
func deleteAll(ctx context.Context, db orm.DB, models ...any) {
	for _, model := range models {
		_, _ = db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	}
}

// cleanAllQueryData removes all approval data in FK-safe order.
func cleanAllQueryData(ctx context.Context, db orm.DB) {
	deleteAll(ctx, db,

		(*approval.ActionLog)(nil),
		(*approval.UrgeRecord)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.BusinessProjection)(nil),
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
