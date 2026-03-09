package command_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &AddCCTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// AddCCTestSuite tests the AddCCHandler.
type AddCCTestSuite struct {
	suite.Suite

	ctx         context.Context
	db          orm.DB
	handler     *command.AddCCHandler
	fixture     *MinimalFixture
	nodeID      string
	instanceSeq int
}

func (s *AddCCTestSuite) SetupSuite() {
	s.handler = command.NewAddCCHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher(), nil)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "cc")

	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "cc-node",
		Kind:              approval.NodeApproval,
		Name:              "CC Test Node",
		IsManualCCAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *AddCCTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.EventOutbox)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
	)
}

func (s *AddCCTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *AddCCTestSuite) insertInstance(currentNodeID, operatorID string) *approval.Instance {
	s.instanceSeq++
	nodeIDPtr := new(string)
	*nodeIDPtr = currentNodeID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Test",
		InstanceNo:    fmt.Sprintf("CC-%03d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: nodeIDPtr,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     currentNodeID,
		AssigneeID: operatorID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should insert operator task for node-level authorization")

	return inst
}

func (s *AddCCTestSuite) TestAddCCSuccess() {
	inst := s.insertInstance(s.nodeID, "operator-1")

	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1", "cc-user-2"},
		OperatorID: "operator-1",
	})
	s.Require().NoError(err, "Should add CC without error")

	// Verify CC records
	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(records, 2, "Should create 2 CC records")
}

func (s *AddCCTestSuite) TestAddCCDuplicateFiltered() {
	inst := s.insertInstance(s.nodeID, "operator-2")

	// Insert existing CC record
	nodeIDPtr := new(string)
	*nodeIDPtr = s.nodeID
	existing := &approval.CCRecord{
		InstanceID: inst.ID,
		NodeID:     nodeIDPtr,
		CCUserID:   "cc-user-1",
		IsManual:   true,
	}
	_, err := s.db.NewInsert().Model(existing).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Add CC with one existing and one new user
	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1", "cc-user-3"},
		OperatorID: "operator-2",
	})
	s.Require().NoError(err, "Should add CC without error")

	// Verify only new CC record added (total = 2: 1 existing + 1 new)
	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().Len(records, 2, "Should have 2 CC records total (1 existing + 1 new)")
}

func (s *AddCCTestSuite) TestAddCCManualNotAllowed() {
	// Create node with manual CC disabled
	node := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "no-cc-node",
		Kind:              approval.NodeApproval,
		Name:              "No CC Node",
		IsManualCCAllowed: false,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)

	s.Require().NoError(err, "Should not return error")
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := s.insertInstance(node.ID, "operator-3")

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1"},
		OperatorID: "operator-3",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrManualCcNotAllowed, "Should return ErrManualCcNotAllowed")
}

func (s *AddCCTestSuite) TestAddCCInstanceNotFound() {
	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: "non-existent",
		CCUserIDs:  []string{"cc-user-1"},
		OperatorID: "operator-4",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceNotFound, "Should return ErrInstanceNotFound")
}

func (s *AddCCTestSuite) TestAddCCInstanceCompleted() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Completed Instance",
		InstanceNo:    "CC-COMPLETED-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceApproved,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert completed instance")

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1"},
		OperatorID: "operator-5",
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceCompleted, "Should reject adding CC for completed instance")
}

func (s *AddCCTestSuite) TestAddCCCurrentNodeNotFound() {
	missingNodeID := "missing-current-node"
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "CC Missing Node Instance",
		InstanceNo:    "CC-MISSING-NODE-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &missingNodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance with missing current node id")

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1"},
		OperatorID: "operator-6",
	})
	s.Require().Error(err, "Should fail when current node cannot be loaded")
}

func (s *AddCCTestSuite) TestAddCCEventUsesInsertedUserIDs() {
	inst := s.insertInstance(s.nodeID, "operator-7")

	// Existing user should be filtered out from insertion and event payload.
	existing := &approval.CCRecord{
		InstanceID: inst.ID,
		NodeID:     &s.nodeID,
		CCUserID:   "cc-user-1",
		IsManual:   true,
	}
	_, err := s.db.NewInsert().Model(existing).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1", "cc-user-2", "cc-user-3"},
		OperatorID: "operator-7",
	})
	s.Require().NoError(err, "Should not return error")

	var outbox approval.EventOutbox
	s.Require().NoError(s.db.NewSelect().
		Model(&outbox).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.cc.notified") }).
		OrderByDesc("created_at").
		Limit(1).
		Scan(s.ctx), "Should not return error")

	rawIDs, ok := outbox.Payload["ccUserIds"].([]any)
	s.Require().True(ok, "Event payload should contain ccUserIds as array")

	actual := make([]string, 0, len(rawIDs))
	for _, item := range rawIDs {
		userID, ok := item.(string)
		s.Require().True(ok, "ccUserIds item should be string")

		actual = append(actual, userID)
	}

	slices.Sort(actual)

	expected := []string{"cc-user-2", "cc-user-3"}
	slices.Sort(expected)
	s.Assert().Equal(expected, actual, "Event should include only actually inserted CC users")
}

func (s *AddCCTestSuite) TestAddCCShouldDeduplicateAndIgnoreEmptyUserIDs() {
	inst := s.insertInstance(s.nodeID, "operator-8")

	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-2", "", "cc-user-2", "cc-user-3"},
		OperatorID: "operator-8",
	})
	s.Require().NoError(err, "Should add CC without error")

	var records []approval.CCRecord
	s.Require().NoError(
		s.db.NewSelect().
			Model(&records).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			OrderBy("created_at").
			Scan(s.ctx),
		"Should query inserted CC records",
	)
	s.Require().Len(records, 2, "Should only insert unique non-empty CC users")
	s.Assert().Equal("cc-user-2", records[0].CCUserID, "Should preserve first-seen CC user order")
	s.Assert().Equal("cc-user-3", records[1].CCUserID, "Should preserve first-seen CC user order")

	var outbox approval.EventOutbox
	s.Require().NoError(
		s.db.NewSelect().
			Model(&outbox).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("event_type", "approval.cc.notified") }).
			OrderByDesc("created_at").
			Limit(1).
			Scan(s.ctx),
		"Should query latest cc-notified event",
	)

	rawIDs, ok := outbox.Payload["ccUserIds"].([]any)
	s.Require().True(ok, "Event payload should contain ccUserIds array")
	s.Require().Len(rawIDs, 2, "Event payload should contain deduplicated ccUserIds")
	s.Assert().Equal("cc-user-2", rawIDs[0], "Event payload should preserve first-seen CC user order")
	s.Assert().Equal("cc-user-3", rawIDs[1], "Event payload should preserve first-seen CC user order")
}

func (s *AddCCTestSuite) TestAddCCShouldRejectUnauthorizedOperator() {
	inst := s.insertInstance(s.nodeID, "authorized-operator")

	_, err := s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-1"},
		OperatorID: "unauthorized-operator",
	})
	s.Require().Error(err, "Unauthorized operator should not add manual CC")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return not-assignee error for unauthorized operator")
}

func (s *AddCCTestSuite) TestAddCCShouldAllowSameUserOnDifferentNodes() {
	inst := s.insertInstance(s.nodeID, "node-operator")

	otherNode := &approval.FlowNode{
		FlowVersionID:     s.fixture.VersionID,
		Key:               "other-cc-node",
		Kind:              approval.NodeApproval,
		Name:              "Other CC Node",
		IsManualCCAllowed: true,
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another node for node-scoped CC dedup test")

	_, err = s.db.NewInsert().Model(&approval.CCRecord{
		InstanceID: inst.ID,
		NodeID:     &otherNode.ID,
		CCUserID:   "cc-user-cross-node",
		IsManual:   false,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert existing CC record on another node")

	_, err = s.handler.Handle(s.ctx, command.AddCCCmd{
		InstanceID: inst.ID,
		CCUserIDs:  []string{"cc-user-cross-node"},
		OperatorID: "node-operator",
	})
	s.Require().NoError(err, "Current node should still allow CC for user already CC'd on another node")

	count, err := s.db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", inst.ID).
				Equals("cc_user_id", "cc-user-cross-node")
		}).
		Count(s.ctx)
	s.Require().NoError(err, "Should count CC records across nodes")
	s.Assert().Equal(int64(2), count, "Should keep both node-specific CC records")
}

func (s *AddCCTestSuite) TestAddCCShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	inst := s.insertInstance(s.nodeID, "operator-concurrency")
	operatorID := "operator-concurrency"

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_cc_record")

	<-lockReady

	start := make(chan struct{})
	errCh := make(chan error, 2)

	var wg sync.WaitGroup

	runOne := func() {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.AddCCCmd{
				InstanceID: inst.ID,
				CCUserIDs:  []string{"cc-user-concurrency"},
				OperatorID: operatorID,
			})

			return err
		})
		errCh <- err
	}

	wg.Go(runOne)
	wg.Go(runOne)
	close(start)

	time.Sleep(200 * time.Millisecond)
	close(releaseLock)

	s.Require().NoError(<-lockDone, "Table lock transaction should complete without error")
	wg.Wait()
	close(errCh)

	for err := range errCh {
		s.Require().NoError(err, "Concurrent add-cc operations should complete without error")
	}

	count, err := s.db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", inst.ID).
				Equals("node_id", s.nodeID).
				Equals("cc_user_id", "cc-user-concurrency")
		}).
		Count(s.ctx)
	s.Require().NoError(err, "Should count inserted CC records")
	s.Assert().Equal(int64(1), count, "Concurrent add-cc should insert only one record for same user on same node")
}
