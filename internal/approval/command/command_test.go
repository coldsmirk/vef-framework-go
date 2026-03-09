package command_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// registry holds all command test suite factories, populated by init() in each suite file.
var registry = testx.NewRegistry[testx.DBEnv]()

// mustMarshal marshals v to json.RawMessage.
// It returns "{}" for unexpected marshal failures to keep test helpers side-effect free.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}

	return data
}

// simpleFlowDef returns a minimal valid flow definition: Start → End.
func simpleFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "end-1"},
		},
	}
}

// approvalFlowDef returns a flow definition with an approval node: Start → Approval → End.
func approvalFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"user-1", "user-2"}, SortOrder: 1},
					},
					CCs: []approval.CCDefinition{
						{Kind: approval.CCUser, IDs: []string{"cc-user-1"}, Timing: approval.CCTimingAlways},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
				ApprovalMethod: approval.ApprovalSequential,
				PassRule:       approval.PassAll,
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// baseFactory runs approval migrations and returns the DBEnv.
func baseFactory(env *testx.DBEnv) *testx.DBEnv {
	require.NoError(env.T, migration.Migrate(env.Ctx, env.DB, env.DS.Kind), "Should run approval migration")

	return env
}

// TestAll runs every registered command suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}
