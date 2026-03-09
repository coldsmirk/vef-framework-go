package resource_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/security"
)

// --- Mock implementations ---

// MockAssigneeService is a no-op implementation of approval.AssigneeService for testing.
type MockAssigneeService struct{}

func (*MockAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

func (*MockAssigneeService) GetDeptLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

func (*MockAssigneeService) GetRoleUsers(context.Context, string) ([]approval.UserInfo, error) {
	return nil, errors.New("not implemented")
}

// MockUserInfoResolver is a no-op implementation of approval.UserInfoResolver for testing.
type MockUserInfoResolver struct{}

func (*MockUserInfoResolver) ResolveUsers(_ context.Context, userIDs []string) (map[string]approval.UserInfo, error) {
	result := make(map[string]approval.UserInfo, len(userIDs))
	for _, id := range userIDs {
		result[id] = approval.UserInfo{ID: id, Name: id}
	}

	return result, nil
}

// MockPrincipalDeptResolver is a test implementation of approval.PrincipalDeptResolver.
type MockPrincipalDeptResolver struct{}

func (*MockPrincipalDeptResolver) Resolve(context.Context, *security.Principal) (deptID, deptName *string, err error) {
	return nil, nil, nil
}

// MockPermissionChecker always grants permission in tests.
type MockPermissionChecker struct{}

func (*MockPermissionChecker) HasPermission(context.Context, *security.Principal, string) (bool, error) {
	return true, nil
}

// MockInstanceNoGenerator is a test implementation of approval.InstanceNoGenerator.
type MockInstanceNoGenerator struct {
	counter atomic.Int64
}

func (g *MockInstanceNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	n := g.counter.Add(1)

	return fmt.Sprintf("%s-%04d", flowCode, n), nil
}

// --- App setup helper ---

// setupResourceApp creates a Postgres container, boots the full app with approval modules,
// and returns the orm.DB and a JWT token for authenticated requests.
func setupResourceApp(s *apptest.Suite) (orm.DB, string) {
	ctx := context.Background()
	pgContainer := testx.NewPostgresContainer(ctx, s.T())

	var db orm.DB
	s.SetupApp(
		fx.Replace(
			pgContainer.DataSource,
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
			&config.ApprovalConfig{AutoMigrate: true},
		),
		fx.Provide(func() context.Context { return ctx }),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return &MockUserInfoResolver{} }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDeptResolver { return &MockPrincipalDeptResolver{} }, fx.As(new(approval.PrincipalDeptResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Decorate(
			fx.Annotate(func() security.PermissionChecker { return &MockPermissionChecker{} }, fx.As(new(security.PermissionChecker))),
		),
		fx.Populate(&db),
	)

	token := s.GenerateToken(security.NewUser("test-admin", "admin", "admin"))

	return db, token
}

// --- Cleanup helpers ---

// deleteAll removes all rows from the given models in order (FK-safe).
func deleteAll(ctx context.Context, db orm.DB, models ...any) {
	for _, model := range models {
		_, _ = db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	}
}

// cleanRuntimeData removes runtime data (instances, tasks, etc.) while preserving flow definitions.
func cleanRuntimeData(ctx context.Context, db orm.DB) {
	deleteAll(ctx, db,
		(*approval.EventOutbox)(nil),
		(*approval.ActionLog)(nil),
		(*approval.UrgeRecord)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
	)
}

// cleanAllApprovalData removes all approval data in FK-safe order.
func cleanAllApprovalData(ctx context.Context, db orm.DB) {
	cleanRuntimeData(ctx, db)
	deleteAll(ctx, db,
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

// --- JSON helper ---

// mustMarshal marshals v to json.RawMessage.
// It returns "{}" for unexpected marshal failures to keep test helpers side-effect free.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}

	return data
}

// toMap converts a struct to map[string]any via JSON round-trip.
// This is needed because mapstructure cannot decode json.RawMessage from map values.
func toMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}
	}

	return m
}

// --- Flow definition builders ---

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
// The approval node has rollback, add/remove assignee, opinion required, transfer, and manual CC enabled.
func approvalFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"test-admin"}, SortOrder: 1},
					},
					CCs: []approval.CCDefinition{
						{Kind: approval.CCUser, IDs: []string{"cc-user-1"}, Timing: approval.CCTimingAlways},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
					IsTransferAllowed:   true,
					IsOpinionRequired:   true,
				},
				IsManualCCAllowed:       true,
				ApprovalMethod:          approval.ApprovalSequential,
				PassRule:                approval.PassAll,
				IsRollbackAllowed:       true,
				RollbackType:            approval.RollbackPrevious,
				IsAddAssigneeAllowed:    true,
				AddAssigneeTypes:        []approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeAfter, approval.AddAssigneeParallel},
				IsRemoveAssigneeAllowed: true,
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// complexFlowDef returns a flow with a condition node:
// Start → Condition → [branch "amount>1000": Approval → End] / [default: Handle → End].
func complexFlowDef() approval.FlowDefinition {
	branchHighID := "branch-high"
	branchDefaultID := "branch-default"

	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "condition-1", Kind: approval.NodeCondition, Data: mustMarshal(approval.ConditionNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "条件分支"},
				Branches: []approval.ConditionBranch{
					{
						ID:    branchHighID,
						Label: "大额审批",
						ConditionGroups: []approval.ConditionGroup{
							{Conditions: []approval.Condition{
								{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 1000},
							}},
						},
						Priority: 1,
					},
					{
						ID:        branchDefaultID,
						Label:     "默认处理",
						IsDefault: true,
						Priority:  2,
					},
				},
			})},
			{ID: "approval-high", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "大额审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"manager-1"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
					IsTransferAllowed:   true,
				},
				ApprovalMethod: approval.ApprovalSequential,
				PassRule:       approval.PassAll,
			})},
			{ID: "handle-default", Kind: approval.NodeHandle, Data: mustMarshal(approval.HandleNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "默认办理"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"handler-1"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "condition-1"},
			{ID: "edge-2", Source: "condition-1", Target: "approval-high", SourceHandle: &branchHighID},
			{ID: "edge-3", Source: "condition-1", Target: "handle-default", SourceHandle: &branchDefaultID},
			{ID: "edge-4", Source: "approval-high", Target: "end-1"},
			{ID: "edge-5", Source: "handle-default", Target: "end-1"},
		},
	}
}

// parallelApprovalFlowDef returns a flow with a parallel approval node:
// Start → Approval(parallel, passAll, 2 assignees: test-admin + approver-2) → End.
func parallelApprovalFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "会签审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"test-admin"}, SortOrder: 1},
						{Kind: approval.AssigneeUser, IDs: []string{"approver-2"}, SortOrder: 2},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
				ApprovalMethod: approval.ApprovalParallel,
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

// noManualCCFlowDef returns a flow with an approval node that disallows manual CC:
// Start → Approval(sequential, 1 assignee, IsManualCCAllowed=false) → End.
func noManualCCFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"test-admin"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
				IsManualCCAllowed: false,
				ApprovalMethod:    approval.ApprovalSequential,
				PassRule:          approval.PassAll,
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}
