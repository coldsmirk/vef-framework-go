package engine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/migration"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// StubProcessor is a configurable mock NodeProcessor for constructor tests.
type StubProcessor struct {
	kind approval.NodeKind
	err  error
}

func (p *StubProcessor) NodeKind() approval.NodeKind { return p.kind }

func (p *StubProcessor) Process(context.Context, *engine.ProcessContext) (*engine.ProcessResult, error) {
	return nil, p.err
}

// --- Suite Infrastructure ---

// registry holds all engine test suite factories, populated by init() in each suite file.
var registry = testx.NewRegistry[testx.DBEnv]()

// baseFactory runs approval migrations and returns the DBEnv.
func baseFactory(env *testx.DBEnv) *testx.DBEnv {
	require.NoError(env.T, migration.Migrate(env.Ctx, env.DB, env.DS.Kind), "Should run approval migration")

	return env
}

// TestAll runs every registered engine suite against all configured databases.
// Test hierarchy: TestAll/<DBDisplayName>/<SuiteName>/...
func TestAll(t *testing.T) {
	registry.RunAll(t, baseFactory)
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FlowEngineTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// --- Standalone Tests (no DB required) ---

// TestNormalizePassRatio tests normalize pass ratio scenarios.
func TestNormalizePassRatio(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"Negative", -1.0, 0},
		{"Zero", 0, 0},
		{"ProportionHalf", 0.5, 50},
		{"ProportionSixtyPercent", 0.6, 60},
		{"ProportionOne", 1.0, 100},
		{"PercentageFifty", 50, 50},
		{"PercentageHundred", 100, 100},
		{"AboveHundred", 150, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.expected, engine.NormalizePassRatio(tt.input), 0.001, "NormalizePassRatio(%v) should produce expected result", tt.input)
		})
	}
}

// TestNewFlowEngine tests new flow engine constructor via behavior.
func TestNewFlowEngine(t *testing.T) {
	t.Run("EmptyProcessors", func(t *testing.T) {
		eng := engine.NewFlowEngine(nil, nil, nil, nil)
		require.NotNil(t, eng, "Should create engine with nil processors")

		node := &approval.FlowNode{Kind: approval.NodeStart, Name: "Start"}
		node.ID = "test-start"
		err := eng.ProcessNode(t.Context(), nil, &approval.Instance{}, node)
		assert.ErrorIs(t, err, engine.ErrProcessorNotFound, "Should fail for any node kind")
	})

	t.Run("RegistersProcessors", func(t *testing.T) {
		stubErr := errors.New("stub reached")
		eng := engine.NewFlowEngine(nil, []engine.NodeProcessor{
			&StubProcessor{kind: approval.NodeStart, err: stubErr},
		}, nil, nil)

		node := &approval.FlowNode{Kind: approval.NodeStart, Name: "Start"}
		node.ID = "test-start"
		err := eng.ProcessNode(t.Context(), nil, &approval.Instance{}, node)
		assert.ErrorIs(t, err, stubErr, "Should reach the registered processor")
		assert.NotErrorIs(t, err, engine.ErrProcessorNotFound, "Should not be processor-not-found")
	})

	t.Run("AllProcessorTypes", func(t *testing.T) {
		kinds := []approval.NodeKind{
			approval.NodeStart, approval.NodeEnd, approval.NodeApproval,
			approval.NodeCC, approval.NodeCondition, approval.NodeHandle,
		}

		var procs []engine.NodeProcessor
		for _, k := range kinds {
			procs = append(procs, &StubProcessor{kind: k, err: errors.New("reached-" + string(k))})
		}

		eng := engine.NewFlowEngine(nil, procs, nil, nil)
		for _, k := range kinds {
			node := &approval.FlowNode{Kind: k, Name: string(k)}
			node.ID = "test-" + string(k)
			err := eng.ProcessNode(t.Context(), nil, &approval.Instance{}, node)
			assert.NotErrorIs(t, err, engine.ErrProcessorNotFound, "Processor for %s should be found", k)
		}
	})

	t.Run("DuplicateOverrides", func(t *testing.T) {
		errFirst := errors.New("first")
		errSecond := errors.New("second")
		eng := engine.NewFlowEngine(nil, []engine.NodeProcessor{
			&StubProcessor{kind: approval.NodeStart, err: errFirst},
			&StubProcessor{kind: approval.NodeStart, err: errSecond},
		}, nil, nil)

		node := &approval.FlowNode{Kind: approval.NodeStart, Name: "Start"}
		node.ID = "test-start"
		err := eng.ProcessNode(t.Context(), nil, &approval.Instance{}, node)
		assert.ErrorIs(t, err, errSecond, "Last registered processor should win")
		assert.NotErrorIs(t, err, errFirst, "First processor should be overridden")
	})
}

// TestEvaluatePassRuleWithTasks tests evaluate pass rule with tasks scenarios.
func TestEvaluatePassRuleWithTasks(t *testing.T) {
	reg := strategy.NewStrategyRegistry(
		[]approval.PassRuleStrategy{strategy.NewAllPassStrategy()},
		nil,
		nil,
	)
	eng := engine.NewFlowEngine(reg, nil, nil, nil)
	node := &approval.FlowNode{PassRule: approval.PassAll, PassRatio: decimal.NewFromInt(0)}

	t.Run("AllApproved", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskApproved},
		}
		result, err := eng.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePassed, result, "Should pass when all tasks approved")
	})

	t.Run("HasPending", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskPending},
		}
		result, err := eng.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePending, result, "Should be pending when one task is pending")
	})

	t.Run("HasRejected", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskRejected},
		}
		result, err := eng.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRuleRejected, result, "Should reject when one task is rejected")
	})

	t.Run("ExcludesNonActionable", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskTransferred},
			{Status: approval.TaskCanceled},
			{Status: approval.TaskRemoved},
			{Status: approval.TaskSkipped},
		}
		result, err := eng.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePassed, result, "Should pass: only actionable task is approved, non-actionable excluded")
	})

	t.Run("HandledCountsAsApproved", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskHandled},
			{Status: approval.TaskApproved},
		}
		result, err := eng.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePassed, result, "Should pass when handled counts as approved")
	})

	t.Run("UnknownRule", func(t *testing.T) {
		unknownNode := &approval.FlowNode{PassRule: "nonexistent", PassRatio: decimal.NewFromInt(0)}
		_, err := eng.EvaluatePassRuleWithTasks(unknownNode, nil)
		assert.Error(t, err, "Should return error for unknown pass rule")
	})
}

// --- FlowEngineTestSuite (DB-backed) ---

// FlowEngineTestSuite tests FlowEngine DB-dependent methods with a real database.
type FlowEngineTestSuite struct {
	suite.Suite

	ctx    context.Context
	db     orm.DB
	engine *engine.FlowEngine

	flowID, flowVersionID                  string
	startNodeID, approvalNodeID, endNodeID string

	instanceSeq int
}

func (s *FlowEngineTestSuite) SetupSuite() {
	reg := strategy.NewStrategyRegistry(
		[]approval.PassRuleStrategy{strategy.NewAllPassStrategy()},
		[]strategy.AssigneeResolver{strategy.NewUserAssigneeResolver()},
		nil,
	)

	s.engine = engine.NewFlowEngine(reg, []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewApprovalProcessor(nil),
	}, nil, nil)

	// Build FK chain: FlowCategory → Flow → FlowVersion
	category := &approval.FlowCategory{TenantID: "default", Code: "engine-test", Name: "Engine Test"}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:              "default",
		CategoryID:            category.ID,
		Code:                  "engine-test-flow",
		Name:                  "Engine Test Flow",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "test",
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")
	s.flowID = flow.ID

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionDraft}
	_, err = s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow version")
	s.flowVersionID = version.ID

	// Create nodes: start → approval → end
	startNode := &approval.FlowNode{
		FlowVersionID:             version.ID,
		Key:                       "start",
		Kind:                      approval.NodeStart,
		Name:                      "Start",
		ConsecutiveApproverAction: approval.ConsecutiveApproverNone,
	}
	_, err = s.db.NewInsert().Model(startNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert start node")
	s.startNodeID = startNode.ID

	approvalNode := &approval.FlowNode{
		FlowVersionID:             version.ID,
		Key:                       "approval",
		Kind:                      approval.NodeApproval,
		Name:                      "Approval",
		PassRule:                  approval.PassAll,
		PassRatio:                 decimal.NewFromInt(0),
		EmptyAssigneeAction:       approval.EmptyAssigneeAutoPass,
		ConsecutiveApproverAction: approval.ConsecutiveApproverNone,
	}
	_, err = s.db.NewInsert().Model(approvalNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert approval node")
	s.approvalNodeID = approvalNode.ID

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		Key:           "end",
		Kind:          approval.NodeEnd,
		Name:          "End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert end node")
	s.endNodeID = endNode.ID

	// Create edges: start→approval, approval→end
	edges := []*approval.FlowEdge{
		{
			FlowVersionID: version.ID,
			Key:           "e1",
			SourceNodeID:  startNode.ID,
			SourceNodeKey: "start",
			TargetNodeID:  approvalNode.ID,
			TargetNodeKey: "approval",
		},
		{
			FlowVersionID: version.ID,
			Key:           "e2",
			SourceNodeID:  approvalNode.ID,
			SourceNodeKey: "approval",
			TargetNodeID:  endNode.ID,
			TargetNodeKey: "end",
		},
	}
	for _, edge := range edges {
		_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
		s.Require().NoError(err, "Should insert edge %s", edge.Key)
	}
}

// cleanTransientData removes all transient test data (respect FK order).
func (s *FlowEngineTestSuite) cleanTransientData() {
	for _, model := range []any{
		(*approval.Task)(nil),
		(*approval.FormSnapshot)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.Instance)(nil),
	} {
		_, err := s.db.NewDelete().
			Model(model).
			Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should clean transient data")
	}
}

func (s *FlowEngineTestSuite) TearDownTest() {
	s.cleanTransientData()
}

func (s *FlowEngineTestSuite) newInstance(applicantID string) *approval.Instance {
	s.instanceSeq++
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.flowID,
		FlowVersionID: s.flowVersionID,
		Title:         "Engine Test Instance",
		InstanceNo:    fmt.Sprintf("ENG-%04d", s.instanceSeq),
		ApplicantID:   applicantID,
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test instance")

	return instance
}

func (s *FlowEngineTestSuite) insertAssigneeConfig(userIDs []string) {
	cfg := &approval.FlowNodeAssignee{
		NodeID:    s.approvalNodeID,
		Kind:      approval.AssigneeUser,
		IDs:       userIDs,
		SortOrder: 1,
	}
	_, err := s.db.NewInsert().Model(cfg).Exec(s.ctx)
	s.Require().NoError(err, "Should insert assignee config")
}

func (s *FlowEngineTestSuite) reloadInstance(id string) *approval.Instance {
	var instance approval.Instance

	instance.ID = id

	err := s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should reload instance")

	return &instance
}

func (s *FlowEngineTestSuite) queryTasks(instanceID string) []approval.Task {
	var tasks []approval.Task

	err := s.db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query tasks")

	return tasks
}

func (s *FlowEngineTestSuite) TestStartProcess() {
	s.Run("HappyPath", func() {
		defer s.cleanTransientData()

		s.insertAssigneeConfig([]string{"user-1"})

		instance := s.newInstance("applicant-1")
		err := s.engine.StartProcess(s.ctx, s.db, instance)
		s.Require().NoError(err, "Should start process without error")

		// Start auto-advances → approval node creates task → waits
		reloaded := s.reloadInstance(instance.ID)
		s.Assert().NotNil(reloaded.CurrentNodeID, "Should set current node")
		s.Assert().Equal(s.approvalNodeID, *reloaded.CurrentNodeID, "Should be at approval node")

		tasks := s.queryTasks(instance.ID)
		s.Assert().Len(tasks, 1, "Should create one task")
		s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task should be assigned to user-1")
	})

	s.Run("EmptyAssigneeAutoPass", func() {
		defer s.cleanTransientData()

		// No assignee configs → EmptyAssigneeAutoPass → continues to end
		instance := s.newInstance("applicant-1")
		err := s.engine.StartProcess(s.ctx, s.db, instance)
		s.Require().NoError(err, "Should start process without error")

		reloaded := s.reloadInstance(instance.ID)
		s.Assert().Equal(s.endNodeID, *reloaded.CurrentNodeID, "Should reach end node")
		s.Assert().Equal(approval.InstanceApproved, reloaded.Status, "Should be approved (auto-pass through)")
		s.Assert().NotNil(reloaded.FinishedAt, "Should set finished time")
	})
}

func (s *FlowEngineTestSuite) TestProcessNode() {
	s.Run("ProcessorNotFound", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		// Use a node kind not registered in the engine
		node := &approval.FlowNode{Kind: approval.NodeCC, Name: "CC"}
		node.ID = "fake-node-id"

		err := s.engine.ProcessNode(s.ctx, s.db, instance, node)
		s.Require().Error(err, "Should error for unknown processor")
		s.Assert().ErrorIs(err, engine.ErrProcessorNotFound, "Should be ErrProcessorNotFound")
	})
}

func (s *FlowEngineTestSuite) TestAdvanceToNextNode() {
	s.Run("NoMatchingEdge", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		// End node has no outgoing edges
		endNode := &approval.FlowNode{Kind: approval.NodeEnd, Name: "End"}
		endNode.ID = s.endNodeID

		err := s.engine.AdvanceToNextNode(s.ctx, s.db, instance, endNode, nil)
		s.Require().Error(err, "Should error when no outgoing edge")
		s.Assert().ErrorIs(err, engine.ErrNoMatchingEdge, "Should be ErrNoMatchingEdge")
	})

	s.Run("NormalAdvance", func() {
		defer s.cleanTransientData()

		s.insertAssigneeConfig([]string{"user-1"})

		instance := s.newInstance("applicant-1")
		startNode := &approval.FlowNode{Kind: approval.NodeStart, Name: "Start"}
		startNode.ID = s.startNodeID

		// Advance from start → should go to approval node and wait
		err := s.engine.AdvanceToNextNode(s.ctx, s.db, instance, startNode, nil)
		s.Require().NoError(err, "Should advance without error")

		reloaded := s.reloadInstance(instance.ID)
		s.Assert().Equal(s.approvalNodeID, *reloaded.CurrentNodeID, "Should be at approval node")
	})
}

func (s *FlowEngineTestSuite) TestEvaluateNodeCompletion() {
	s.Run("AllApproved", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		node := &approval.FlowNode{
			PassRule:  approval.PassAll,
			PassRatio: decimal.NewFromInt(0),
		}
		node.ID = s.approvalNodeID

		// Insert approved tasks
		for _, uid := range []string{"user-1", "user-2"} {
			task := &approval.Task{
				TenantID:   "default",
				InstanceID: instance.ID,
				NodeID:     s.approvalNodeID,
				AssigneeID: uid,
				Status:     approval.TaskApproved,
			}
			_, err := s.db.NewInsert().Model(task).Exec(s.ctx)
			s.Require().NoError(err, "Should insert task")
		}

		result, err := s.engine.EvaluateNodeCompletion(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should evaluate without error")
		s.Assert().Equal(approval.PassRulePassed, result, "Should pass when all tasks approved")
	})

	s.Run("HasPending", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		node := &approval.FlowNode{
			PassRule:  approval.PassAll,
			PassRatio: decimal.NewFromInt(0),
		}
		node.ID = s.approvalNodeID

		tasks := []approval.Task{
			{TenantID: "default", InstanceID: instance.ID, NodeID: s.approvalNodeID, AssigneeID: "user-1", Status: approval.TaskApproved},
			{TenantID: "default", InstanceID: instance.ID, NodeID: s.approvalNodeID, AssigneeID: "user-2", Status: approval.TaskPending},
		}
		for i := range tasks {
			_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
			s.Require().NoError(err, "Should insert task")
		}

		result, err := s.engine.EvaluateNodeCompletion(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should evaluate without error")
		s.Assert().Equal(approval.PassRulePending, result, "Should be pending when one task is pending")
	})

	s.Run("HasRejected", func() {
		defer s.cleanTransientData()

		instance := s.newInstance("applicant-1")
		node := &approval.FlowNode{
			PassRule:  approval.PassAll,
			PassRatio: decimal.NewFromInt(0),
		}
		node.ID = s.approvalNodeID

		tasks := []approval.Task{
			{TenantID: "default", InstanceID: instance.ID, NodeID: s.approvalNodeID, AssigneeID: "user-1", Status: approval.TaskApproved},
			{TenantID: "default", InstanceID: instance.ID, NodeID: s.approvalNodeID, AssigneeID: "user-2", Status: approval.TaskRejected},
		}
		for i := range tasks {
			_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
			s.Require().NoError(err, "Should insert task")
		}

		result, err := s.engine.EvaluateNodeCompletion(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should evaluate without error")
		s.Assert().Equal(approval.PassRuleRejected, result, "Should reject when one task is rejected")
	})
}

func (s *FlowEngineTestSuite) TestHandleProcessResult() {
	s.Run("WaitAction", func() {
		defer s.cleanTransientData()

		s.insertAssigneeConfig([]string{"user-1"})

		instance := s.newInstance("applicant-1")
		err := s.engine.StartProcess(s.ctx, s.db, instance)
		s.Require().NoError(err, "Should start process")

		// After start → approval with assignee → should wait
		reloaded := s.reloadInstance(instance.ID)
		s.Assert().Equal(approval.InstanceRunning, reloaded.Status, "Should still be running (waiting)")
		s.Assert().Equal(s.approvalNodeID, *reloaded.CurrentNodeID, "Should be at approval node")
	})

	s.Run("CompleteAction", func() {
		defer s.cleanTransientData()

		// No assignees → auto-pass → end node → complete
		instance := s.newInstance("applicant-1")
		err := s.engine.StartProcess(s.ctx, s.db, instance)
		s.Require().NoError(err, "Should start process")

		reloaded := s.reloadInstance(instance.ID)
		s.Assert().Equal(approval.InstanceApproved, reloaded.Status, "Should be approved")
		s.Assert().NotNil(reloaded.FinishedAt, "Should have finished time")
		s.Assert().Equal(s.endNodeID, *reloaded.CurrentNodeID, "Should be at end node")
	})
}
