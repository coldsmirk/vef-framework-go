package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &MarkCCReadTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// MarkCCReadTestSuite tests the MarkCCReadHandler.
type MarkCCReadTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	handler   *command.MarkCCReadHandler
	fixture   *MinimalFixture
	ccNodeID  string
	endNodeID string
}

func (s *MarkCCReadTestSuite) SetupSuite() {
	eng := buildTestEngine()
	_, nodeSvc, _ := buildTestServices(eng)
	s.handler = command.NewMarkCCReadHandler(s.db, nodeSvc)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "mark-cc")

	ccNode := &approval.FlowNode{
		FlowVersionID:         s.fixture.VersionID,
		Key:                   "mark-cc-node",
		Kind:                  approval.NodeCC,
		Name:                  "Mark CC Node",
		IsReadConfirmRequired: true,
	}
	_, err := s.db.NewInsert().Model(ccNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create CC node for read-confirm scenarios")
	s.ccNodeID = ccNode.ID

	endNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "mark-cc-end",
		Kind:          approval.NodeEnd,
		Name:          "Mark CC End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create end node for read-confirm scenarios")
	s.endNodeID = endNode.ID

	edge := &approval.FlowEdge{
		FlowVersionID: s.fixture.VersionID,
		SourceNodeID:  ccNode.ID,
		SourceNodeKey: ccNode.Key,
		TargetNodeID:  endNode.ID,
		TargetNodeKey: endNode.Key,
	}
	_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
	s.Require().NoError(err, "Should create edge from CC node to end node")
}

func (s *MarkCCReadTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db, (*approval.EventOutbox)(nil), (*approval.CCRecord)(nil), (*approval.Instance)(nil))
}

func (s *MarkCCReadTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *MarkCCReadTestSuite) createInstance(no string, currentNodeID *string) string {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Read Test",
		InstanceNo:    no,
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: currentNodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst.ID
}

func (s *MarkCCReadTestSuite) TestMarkReadSuccess() {
	instID := s.createInstance("MCC-001", nil)

	// Insert unread CC records
	records := []approval.CCRecord{
		{InstanceID: instID, CCUserID: "cc-user-1", IsManual: false},
		{InstanceID: instID, CCUserID: "cc-user-1", IsManual: true},
		{InstanceID: instID, CCUserID: "cc-user-2", IsManual: false},
	}
	for i := range records {
		_, err := s.db.NewInsert().Model(&records[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should not return error")
	}

	_, err := s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-1",
	})
	s.Require().NoError(err, "Should mark CC as read without error")

	// Verify cc-user-1's records are marked as read
	var updatedRecords []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&updatedRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instID).
				Equals("cc_user_id", "cc-user-1")
		}).
		Scan(s.ctx), "Should not return error")

	for _, r := range updatedRecords {
		s.Assert().NotNil(r.ReadAt, "Should set ReadAt for cc-user-1")
	}

	// Verify cc-user-2's records are untouched
	var otherRecords []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&otherRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instID).
				Equals("cc_user_id", "cc-user-2")
		}).
		Scan(s.ctx), "Should not return error")

	for _, r := range otherRecords {
		s.Assert().Nil(r.ReadAt, "Should not set ReadAt for cc-user-2")
	}
}

func (s *MarkCCReadTestSuite) TestMarkReadNoRecords() {
	_, err := s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: "non-existent-instance",
		UserID:     "cc-user-1",
	})
	s.Require().NoError(err, "Should not error when no CC records exist")
}

func (s *MarkCCReadTestSuite) TestMarkReadIdempotent() {
	instID := s.createInstance("MCC-002", nil)

	record := &approval.CCRecord{
		InstanceID: instID,
		CCUserID:   "cc-user-3",
		IsManual:   false,
	}
	_, err := s.db.NewInsert().Model(record).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Mark read first time
	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-3",
	})
	s.Require().NoError(err, "Should not return error")

	// Mark read second time - should be idempotent (no unread records left)
	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-user-3",
	})
	s.Require().NoError(err, "Should be idempotent")
}

func (s *MarkCCReadTestSuite) TestMarkReadShouldAdvanceCCNodeWhenAllRead() {
	currentNodeID := s.ccNodeID
	instID := s.createInstance("MCC-003", &currentNodeID)

	records := []approval.CCRecord{
		{InstanceID: instID, NodeID: &s.ccNodeID, CCUserID: "cc-advance-1", IsManual: false},
		{InstanceID: instID, NodeID: &s.ccNodeID, CCUserID: "cc-advance-2", IsManual: false},
	}
	for i := range records {
		_, err := s.db.NewInsert().Model(&records[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should create unread CC records for read-confirm node")
	}

	_, err := s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-advance-1",
	})
	s.Require().NoError(err, "Should mark first user as read")

	var intermediate approval.Instance

	intermediate.ID = instID
	s.Require().NoError(
		s.db.NewSelect().Model(&intermediate).WherePK().Scan(s.ctx),
		"Should reload instance after first user marks read",
	)
	s.Assert().Equal(approval.InstanceRunning, intermediate.Status, "Instance should remain running before all CC users read")
	s.Require().NotNil(intermediate.CurrentNodeID, "Current node should still exist before all reads complete")
	s.Assert().Equal(s.ccNodeID, *intermediate.CurrentNodeID, "Instance should stay on CC node before all reads complete")

	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-advance-2",
	})
	s.Require().NoError(err, "Should mark second user as read and complete CC node")

	var finished approval.Instance

	finished.ID = instID
	s.Require().NoError(
		s.db.NewSelect().Model(&finished).WherePK().Scan(s.ctx),
		"Should reload instance after all users mark read",
	)
	s.Assert().Equal(approval.InstanceApproved, finished.Status, "Instance should advance to end node and complete after all reads")
	s.Require().NotNil(finished.CurrentNodeID, "Instance should keep current node pointing to end node after completion")
	s.Assert().Equal(s.endNodeID, *finished.CurrentNodeID, "Instance current node should move to end node after completion")
	s.Assert().NotNil(finished.FinishedAt, "Instance should set finished time after reaching end")
}

func (s *MarkCCReadTestSuite) TestMarkReadShouldNotAdvanceWhenCCNodeIsNotCurrent() {
	currentNodeID := s.endNodeID
	instID := s.createInstance("MCC-004", &currentNodeID)

	record := &approval.CCRecord{
		InstanceID: instID,
		NodeID:     &s.ccNodeID,
		CCUserID:   "cc-stale-node",
		IsManual:   false,
	}
	_, err := s.db.NewInsert().Model(record).Exec(s.ctx)
	s.Require().NoError(err, "Should create unread CC record on non-current node")

	_, err = s.handler.Handle(s.ctx, command.MarkCCReadCmd{
		InstanceID: instID,
		UserID:     "cc-stale-node",
	})
	s.Require().NoError(err, "Should mark CC record as read even when node is not current")

	var instance approval.Instance

	instance.ID = instID
	s.Require().NoError(
		s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx),
		"Should reload instance after marking CC read on non-current node",
	)
	s.Assert().Equal(approval.InstanceRunning, instance.Status, "Instance status should remain running when CC node is not current")
	s.Require().NotNil(instance.CurrentNodeID, "Instance should keep current node when CC node is not current")
	s.Assert().Equal(s.endNodeID, *instance.CurrentNodeID, "Instance current node should remain unchanged when CC node is not current")
}
