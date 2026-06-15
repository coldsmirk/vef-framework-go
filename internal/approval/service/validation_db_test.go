package service_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ValidationServiceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// roleStubAssigneeService answers GetRoleUsers from an in-memory map so the
// role branch of CheckInitiationPermission can be exercised without a host.
// It deliberately does NOT implement RoleMembershipChecker, forcing the
// GetRoleUsers fallback path (the fast path is unit-tested in TestUserHasRole).
type roleStubAssigneeService struct {
	usersByRole map[string][]approval.UserInfo
}

func (*roleStubAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, nil
}

func (*roleStubAssigneeService) GetDepartmentLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, nil
}

func (s *roleStubAssigneeService) GetRoleUsers(_ context.Context, roleID string) ([]approval.UserInfo, error) {
	return s.usersByRole[roleID], nil
}

// ValidationServiceTestSuite covers the DB-backed authorization gates:
// CheckInitiationPermission (who may start a flow) and ValidateRollbackTarget
// (which node a running instance may rewind to).
type ValidationServiceTestSuite struct {
	suite.Suite

	ctx context.Context
	db  orm.DB
}

func (s *ValidationServiceTestSuite) TearDownTest() {
	cleanAllServiceData(s.ctx, s.db)
}

func (s *ValidationServiceTestSuite) TearDownSuite() {
	cleanAllServiceData(s.ctx, s.db)
}

// seedFlow inserts a category + flow and returns the flow ID.
func (s *ValidationServiceTestSuite) seedFlow(code string) string {
	cat := &approval.FlowCategory{TenantID: "default", Code: code + "-cat", Name: "Cat"}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "should insert category")

	flow := &approval.Flow{
		TenantID: "default", CategoryID: cat.ID, Code: code, Name: "Flow",
		BindingMode: approval.BindingStandalone,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "should insert flow")

	return flow.ID
}

func (s *ValidationServiceTestSuite) seedInitiator(flowID string, kind approval.InitiatorKind, ids ...string) {
	initiator := &approval.FlowInitiator{FlowID: flowID, Kind: kind, IDs: ids}
	_, err := s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "should insert flow initiator")
}

func (s *ValidationServiceTestSuite) TestCheckInitiationPermission() {
	s.Run("EmptyInitiatorsDeniesAll", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-empty")
		svc := service.NewValidationService(nil)

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", nil)
		s.Require().NoError(err, "Empty initiators should be a denial, not an error")
		s.Assert().False(allowed, "No initiator rows must deny everyone")
	})

	s.Run("UserMatch", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-user")
		s.seedInitiator(flowID, approval.InitiatorUser, "u1", "u2")

		svc := service.NewValidationService(nil)

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "u2", nil)
		s.Require().NoError(err, "User match check should not error")
		s.Assert().True(allowed, "Applicant listed in an InitiatorUser rule is allowed")
	})

	s.Run("UserNoMatch", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-user-no")
		s.seedInitiator(flowID, approval.InitiatorUser, "u1")

		svc := service.NewValidationService(nil)

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "stranger", nil)
		s.Require().NoError(err, "User no-match check should not error")
		s.Assert().False(allowed, "Applicant absent from the user rule is denied")
	})

	s.Run("DepartmentMatch", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-dept")
		s.seedInitiator(flowID, approval.InitiatorDepartment, "dept-1")

		svc := service.NewValidationService(nil)

		dept := "dept-1"
		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", &dept)
		s.Require().NoError(err, "Department match check should not error")
		s.Assert().True(allowed, "Applicant whose department is listed is allowed")
	})

	s.Run("DepartmentNilApplicantDepartmentSkips", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-dept-nil")
		s.seedInitiator(flowID, approval.InitiatorDepartment, "dept-1")

		svc := service.NewValidationService(nil)

		// A nil applicantDepartmentID must skip the department rule (no panic,
		// no false match) and fall through to a denial.
		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", nil)
		s.Require().NoError(err, "Nil department must not error")
		s.Assert().False(allowed, "A nil applicant department cannot satisfy a department rule")
	})

	s.Run("RoleMatchViaFallback", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-role")
		s.seedInitiator(flowID, approval.InitiatorRole, "role-1")

		svc := service.NewValidationService(&roleStubAssigneeService{
			usersByRole: map[string][]approval.UserInfo{"role-1": {{ID: "applicant"}}},
		})

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", nil)
		s.Require().NoError(err, "Role match check should not error")
		s.Assert().True(allowed, "Applicant holding a listed role (via GetRoleUsers) is allowed")
	})

	s.Run("RoleNoMatch", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-role-no")
		s.seedInitiator(flowID, approval.InitiatorRole, "role-1")

		svc := service.NewValidationService(&roleStubAssigneeService{
			usersByRole: map[string][]approval.UserInfo{"role-1": {{ID: "someone-else"}}},
		})

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", nil)
		s.Require().NoError(err, "Role no-match check should not error")
		s.Assert().False(allowed, "Applicant not holding any listed role is denied")
	})

	s.Run("RoleWithoutAssigneeServiceSkips", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		flowID := s.seedFlow("init-role-nosvc")
		s.seedInitiator(flowID, approval.InitiatorRole, "role-1")
		// No AssigneeService configured: the role branch must be skipped
		// rather than dereferencing a nil service.
		svc := service.NewValidationService(nil)

		allowed, err := svc.CheckInitiationPermission(s.ctx, s.db, flowID, "applicant", nil)
		s.Require().NoError(err, "Role rule without an assignee service must not error")
		s.Assert().False(allowed, "Without an assignee service the role rule cannot grant access")
	})
}

func (s *ValidationServiceTestSuite) TestValidateRollbackTarget() {
	// seedVersionWithNodes builds a published version with start → A → B nodes
	// and the start→A, A→B edges, returning the version ID and node IDs.
	seedVersionWithNodes := func(code string) (versionID, startID, nodeAID, nodeBID string) {
		flowID := s.seedFlow(code)

		version := &approval.FlowVersion{FlowID: flowID, Version: 1, Status: approval.VersionPublished}
		_, err := s.db.NewInsert().Model(version).Exec(s.ctx)
		s.Require().NoError(err, "should insert version")

		start := &approval.FlowNode{FlowVersionID: version.ID, Key: "start", Kind: approval.NodeStart, Name: "Start"}
		nodeA := &approval.FlowNode{FlowVersionID: version.ID, Key: "a", Kind: approval.NodeApproval, Name: "A"}

		nodeB := &approval.FlowNode{FlowVersionID: version.ID, Key: "b", Kind: approval.NodeApproval, Name: "B"}
		for _, n := range []*approval.FlowNode{start, nodeA, nodeB} {
			_, err = s.db.NewInsert().Model(n).Exec(s.ctx)
			s.Require().NoError(err, "should insert node %s", n.Key)
		}

		edges := []*approval.FlowEdge{
			{FlowVersionID: version.ID, Key: "e1", SourceNodeID: start.ID, TargetNodeID: nodeA.ID},
			{FlowVersionID: version.ID, Key: "e2", SourceNodeID: nodeA.ID, TargetNodeID: nodeB.ID},
		}
		for _, e := range edges {
			_, err = s.db.NewInsert().Model(e).Exec(s.ctx)
			s.Require().NoError(err, "should insert edge %s", e.Key)
		}

		return version.ID, start.ID, nodeA.ID, nodeB.ID
	}

	instanceFor := func(versionID string) *approval.Instance {
		return &approval.Instance{TenantID: "default", FlowVersionID: versionID}
	}

	svc := service.NewValidationService(nil)

	s.Run("TargetEqualsCurrentNodeRejected", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, _ := seedVersionWithNodes("rb-self")
		current := &approval.FlowNode{RollbackType: approval.RollbackAny}
		current.ID = nodeAID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Rolling back to the current node must be rejected")
	})

	s.Run("RollbackNoneDenies", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-none")
		current := &approval.FlowNode{RollbackType: approval.RollbackNone}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().ErrorIs(err, shared.ErrRollbackNotAllowed, "RollbackNone must deny rollback")
	})

	s.Run("OutOfEnumRollbackTypeDenies", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-bogus")
		current := &approval.FlowNode{RollbackType: approval.RollbackType("bogus")}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().ErrorIs(err, shared.ErrRollbackNotAllowed,
			"A corrupt out-of-enum rollback type must deny, never behave like 'any'")
	})

	s.Run("PreviousAcceptsAdjacent", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-prev-ok")
		current := &approval.FlowNode{RollbackType: approval.RollbackPrevious}
		current.ID = nodeBID

		// A → B edge exists, so rolling B back to A is adjacent and allowed.
		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().NoError(err, "Previous rollback to an adjacent predecessor must be allowed")
	})

	s.Run("PreviousRejectsNonAdjacent", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, startID, _, nodeBID := seedVersionWithNodes("rb-prev-no")
		current := &approval.FlowNode{RollbackType: approval.RollbackPrevious}
		current.ID = nodeBID

		// No start → B edge exists, so B cannot roll back to start.
		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, startID)
		s.Require().ErrorIs(err, shared.ErrInvalidRollbackTarget,
			"Previous rollback to a non-adjacent node must be rejected")
	})

	s.Run("StartAcceptsStartNode", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, startID, _, nodeBID := seedVersionWithNodes("rb-start-ok")
		current := &approval.FlowNode{RollbackType: approval.RollbackStart}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, startID)
		s.Require().NoError(err, "Start rollback to the version's start node must be allowed")
	})

	s.Run("StartRejectsNonStartNode", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-start-no")
		current := &approval.FlowNode{RollbackType: approval.RollbackStart}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().ErrorIs(err, shared.ErrInvalidRollbackTarget,
			"Start rollback to a non-start node must be rejected")
	})

	s.Run("AnyAcceptsNodeInVersion", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-any-ok")
		current := &approval.FlowNode{RollbackType: approval.RollbackAny}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().NoError(err, "Any rollback to a node belonging to the version must be allowed")
	})

	s.Run("AnyRejectsNodeOutsideVersion", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, _, nodeBID := seedVersionWithNodes("rb-any-no")
		current := &approval.FlowNode{RollbackType: approval.RollbackAny}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, "node-from-another-version")
		s.Require().ErrorIs(err, shared.ErrInvalidRollbackTarget,
			"Any rollback to a node not in the version must be rejected")
	})

	s.Run("SpecifiedAcceptsWhitelistedKey", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-spec-ok")
		// Node A's key is "a"; whitelist it.
		current := &approval.FlowNode{RollbackType: approval.RollbackSpecified, RollbackTargetKeys: []string{"a"}}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().NoError(err, "Specified rollback to a whitelisted target key must be allowed")
	})

	s.Run("SpecifiedRejectsKeyNotInWhitelist", func() {
		defer cleanAllServiceData(s.ctx, s.db)

		versionID, _, nodeAID, nodeBID := seedVersionWithNodes("rb-spec-no")
		// Node A's key is "a" but only "c" is whitelisted.
		current := &approval.FlowNode{RollbackType: approval.RollbackSpecified, RollbackTargetKeys: []string{"c"}}
		current.ID = nodeBID

		err := svc.ValidateRollbackTarget(s.ctx, s.db, instanceFor(versionID), current, nodeAID)
		s.Require().ErrorIs(err, shared.ErrInvalidRollbackTarget,
			"Specified rollback to a target whose key is not whitelisted must be rejected")
	})
}
