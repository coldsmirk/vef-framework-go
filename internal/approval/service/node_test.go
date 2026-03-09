package service_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &NodeServiceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// NodeServiceTestSuite tests NodeService behavior.
type NodeServiceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	svc     *service.NodeService
	fixture *SvcFixture
}

func (s *NodeServiceTestSuite) SetupSuite() {
	passRules := []approval.PassRuleStrategy{
		strategy.NewAllPassStrategy(),
		strategy.NewOnePassStrategy(),
		strategy.NewRatioPassStrategy(),
		strategy.NewOneRejectStrategy(),
	}

	assigneeResolvers := []strategy.AssigneeResolver{
		strategy.NewUserAssigneeResolver(),
		strategy.NewSelfAssigneeResolver(),
	}

	registry := strategy.NewStrategyRegistry(passRules, assigneeResolvers, nil)
	processors := []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewConditionProcessor(),
		engine.NewApprovalProcessor(nil),
		engine.NewHandleProcessor(nil),
		engine.NewCCProcessor(),
	}

	eng := engine.NewFlowEngine(registry, processors, dispatcher.NewEventPublisher(), nil)
	taskSvc := service.NewTaskService()
	s.svc = service.NewNodeService(eng, dispatcher.NewEventPublisher(), taskSvc, nil)
	s.fixture = setupSvcFixture(s.T(), s.ctx, s.db)
}

func (s *NodeServiceTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.FormSnapshot)(nil),
		(*approval.EventOutbox)(nil),
		(*approval.ActionLog)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNode)(nil),
	)
}

func (s *NodeServiceTestSuite) TearDownSuite() {
	cleanAllServiceData(s.ctx, s.db)
}

func (s *NodeServiceTestSuite) TestCheckCCNodeCompletionShouldNotAdvanceTwice() {
	ccNode := &approval.FlowNode{
		FlowVersionID:         s.fixture.VersionID,
		Key:                   "svc-cc-node",
		Kind:                  approval.NodeCC,
		Name:                  "CC Node",
		IsReadConfirmRequired: true,
	}
	_, err := s.db.NewInsert().Model(ccNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert CC node")

	nextNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "svc-next-approval-node",
		Kind:          approval.NodeApproval,
		Name:          "Next Approval Node",
	}
	_, err = s.db.NewInsert().Model(nextNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert next approval node")

	edge := &approval.FlowEdge{
		FlowVersionID: s.fixture.VersionID,
		SourceNodeID:  ccNode.ID,
		TargetNodeID:  nextNode.ID,
	}
	_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
	s.Require().NoError(err, "Should insert CC to approval edge")

	assignee := &approval.FlowNodeAssignee{
		NodeID: nextNode.ID,
		Kind:   approval.AssigneeUser,
		IDs:    []string{"next-approver"},
	}
	_, err = s.db.NewInsert().Model(assignee).Exec(s.ctx)
	s.Require().NoError(err, "Should insert assignee config for next node")

	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Completion Instance",
		InstanceNo:    "SVC-CC-001",
		ApplicantID:   "applicant",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &ccNode.ID,
		FormData:      map[string]any{"amount": float64(100)},
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance")

	readAt := timex.Now()
	ccRecord := &approval.CCRecord{
		InstanceID: instance.ID,
		NodeID:     &ccNode.ID,
		CCUserID:   "cc-user",
		IsManual:   false,
		ReadAt:     &readAt,
	}
	_, err = s.db.NewInsert().Model(ccRecord).Exec(s.ctx)
	s.Require().NoError(err, "Should insert read CC record")

	records := []approval.CCRecord{
		{
			NodeID: ccRecord.NodeID,
		},
	}

	err = s.svc.CheckCCNodeCompletion(s.ctx, s.db, instance.ID, records)
	s.Require().NoError(err, "First completion check should advance flow")

	err = s.svc.CheckCCNodeCompletion(s.ctx, s.db, instance.ID, records)
	s.Require().NoError(err, "Second completion check should be idempotent")

	var nextTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&nextTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID).
					Equals("node_id", nextNode.ID)
			}).
			Scan(s.ctx),
		"Should query next node tasks",
	)
	s.Require().Len(nextTasks, 1, "Repeated completion checks should not create duplicate next-node tasks")

	updatedInstance := &approval.Instance{}
	updatedInstance.ID = instance.ID
	s.Require().NoError(
		s.db.NewSelect().
			Model(updatedInstance).
			WherePK().
			Scan(s.ctx),
		"Should reload instance after completion checks",
	)
	s.Require().NotNil(updatedInstance.CurrentNodeID, "Instance should have current node after advancing")
	s.Assert().Equal(nextNode.ID, *updatedInstance.CurrentNodeID, "Instance should advance to next node once")
}

func (s *NodeServiceTestSuite) TestCheckCCNodeCompletionShouldFailWhenCCNodeMissing() {
	missingNodeID := "missing-cc-node"
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Missing CC Node Instance",
		InstanceNo:    "SVC-CC-MISSING-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &missingNodeID,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance that points to missing CC node")

	records := []approval.CCRecord{
		{
			NodeID: &missingNodeID,
		},
	}

	err = s.svc.CheckCCNodeCompletion(s.ctx, s.db, instance.ID, records)
	s.Require().Error(err, "Should return error when referenced CC node is missing")
	s.Assert().ErrorContains(err, "load cc node", "Error should include missing CC node load context")
}

func (s *NodeServiceTestSuite) TestTriggerNodeCCShouldRespectTimingAndDeduplicate() {
	instance := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "svc-trigger-cc-node",
		Kind:          approval.NodeApproval,
		Name:          "Trigger CC Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert node for trigger-node-cc scenario")

	ccConfigs := []approval.FlowNodeCC{
		{
			NodeID: node.ID,
			Kind:   approval.CCUser,
			IDs:    []string{"cc-user-1", "cc-user-2"},
			Timing: approval.CCTimingAlways,
		},
		{
			NodeID: node.ID,
			Kind:   approval.CCUser,
			IDs:    []string{"cc-user-2", "cc-user-3"},
			Timing: approval.CCTimingOnApprove,
		},
		{
			NodeID: node.ID,
			Kind:   approval.CCUser,
			IDs:    []string{"cc-user-4"},
			Timing: approval.CCTimingOnReject,
		},
	}
	_, err = s.db.NewInsert().Model(&ccConfigs).Exec(s.ctx)
	s.Require().NoError(err, "Should insert CC configs with mixed timing and duplicates")

	err = s.svc.TriggerNodeCC(s.ctx, s.db, instance, node, approval.PassRulePassed)
	s.Require().NoError(err, "Should trigger CC notifications for approved node completion")

	var records []approval.CCRecord
	s.Require().NoError(
		s.db.NewSelect().
			Model(&records).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID).
					Equals("node_id", node.ID)
			}).
			Scan(s.ctx),
		"Should query inserted CC records",
	)
	s.Require().Len(records, 3, "Should insert deduplicated CC records for always + on-approve configs")

	recordUserIDs := make([]string, 0, len(records))
	for i := range records {
		recordUserIDs = append(recordUserIDs, records[i].CCUserID)
	}

	s.Assert().ElementsMatch(
		[]string{"cc-user-1", "cc-user-2", "cc-user-3"},
		recordUserIDs,
		"Should include only deduplicated users from applicable timing configs",
	)

	notifiedCount, err := s.db.NewSelect().
		Model((*approval.EventOutbox)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.cc.notified") }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count CC notified events")
	s.Assert().Equal(int64(1), notifiedCount, "Should emit a single CC notification event for merged recipients")
}

func (s *NodeServiceTestSuite) TestTriggerNodeCCShouldIgnoreExistingRecordsAndPublishOnlyNewUsers() {
	instance := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
	node := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "svc-trigger-cc-existing-node",
		Kind:          approval.NodeApproval,
		Name:          "Trigger CC Existing Node",
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should insert node for existing-cc dedup scenario")

	ccConfigs := []approval.FlowNodeCC{
		{
			NodeID: node.ID,
			Kind:   approval.CCUser,
			IDs:    []string{"cc-user-existing", "cc-user-new"},
			Timing: approval.CCTimingAlways,
		},
	}
	_, err = s.db.NewInsert().Model(&ccConfigs).Exec(s.ctx)
	s.Require().NoError(err, "Should insert CC config for existing-record dedup scenario")

	_, err = s.db.NewInsert().Model(&approval.CCRecord{
		InstanceID: instance.ID,
		NodeID:     &node.ID,
		CCUserID:   "cc-user-existing",
		IsManual:   true,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert existing manual CC record")

	err = s.svc.TriggerNodeCC(s.ctx, s.db, instance, node, approval.PassRulePassed)
	s.Require().NoError(err, "Triggering node CC should ignore existing records instead of failing")

	var records []approval.CCRecord
	s.Require().NoError(
		s.db.NewSelect().
			Model(&records).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instance.ID).
					Equals("node_id", node.ID)
			}).
			OrderBy("created_at").
			Scan(s.ctx),
		"Should query CC records after trigger",
	)
	s.Require().Len(records, 2, "Should keep one existing record and insert only one new record")

	var outbox approval.EventOutbox
	s.Require().NoError(
		s.db.NewSelect().
			Model(&outbox).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("event_type", "approval.cc.notified")
			}).
			OrderByDesc("created_at").
			Limit(1).
			Scan(s.ctx),
		"Should query latest cc-notified outbox event",
	)

	rawIDs, ok := outbox.Payload["ccUserIds"].([]any)
	s.Require().True(ok, "Outbox payload should contain ccUserIds array")
	s.Require().Len(rawIDs, 1, "CC event should include only newly inserted users")
	s.Assert().Equal("cc-user-new", rawIDs[0], "CC event should exclude already existing CC users")
}
