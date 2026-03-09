package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
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
